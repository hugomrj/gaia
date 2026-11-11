// internal/tenant/store.go

package tenant

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

var reSlug = regexp.MustCompile(`^[a-z0-9-]{3,32}$`)

type Store struct {
	mu      sync.RWMutex
	bySlug  map[string]*Tenant
	byToken map[string]*Tenant
	byID    map[int64]*Tenant
	db      *sql.DB
	encrypt bool
}

func NewStore(db *sql.DB, encrypt bool) (*Store, error) {
	s := &Store{
		bySlug:  make(map[string]*Tenant),
		byToken: make(map[string]*Tenant),
		byID:    make(map[int64]*Tenant),
		db:      db,
		encrypt: encrypt,
	}
	if err := s.Reload(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Reload() error {
	// 1. Cargar Tenants
	rows, err := s.db.Query(`SELECT id, slug, name, token_hash, status, created_at, updated_at FROM tenants`)
	if err != nil {
		return fmt.Errorf("query tenants: %w", err)
	}
	defer rows.Close()

	bySlug := make(map[string]*Tenant)
	byToken := make(map[string]*Tenant)
	byID := make(map[int64]*Tenant)

	for rows.Next() {
		t := &Tenant{}
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.TokenHash, &t.Status, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("scan tenant: %w", err)
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		t.Providers = make([]Provider, 0)

		bySlug[t.Slug] = t
		byToken[t.TokenHash] = t
		byID[t.ID] = t
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// 2. Cargar Providers
	pRows, err := s.db.Query(`SELECT tenant_id, provider, api_key_plain, api_key_enc, is_default FROM tenant_providers`)
	if err != nil {
		return fmt.Errorf("query providers: %w", err)
	}
	defer pRows.Close()

	for pRows.Next() {
		var tenantID int64
		var p Provider
		var plain sql.NullString
		var enc []byte
		var isDefault int

		if err := pRows.Scan(&tenantID, &p.Provider, &plain, &enc, &isDefault); err != nil {
			return fmt.Errorf("scan provider: %w", err)
		}
		if plain.Valid {
			p.APIKeyPlain = plain.String
		}
		p.APIKeyEnc = enc
		p.IsDefault = isDefault == 1

		if t, ok := byID[tenantID]; ok {
			t.Providers = append(t.Providers, p)
		}
	}
	if err := pRows.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	s.bySlug = bySlug
	s.byToken = byToken
	s.byID = byID
	s.mu.Unlock()
	return nil
}

func (s *Store) GetBySlug(slug string) (*Tenant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.bySlug[slug]
	return t, ok
}

func (s *Store) GetByTokenHash(hash string) (*Tenant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.byToken[hash]
	return t, ok
}

func (s *Store) List() []*Tenant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Tenant, 0, len(s.byID))
	for _, t := range s.byID {
		out = append(out, t)
	}
	return out
}

func (s *Store) Create(slug, name, providerName, apiKey string) (*Tenant, string, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if !reSlug.MatchString(slug) {
		return nil, "", errors.New("slug inválido")
	}
	if strings.TrimSpace(name) == "" {
		return nil, "", errors.New("name vacío")
	}
	if strings.TrimSpace(providerName) == "" || strings.TrimSpace(apiKey) == "" {
		return nil, "", errors.New("provider y api_key son obligatorios")
	}
	if s.encrypt {
		return nil, "", errors.New("ENCRYPT_KEYS=true no implementado")
	}

	token, hash, err := GenerateToken()
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC()

	// Iniciar Transacción SQLite
	tx, err := s.db.Begin()
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()

	// 1. Insertar Tenant
	res, err := tx.Exec(
		`INSERT INTO tenants (slug, name, token_hash, status, created_at, updated_at) VALUES (?, ?, ?, 'active', ?, ?)`,
		slug, name, hash, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, "", fmt.Errorf("el slug %q ya existe", slug)
		}
		return nil, "", fmt.Errorf("insert tenant: %w", err)
	}
	id, _ := res.LastInsertId()

	// 2. Insertar Proveedor
	_, err = tx.Exec(
		`INSERT INTO tenant_providers (tenant_id, provider, api_key_plain, api_key_enc, is_default) VALUES (?, ?, ?, NULL, 1)`,
		id, providerName, apiKey,
	)
	if err != nil {
		return nil, "", fmt.Errorf("insert provider: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, "", err
	}

	// Actualizar Memoria
	t := &Tenant{
		ID:        id,
		Slug:      slug,
		Name:      name,
		TokenHash: hash,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
		Providers: []Provider{
			{Provider: providerName, APIKeyPlain: apiKey, IsDefault: true},
		},
	}

	s.mu.Lock()
	s.bySlug[t.Slug] = t
	s.byToken[t.TokenHash] = t
	s.byID[t.ID] = t
	s.mu.Unlock()

	return t, token, nil
}

func (s *Store) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM tenants WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("tenant no encontrado")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.byID[id]; ok {
		delete(s.bySlug, t.Slug)
		delete(s.byToken, t.TokenHash)
		delete(s.byID, id)
	}
	return nil
}

func (s *Store) RotateToken(id int64) (*Tenant, string, error) {
	s.mu.RLock()
	t, ok := s.byID[id]
	s.mu.RUnlock()
	if !ok {
		return nil, "", errors.New("tenant no encontrado")
	}

	token, hash, err := GenerateToken()
	if err != nil {
		return nil, "", err
	}

	now := time.Now().UTC()
	if _, err := s.db.Exec(
		`UPDATE tenants SET token_hash = ?, updated_at = ? WHERE id = ?`,
		hash, now.Format(time.RFC3339), id,
	); err != nil {
		return nil, "", fmt.Errorf("update: %w", err)
	}

	s.mu.Lock()
	delete(s.byToken, t.TokenHash)
	t.TokenHash = hash
	t.UpdatedAt = now
	s.byToken[hash] = t
	s.mu.Unlock()

	return t, token, nil
}

func GenerateToken() (plain, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	plain = "gaia_sk_" + hex.EncodeToString(b)
	hash = HashToken(plain)
	return
}

func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}