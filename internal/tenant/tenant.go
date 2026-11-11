//  internal/tenant/tenant.go  
package tenant

import "time"

type Tenant struct {
	ID        int64
	Slug      string
	Name      string
	TokenHash string
	Status    string
	Providers []Provider // <-- NUEVO
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (t *Tenant) IsActive() bool {
	return t.Status == "active"
}

// GetProvider busca el proveedor específico o devuelve el default si viene vacío
func (t *Tenant) GetProvider(name string) (*Provider, bool) {
	// Si no piden uno específico, y solo hay uno, lo usamos directo
	if name == "" && len(t.Providers) == 1 {
		return &t.Providers[0], true
	}
	
	for i := range t.Providers {
		// Si no piden uno específico, buscamos el marcado como default
		if name == "" && t.Providers[i].IsDefault {
			return &t.Providers[i], true
		}
		// Si piden uno específico, lo devolvemos
		if name != "" && t.Providers[i].Provider == name {
			return &t.Providers[i], true
		}
	}
	return nil, false
}

type Provider struct {
	Provider    string
	APIKeyPlain string
	APIKeyEnc   []byte
	IsDefault   bool
}