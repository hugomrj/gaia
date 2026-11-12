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
	bySlug := make(map[string]*Tenant)
	byToken := make(map[string]*Tenant)
	byID := make(map[int64]*Tenant)
	tpMap := make(map[int64]*TenantProvider)

	// 1. Tenants
	rows, err := s.db.Query(`SELECT id, slug, name, token_hash, token_plain, status, created_at, updated_at FROM tenants`)
	if err != nil {
		return fmt.Errorf("query tenants: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		t := &Tenant{}
		var createdAt, updatedAt string
		
		// Incluimos token_plain en el Scan
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.TokenHash, &t.TokenPlain, &t.Status, &createdAt, &updatedAt); err != nil {
			return err
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		t.Providers = make([]TenantProvider, 0)

		bySlug[t.Slug] = t
		byToken[t.TokenHash] = t
		byID[t.ID] = t
	}

	// 2. Cabeceras (tenant_proveedor)
	pRows, err := s.db.Query(`
		SELECT tp.id, tp.tenant_id, p.name, tp.is_default 
		FROM tenant_proveedor tp
		JOIN providers p ON p.id = tp.provider_id
	`)
	if err != nil {
		return fmt.Errorf("query tenant_proveedor: %w", err)
	}
	defer pRows.Close()

	for pRows.Next() {
		var tp TenantProvider
		var isDefault int
		if err := pRows.Scan(&tp.ID, &tp.TenantID, &tp.ProviderName, &isDefault); err != nil {
			return err
		}
		tp.IsDefault = isDefault == 1
		tp.Keys = make([]APIKey, 0)

		tpMap[tp.ID] = &tp
		if t, ok := byID[tp.TenantID]; ok {
			t.Providers = append(t.Providers, tp)
		}
	}

	// 3. Llaves (keys)
	kRows, err := s.db.Query(`SELECT id, tenant_proveedor_id, api_key_plain, api_key_enc, is_active FROM keys`)
	if err != nil {
		return fmt.Errorf("query keys: %w", err)
	}
	defer kRows.Close()

	for kRows.Next() {
		var k APIKey
		var tpID int64
		var plain sql.NullString
		var enc []byte
		var isActive int

		if err := kRows.Scan(&k.ID, &tpID, &plain, &enc, &isActive); err != nil {
			return err
		}
		if plain.Valid {
			k.APIKeyPlain = plain.String
		}
		k.APIKeyEnc = enc
		k.IsActive = isActive == 1

		if tp, ok := tpMap[tpID]; ok {
			tp.Keys = append(tp.Keys, k)
			if t, ok := byID[tp.TenantID]; ok {
				for i := range t.Providers {
					if t.Providers[i].ID == tpID {
						t.Providers[i].Keys = tp.Keys
						break
					}
				}
			}
		}
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

// Create inserta el Tenant guardando el token en texto plano y el hash
func (s *Store) Create(slug, name string) (*Tenant, string, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if !reSlug.MatchString(slug) {
		return nil, "", errors.New("slug inválido: 3-32 caracteres, minúsculas, números o guiones")
	}
	if strings.TrimSpace(name) == "" {
		return nil, "", errors.New("el nombre no puede estar vacío")
	}

	token, hash, err := GenerateToken()
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC()

	// Insertamos incluyendo token_plain
	res, err := s.db.Exec(
		`INSERT INTO tenants (slug, name, token_hash, token_plain, status, created_at, updated_at) VALUES (?, ?, ?, ?, 'active', ?, ?)`,
		slug, name, hash, token, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, "", fmt.Errorf("el slug %q ya existe", slug)
		}
		return nil, "", fmt.Errorf("insert tenant: %w", err)
	}
	id, _ := res.LastInsertId()

	// Creamos el tenant en memoria
	newTenant := &Tenant{
		ID:         id,
		Slug:       slug,
		Name:       name,
		TokenHash:  hash,
		TokenPlain: token,
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
		Providers:  make([]TenantProvider, 0),
	}

	s.mu.Lock()
	s.bySlug[slug] = newTenant
	s.byToken[hash] = newTenant
	s.byID[id] = newTenant
	s.mu.Unlock()

	return newTenant, token, nil
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

	// Actualizamos ambos campos en la DB
	if _, err := s.db.Exec(
		`UPDATE tenants SET token_hash = ?, token_plain = ?, updated_at = ? WHERE id = ?`,
		hash, token, now.Format(time.RFC3339), id,
	); err != nil {
		return nil, "", fmt.Errorf("update: %w", err)
	}

	s.mu.Lock()
	delete(s.byToken, t.TokenHash)
	t.TokenHash = hash
	t.TokenPlain = token
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

// ==========================================
// MÓDULO DE KEYS
// ==========================================

type GlobalKey struct {
	KeyID        int64
	TenantName   string
	TenantSlug   string
	ProviderName string
	APIKeyPlain  string
	IsDefault    bool
}

func (g GlobalKey) MaskedKey() string {
	if len(g.APIKeyPlain) <= 8 {
		return "••••••••"
	}
	return g.APIKeyPlain[:4] + "••••••••" + g.APIKeyPlain[len(g.APIKeyPlain)-4:]
}

func (s *Store) ListProviders() ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM providers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, nil
}

func (s *Store) ListAllKeys() ([]GlobalKey, error) {
	q := `
	SELECT k.id, t.name, t.slug, p.name, k.api_key_plain, tp.is_default
	FROM keys k
	JOIN tenant_proveedor tp ON tp.id = k.tenant_proveedor_id
	JOIN tenants t ON t.id = tp.tenant_id
	JOIN providers p ON p.id = tp.provider_id
	ORDER BY t.name ASC, p.name ASC
	`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []GlobalKey
	for rows.Next() {
		var gk GlobalKey
		var apiKey sql.NullString
		var isDefault int
		if err := rows.Scan(&gk.KeyID, &gk.TenantName, &gk.TenantSlug, &gk.ProviderName, &apiKey, &isDefault); err != nil {
			return nil, err
		}
		if apiKey.Valid {
			gk.APIKeyPlain = apiKey.String
		}
		gk.IsDefault = isDefault == 1
		result = append(result, gk)
	}
	return result, nil
}

func (s *Store) AddKey(tenantID int64, providerName, apiKey string, isDefault bool) error {
	if s.encrypt {
		return errors.New("encriptación no implementada aún")
	}

	var providerID int64
	err := s.db.QueryRow(`SELECT id FROM providers WHERE name = ?`, providerName).Scan(&providerID)
	if err != nil {
		return fmt.Errorf("proveedor %q no encontrado", providerName)
	}

	var tpID int64
	err = s.db.QueryRow(`SELECT id FROM tenant_proveedor WHERE tenant_id = ? AND provider_id = ?`, tenantID, providerID).Scan(&tpID)
	
	if err == sql.ErrNoRows {
		var isDefInt int = 0
		if isDefault { isDefInt = 1 }
		
		res, err := s.db.Exec(`INSERT INTO tenant_proveedor (tenant_id, provider_id, is_default) VALUES (?, ?, ?)`, tenantID, providerID, isDefInt)
		if err != nil {
			return fmt.Errorf("insertar cabecera: %w", err)
		}
		tpID, _ = res.LastInsertId()
	} else if err != nil {
		return fmt.Errorf("buscar cabecera: %w", err)
	} else if isDefault {
		_, err = s.db.Exec(`UPDATE tenant_proveedor SET is_default = 1 WHERE id = ?`, tpID)
		if err != nil {
			return fmt.Errorf("actualizar default: %w", err)
		}
	}

	_, err = s.db.Exec(
		`INSERT INTO keys (tenant_proveedor_id, api_key_plain, api_key_enc, is_active) VALUES (?, ?, NULL, 1)`,
		tpID, apiKey,
	)
	if err != nil {
		return fmt.Errorf("insertar key: %w", err)
	}

	return s.Reload()
}

func (s *Store) DeleteKey(keyID int64) error {
	_, err := s.db.Exec(`DELETE FROM keys WHERE id = ?`, keyID)
	if err != nil {
		return err
	}
	return s.Reload()
}