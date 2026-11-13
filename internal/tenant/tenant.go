//  internal/tenant/tenant.go
package tenant

import (
	"math/rand/v2"
	"time"
)

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

func (t *Tenant) GetActiveKey(providerName string) (*APIKey, bool) {
	var selectedProvider *TenantProvider

	if providerName == "" && len(t.Providers) > 0 {
		// Si no piden uno específico, usamos el primero que tenga el tenant
		selectedProvider = &t.Providers[0]
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

	// Retornar la primera llave activa que encontremos
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

// PickRandomActiveKey devuelve una API key activa al azar del proveedor.
// Filtra por: IsActive=true y APIKeyPlain no vacío.
// Retorna (nil, false) si no queda ninguna elegible.
func (tp *TenantProvider) PickRandomActiveKey() (*APIKey, bool) {
	active := make([]*APIKey, 0, len(tp.Keys))
	for i := range tp.Keys {
		if tp.Keys[i].IsActive && tp.Keys[i].APIKeyPlain != "" {
			active = append(active, &tp.Keys[i])
		}
	}
	if len(active) == 0 {
		return nil, false
	}
	return active[rand.IntN(len(active))], true
}

// APIKey es el detalle real (la llave)
type APIKey struct {
	ID          int64
	APIKeyPlain string
	APIKeyEnc   []byte
	IsActive    bool
	UsageCount  int64
}

// MaskedKey devuelve la llave enmascarada para mostrarla en la tabla de forma segura
func (k APIKey) MaskedKey() string {
	if len(k.APIKeyPlain) <= 8 {
		return "••••••••"
	}
	return k.APIKeyPlain[:4] + "••••••••" + k.APIKeyPlain[len(k.APIKeyPlain)-4:]
}



