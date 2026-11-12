//	internal/admin/tenants.go
package admin

import (
	"net/http"

	"gaia/internal/tenant"

	"github.com/gin-gonic/gin"
)

type TenantsHandler struct {
	store     *tenant.Store
	adminUser string
}

func NewTenantsHandler(s *tenant.Store, adminUser string) *TenantsHandler {
	return &TenantsHandler{store: s, adminUser: adminUser}
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

// GET /admin/tenants/:slug
func (h *TenantsHandler) Detail(c *gin.Context) {
	slug := c.Param("slug")
	
	t, ok := h.store.GetBySlug(slug)
	if !ok {
		c.Redirect(http.StatusFound, "/admin/tenants")
		return
	}

	c.HTML(http.StatusOK, "tenant_detail.html", gin.H{
		"User":   h.adminUser,
		"Active": "tenants",
		"tenant": t,
	})
}

// POST /admin/tenants/:slug/edit
func (h *TenantsHandler) Update(c *gin.Context) {
	slug := c.Param("slug")
	name := c.PostForm("name")

	t, ok := h.store.GetBySlug(slug)
	if !ok {
		c.Redirect(http.StatusFound, "/admin/tenants")
		return
	}

	if err := h.store.UpdateName(t.ID, name); err != nil {
		c.Redirect(http.StatusFound, "/admin/tenants/"+slug)
		return
	}

	c.Redirect(http.StatusFound, "/admin/tenants/"+slug)
}

// POST /admin/tenants/:slug/delete
func (h *TenantsHandler) Delete(c *gin.Context) {
	slug := c.Param("slug")
	
	t, ok := h.store.GetBySlug(slug)
	if !ok {
		c.Redirect(http.StatusFound, "/admin/tenants")
		return
	}
	
	h.store.Delete(t.ID)
	c.Redirect(http.StatusFound, "/admin/tenants")
}

// POST /admin/tenants/:slug/rotate
func (h *TenantsHandler) Rotate(c *gin.Context) {
	slug := c.Param("slug")
	
	t, ok := h.store.GetBySlug(slug)
	if !ok {
		c.Redirect(http.StatusFound, "/admin/tenants")
		return
	}
	
	t, token, err := h.store.RotateToken(t.ID)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/tenants/"+slug)
		return
	}

	c.HTML(http.StatusOK, "tenant_created.html", gin.H{
		"tenant": t,
		"token":  token,
		"rotate": true,
	})
}