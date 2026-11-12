// internal/admin/keys.go
package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"gaia/internal/tenant"

	"github.com/gin-gonic/gin"
)

type KeysHandler struct {
	store     *tenant.Store
	adminUser string
}

func NewKeysHandler(s *tenant.Store, adminUser string) *KeysHandler {
	return &KeysHandler{store: s, adminUser: adminUser}
}

// GET /admin/keys
func (h *KeysHandler) List(c *gin.Context) {
	tenantSlug := c.Query("tenant")
	activeProvider := c.Query("provider")

	var currentTenant *tenant.Tenant
	tenants := h.store.List()

	// Si no hay tenant en la URL, usamos el primero por defecto
	if tenantSlug == "" && len(tenants) > 0 {
		currentTenant = tenants[0]
		tenantSlug = currentTenant.Slug
	} else if tenantSlug != "" {
		if t, ok := h.store.GetBySlug(tenantSlug); ok {
			currentTenant = t
		}
	}

	// Si no hay provider seleccionado, usamos el primero que tenga el tenant
	if activeProvider == "" && currentTenant != nil && len(currentTenant.Providers) > 0 {
		activeProvider = currentTenant.Providers[0].ProviderName
	}

	providers, _ := h.store.ListProviders()

	c.HTML(http.StatusOK, "keys.html", gin.H{
		"User":           h.adminUser,
		"Active":         "keys",
		"currentTenant":  currentTenant,
		"activeProvider": activeProvider,
		"tenants":        tenants,
		"providers":      providers,
	})
}

// POST /admin/keys/provider (Habilitar Proveedor)
func (h *KeysHandler) LinkProvider(c *gin.Context) {
	tenantID := c.PostForm("tenant_id")
	providerName := c.PostForm("provider")
	returnSlug := c.PostForm("return_slug")

	var tID int64
	if _, err := fmt.Sscan(tenantID, &tID); err != nil {
		c.String(http.StatusBadRequest, "Tenant inválido")
		return
	}

	if err := h.store.LinkProvider(tID, providerName); err != nil {
		// Si falla (ej. ya existe), recargamos igual
		c.Redirect(http.StatusFound, "/admin/keys?tenant="+returnSlug)
		return
	}

	c.Redirect(http.StatusFound, "/admin/keys?tenant="+returnSlug+"&provider="+providerName)
}






// POST /admin/keys (Agregar Key a un Proveedor ya habilitado)
func (h *KeysHandler) Create(c *gin.Context) {
	tenantID := c.PostForm("tenant_id")
	providerName := c.PostForm("provider")
	apiKey := c.PostForm("api_key")
	returnSlug := c.PostForm("return_slug")

	var tID int64
	if _, err := fmt.Sscan(tenantID, &tID); err != nil {
		c.String(http.StatusBadRequest, "Tenant inválido")
		return
	}

	// Ya no le pasamos el isDefault
	if err := h.store.AddKey(tID, providerName, apiKey); err != nil {
		c.String(http.StatusBadRequest, "Error: %v", err)
		return
	}

	c.Redirect(http.StatusFound, "/admin/keys?tenant="+returnSlug+"&provider="+providerName)
}





// POST /admin/keys/:id/delete
func (h *KeysHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	h.store.DeleteKey(id)

	tenantSlug := c.Query("tenant")
	providerName := c.Query("provider")
	
	if tenantSlug != "" {
		c.Redirect(http.StatusFound, "/admin/keys?tenant="+tenantSlug+"&provider="+providerName)
	} else {
		c.Redirect(http.StatusFound, "/admin/keys")
	}
}





// POST /admin/keys/:id/toggle
func (h *KeysHandler) ToggleKey(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	h.store.ToggleKeyStatus(id)

	tenantSlug := c.Query("tenant")
	providerName := c.Query("provider")
	
	if tenantSlug != "" {
		c.Redirect(http.StatusFound, "/admin/keys?tenant="+tenantSlug+"&provider="+providerName)
	} else {
		c.Redirect(http.StatusFound, "/admin/keys")
	}
}




