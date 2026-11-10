//	internal/tenant/store.go
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

// NewStore abre una DB existente y carga todos los tenants en memoria.
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

// Reload re-lee todos los tenants de SQLite y reemplaza la caché atómicamente.
func (s *Store) Reload() error {
	rows, err := s.db.Query(`SELECT id, slug, name, provider,
		api_key_plain, api_key_enc, token_hash, status, created_at, updated_at
		FROM tenants`)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	bySlug := make(map[string]*Tenant)
	byToken := make(map[string]*Tenant)
	byID := make(map[int64]*Tenant)

	for rows.Next() {
		t := &Tenant{}
		var plain sql.NullString
		var enc []byte
		var createdAt, updatedAt string // SQLite a veces devuelve string

		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Provider,
			&plain, &enc, &t.TokenHash, &t.Status, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		
		if plain.Valid { t.APIKeyPlain = plain.String }
		t.APIKeyEnc = enc
		
		// Parseo de fechas (soporta formato RFC3339 y formato SQLite)
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		bySlug[t.Slug] = t
		byToken[t.TokenHash] = t
		byID[t.ID] = t
	}
	if err := rows.Err(); err != nil { return err }

	s.mu.Lock()
	s.bySlug = bySlug
	s.byToken = byToken
	s.byID = byID
	s.mu.Unlock()
	return nil
}

// GetBySlug busca por slug (hot path, solo memoria).
func (s *Store) GetBySlug(slug string) (*Tenant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.bySlug[slug]
	return t, ok
}

// GetByTokenHash busca por SHA-256 del token (hot path, solo memoria).
func (s *Store) GetByTokenHash(hash string) (*Tenant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.byToken[hash]
	return t, ok
}

// List devuelve todos los tenants.
func (s *Store) List() []*Tenant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Tenant, 0, len(s.byID))
	for _, t := range s.byID {
		out = append(out, t)
	}
	return out
}

// Create inserta un tenant nuevo. Devuelve el token en claro UNA sola vez.
func (s *Store) Create(slug, name, provider, apiKey string) (*Tenant, string, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if !reSlug.MatchString(slug) {
		return nil, "", errors.New("slug inválido: 3-32 chars, minúsculas, números o guiones")
	}
	if strings.TrimSpace(name) == "" { return nil, "", errors.New("name vacío") }
	if strings.TrimSpace(provider) == "" { return nil, "", errors.New("provider vacío") }
	if strings.TrimSpace(apiKey) == "" { return nil, "", errors.New("api_key vacía") }

	token, hash, err := GenerateToken()
	if err != nil { return nil, "", fmt.Errorf("generar token: %w", err) }

	now := time.Now().UTC()
	var plainVal any = apiKey
	var encVal any = nil
	
	if s.encrypt {
		return nil, "", errors.New("ENCRYPT_KEYS=true aún no implementado (Fase 4)")
	}

	res, err := s.db.Exec(
		`INSERT INTO tenants
		(slug, name, provider, api_key_plain, api_key_enc, token_hash, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		slug, name, provider, plainVal, encVal, hash, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, "", fmt.Errorf("el slug %q ya existe", slug)
		}
		return nil, "", fmt.Errorf("insert: %w", err)
	}
	id, _ := res.LastInsertId()

	t := &Tenant{
		ID: id, Slug: slug, Name: name, Provider: provider,
		APIKeyPlain: apiKey, TokenHash: hash, Status: "active",
		CreatedAt: now, UpdatedAt: now,
	}

	s.mu.Lock()
	s.bySlug[t.Slug] = t
	s.byToken[t.TokenHash] = t
	s.byID[t.ID] = t
	s.mu.Unlock()

	return t, token, nil
}

// Delete borra un tenant.
func (s *Store) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM tenants WHERE id = ?`, id)
	if err != nil { return fmt.Errorf("delete: %w", err) }
	n, _ := res.RowsAffected()
	if n == 0 { return errors.New("tenant no encontrado") }

	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.byID[id]; ok {
		delete(s.bySlug, t.Slug)
		delete(s.byToken, t.TokenHash)
		delete(s.byID, id)
	}
	return nil
}

// RotateToken genera un token nuevo e invalida el anterior.
func (s *Store) RotateToken(id int64) (*Tenant, string, error) {
	s.mu.RLock()
	t, ok := s.byID[id]
	s.mu.RUnlock()
	if !ok { return nil, "", errors.New("tenant no encontrado") }

	token, hash, err := GenerateToken()
	if err != nil { return nil, "", err }
	
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

// APIKeyFor devuelve la key utilizable (plain o cifrada).
func (s *Store) APIKeyFor(t *Tenant) (string, error) {
	if s.encrypt {
		if len(t.APIKeyEnc) == 0 { return "", errors.New("tenant sin API key cifrada") }
		return "", errors.New("descifrado no implementado (Fase 4)")
	}
	if t.APIKeyPlain == "" { return "", errors.New("tenant sin API key") }
	return t.APIKeyPlain, nil
}

// GenerateToken genera gaia_sk_<64 hex> y su hash SHA-256.
func GenerateToken() (plain, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil { return "", "", err }
	plain = "gaia_sk_" + hex.EncodeToString(b)
	hash = HashToken(plain)
	return
}

// HashToken calcula SHA-256 del token.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}