// internal/admin/keys.go
package admin

import (
	"fmt"
	"net/http"

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
	keys, err := h.store.ListAllKeys()
	if err != nil {
		c.String(http.StatusInternalServerError, "Error al cargar keys: %v", err)
		return
	}

	providers, err := h.store.ListProviders()
	if err != nil {
		c.String(http.StatusInternalServerError, "Error al cargar providers: %v", err)
		return
	}

	c.HTML(http.StatusOK, "keys.html", gin.H{
		"User":      h.adminUser,
		"Active":    "keys",
		"keys":      keys,
		"tenants":   h.store.List(),
		"providers": providers,
	})
}

// POST /admin/keys
func (h *KeysHandler) Create(c *gin.Context) {
	tenantID := c.PostForm("tenant_id")
	providerName := c.PostForm("provider")
	apiKey := c.PostForm("api_key")
	isDefault := c.PostForm("is_default") == "1"

	// Parseo simple del ID
	var tID int64
	if _, err := fmt.Sscan(tenantID, &tID); err != nil {
		c.String(http.StatusBadRequest, "Tenant inválido")
		return
	}

	if err := h.store.AddKey(tID, providerName, apiKey, isDefault); err != nil {
		c.HTML(http.StatusBadRequest, "keys.html", gin.H{
			"User":      h.adminUser,
			"Active":    "keys",
			"keys":      h.mustListKeys(),
			"tenants":   h.store.List(),
			"providers": h.mustListProviders(),
			"error":     err.Error(),
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/keys")
}

func (h *KeysHandler) mustListKeys() []tenant.GlobalKey {
	k, _ := h.store.ListAllKeys()
	return k
}

func (h *KeysHandler) mustListProviders() []string {
	p, _ := h.store.ListProviders()
	return p
}