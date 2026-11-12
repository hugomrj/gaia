//  internal/tenant/tenant.go  
package tenant

import "time"

type Tenant struct {
	ID         int64
	Slug       string
	Name       string
	TokenHash  string
	TokenPlain string // <-- NUEVO CAMPO
	Status     string
	Providers  []TenantProvider
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (t *Tenant) IsActive() bool {
	return t.Status == "active"
}

// GetActiveKey busca la API Key que se debe usar según el proveedor solicitado.
// Si providerName está vacío, busca la cabecera marcada como default.
func (t *Tenant) GetActiveKey(providerName string) (*APIKey, bool) {
	var selectedProvider *TenantProvider

	if providerName == "" {
		// Buscar el proveedor por defecto
		for i := range t.Providers {
			if t.Providers[i].IsDefault {
				selectedProvider = &t.Providers[i]
				break
			}
		}
		// Si no hay default, usar el primero que tenga
		if selectedProvider == nil && len(t.Providers) > 0 {
			selectedProvider = &t.Providers[0]
		}
	} else {
		// Buscar el proveedor por nombre
		for i := range t.Providers {
			if t.Providers[i].ProviderName == providerName {
				selectedProvider = &t.Providers[i]
				break
			}
		}
	}

	if selectedProvider == nil {
		return nil, false
	}

	// Retornar la primera llave activa
	for i := range selectedProvider.Keys {
		if selectedProvider.Keys[i].IsActive {
			return &selectedProvider.Keys[i], true
		}
	}

	return nil, false
}

// TenantProvider es la tabla intermedia (Cabecera)
type TenantProvider struct {
	ID           int64
	TenantID     int64
	ProviderName string
	IsDefault    bool
	Keys         []APIKey // Detalles
}

// APIKey es el detalle real (la llave)
type APIKey struct {
	ID          int64
	APIKeyPlain string
	APIKeyEnc   []byte
	IsActive    bool
}