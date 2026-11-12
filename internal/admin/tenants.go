//	internal/admin/tenants.go
package admin

import (
	"net/http"
	"strconv"

	"gaia/internal/tenant"

	"github.com/gin-gonic/gin"
)

type TenantsHandler struct {
	store     *tenant.Store
	adminUser string
}

// El constructor ahora recibe el nombre del admin
func NewTenantsHandler(s *tenant.Store, adminUser string) *TenantsHandler {
	return &TenantsHandler{
		store:     s,
		adminUser: adminUser,
	}
}

// GET /admin/tenants
func (h *TenantsHandler) List(c *gin.Context) {
	c.HTML(http.StatusOK, "tenants.html", gin.H{
		"User":    h.adminUser,
		"Active":  "tenants",
		"tenants": h.store.List(),
	})
}

// POST /admin/tenants
func (h *TenantsHandler) Create(c *gin.Context) {
	slug := c.PostForm("slug")
	name := c.PostForm("name")
	
	// Ya NO leemos provider ni api_key porque el tenant nace independiente

	t, token, err := h.store.Create(slug, name)
	if err != nil {
		c.HTML(http.StatusBadRequest, "tenants.html", gin.H{
			"User":    h.adminUser,
			"Active":  "tenants",
			"tenants": h.store.List(),
			"error":   err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "tenant_created.html", gin.H{
		"tenant": t,
		"token":  token,
	})
}

// POST /admin/tenants/:id/delete
func (h *TenantsHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	
	if err := h.store.Delete(id); err != nil {
		c.HTML(http.StatusBadRequest, "tenants.html", gin.H{
			"User":    h.adminUser,
			"Active":  "tenants",
			"tenants": h.store.List(),
			"error":   "Error al borrar: " + err.Error(),
		})
		return
	}
	
	c.Redirect(http.StatusFound, "/admin/tenants")
}

// POST /admin/tenants/:id/rotate
func (h *TenantsHandler) Rotate(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	
	t, token, err := h.store.RotateToken(id)
	if err != nil {
		c.HTML(http.StatusBadRequest, "tenants.html", gin.H{
			"User":    h.adminUser,
			"Active":  "tenants",
			"tenants": h.store.List(),
			"error":   "Error al rotar token: " + err.Error(),
		})
		return
	}
	
	c.HTML(http.StatusOK, "tenant_created.html", gin.H{
		"tenant": t,
		"token":  token,
		"rotate": true,
	})
}