//	internal/admin/tenants.go
package admin

import (
	"net/http"
	"strconv"

	"gaia/internal/tenant"

	"github.com/gin-gonic/gin"
)

type TenantsHandler struct {
	store *tenant.Store
}

func NewTenantsHandler(s *tenant.Store) *TenantsHandler {
	return &TenantsHandler{store: s}
}

// GET /admin/tenants — lista todos los tenants y muestra el form de creación
func (h *TenantsHandler) List(c *gin.Context) {
	c.HTML(http.StatusOK, "tenants.html", gin.H{
		"tenants": h.store.List(),
	})
}

// POST /admin/tenants — crear un tenant nuevo
func (h *TenantsHandler) Create(c *gin.Context) {
	slug := c.PostForm("slug")
	name := c.PostForm("name")
	provider := c.PostForm("provider")
	apiKey := c.PostForm("api_key")

	t, token, err := h.store.Create(slug, name, provider, apiKey)
	if err != nil {
		// Si falla, volvemos a mostrar la tabla con el mensaje de error
		c.HTML(http.StatusBadRequest, "tenants.html", gin.H{
			"tenants": h.store.List(),
			"error":   err.Error(),
		})
		return
	}

	// Si funciona, mostramos la pantalla del token (se ve una sola vez)
	c.HTML(http.StatusOK, "tenant_created.html", gin.H{
		"tenant": t,
		"token":  token,
	})
}

// POST /admin/tenants/:id/delete — borrar tenant
func (h *TenantsHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	
	if err := h.store.Delete(id); err != nil {
		c.HTML(http.StatusBadRequest, "tenants.html", gin.H{
			"tenants": h.store.List(),
			"error":   "Error al borrar: " + err.Error(),
		})
		return
	}
	
	// Éxito: volvemos a la lista
	c.Redirect(http.StatusFound, "/admin/tenants")
}

// POST /admin/tenants/:id/rotate — regenerar el token de acceso
func (h *TenantsHandler) Rotate(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	
	t, token, err := h.store.RotateToken(id)
	if err != nil {
		c.HTML(http.StatusBadRequest, "tenants.html", gin.H{
			"tenants": h.store.List(),
			"error":   "Error al rotar token: " + err.Error(),
		})
		return
	}
	
	// Reutilizamos la misma pantalla del token, pero con un flag para el título
	c.HTML(http.StatusOK, "tenant_created.html", gin.H{
		"tenant": t,
		"token":  token,
		"rotate": true,
	})
}