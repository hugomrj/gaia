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
		SELECT tp.id, tp.tenant_id, p.name 
		FROM tenant_proveedor tp
		JOIN providers p ON p.id = tp.provider_id
	`)
	if err != nil {
		return fmt.Errorf("query tenant_proveedor: %w", err)
	}
	defer pRows.Close()

	for pRows.Next() {
		var tp TenantProvider
		if err := pRows.Scan(&tp.ID, &tp.TenantID, &tp.ProviderName); err != nil {
			return err
		}
		tp.Keys = make([]APIKey, 0)

		tpMap[tp.ID] = &tp
		if t, ok := byID[tp.TenantID]; ok {
			t.Providers = append(t.Providers, tp)
		}
	}



	// 3. Llaves (keys)
	kRows, err := s.db.Query(`SELECT id, tenant_proveedor_id, api_key_plain, api_key_enc, is_active, usage_count FROM keys`)
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

		// El Scan debe incluir &k.UsageCount
		if err := kRows.Scan(&k.ID, &tpID, &plain, &enc, &isActive, &k.UsageCount); err != nil {
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
	SELECT k.id, t.name, t.slug, p.name, k.api_key_plain
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
		if err := rows.Scan(&gk.KeyID, &gk.TenantName, &gk.TenantSlug, &gk.ProviderName, &apiKey); err != nil {
			return nil, err
		}
		if apiKey.Valid {
			gk.APIKeyPlain = apiKey.String
		}
		result = append(result, gk)
	}
	return result, nil
}

// LinkProvider asocia un proveedor a un tenant (crea la cabecera)
func (s *Store) LinkProvider(tenantID int64, providerName string) error {
	var providerID int64
	err := s.db.QueryRow(`SELECT id FROM providers WHERE name = ?`, providerName).Scan(&providerID)
	if err != nil {
		return fmt.Errorf("proveedor %q no encontrado", providerName)
	}

	var tpID int64
	err = s.db.QueryRow(`SELECT id FROM tenant_proveedor WHERE tenant_id = ? AND provider_id = ?`, tenantID, providerID).Scan(&tpID)
	if err == nil {
		return errors.New("el tenant ya tiene este proveedor habilitado")
	}

	// Insertamos sin el campo is_default
	_, err = s.db.Exec(`INSERT INTO tenant_proveedor (tenant_id, provider_id) VALUES (?, ?)`, tenantID, providerID)
	if err != nil {
		return fmt.Errorf("insertar cabecera: %w", err)
	}

	return s.Reload()
}

// AddKey inserta una llave al proveedor (ya no recibe isDefault)
func (s *Store) AddKey(tenantID int64, providerName, apiKey string) error {
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
		// Si no existe la cabecera, la creamos
		res, err := s.db.Exec(`INSERT INTO tenant_proveedor (tenant_id, provider_id) VALUES (?, ?)`, tenantID, providerID)
		if err != nil {
			return fmt.Errorf("insertar cabecera: %w", err)
		}
		tpID, _ = res.LastInsertId()
	} else if err != nil {
		return fmt.Errorf("buscar cabecera: %w", err)
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

func (s *Store) UpdateName(id int64, name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("el nombre no puede estar vacío")
	}

	now := time.Now().UTC()
	_, err := s.db.Exec(`UPDATE tenants SET name = ?, updated_at = ? WHERE id = ?`, name, now.Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update name: %w", err)
	}

	s.mu.Lock()
	if t, ok := s.byID[id]; ok {
		t.Name = name
		t.UpdatedAt = now
	}
	s.mu.Unlock()
	return nil
}

// ==========================================
// MÓDULO DE PROVIDERS
// ==========================================

type ProviderInfo struct {
	ID   int64
	Name string
}

func (s *Store) CreateProvider(name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return errors.New("el nombre del proveedor no puede estar vacío")
	}

	_, err := s.db.Exec(`INSERT INTO providers (name) VALUES (?)`, name)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("el proveedor %q ya existe", name)
		}
		return fmt.Errorf("insert provider: %w", err)
	}

	return nil
}

func (s *Store) ListProvidersWithID() ([]ProviderInfo, error) {
	rows, err := s.db.Query(`SELECT id, name FROM providers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ProviderInfo
	for rows.Next() {
		var p ProviderInfo
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, nil
}

func (s *Store) UpdateProvider(id int64, name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return errors.New("el nombre no puede estar vacío")
	}

	_, err := s.db.Exec(`UPDATE providers SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("el proveedor %q ya existe", name)
		}
		return fmt.Errorf("update provider: %w", err)
	}
	return nil
}

func (s *Store) DeleteProvider(id int64) error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM tenant_proveedor WHERE provider_id = ?`, id).Scan(&count)
	if err != nil {
		return fmt.Errorf("verificar uso: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("no se puede borrar: hay %d tenant(s) usando este proveedor", count)
	}

	_, err = s.db.Exec(`DELETE FROM providers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	return nil
}



// ToggleKeyStatus cambia el estado de una llave (activa/inactiva)
func (s *Store) ToggleKeyStatus(keyID int64) error {
	// NOT is_active invierte el valor (si es 1 pasa a 0, y viceversa)
	_, err := s.db.Exec(`UPDATE keys SET is_active = NOT is_active WHERE id = ?`, keyID)
	if err != nil {
		return fmt.Errorf("toggle key: %w", err)
	}
	return s.Reload()
}