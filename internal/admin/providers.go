//	internal/admin/providers.go
package admin

import (
	"net/http"
	"strconv"

	"gaia/internal/tenant"

	"github.com/gin-gonic/gin"
)

type ProvidersHandler struct {
	store     *tenant.Store
	adminUser string
}

func NewProvidersHandler(s *tenant.Store, adminUser string) *ProvidersHandler {
	return &ProvidersHandler{store: s, adminUser: adminUser}
}

// GET /admin/providers
func (h *ProvidersHandler) List(c *gin.Context) {
	providers, _ := h.store.ListProvidersWithID()

	c.HTML(http.StatusOK, "providers.html", gin.H{
		"User":      h.adminUser,
		"Active":    "providers",
		"providers": providers,
	})
}

// POST /admin/providers
func (h *ProvidersHandler) Create(c *gin.Context) {
	name := c.PostForm("name")

	if err := h.store.CreateProvider(name); err != nil {
		providers, _ := h.store.ListProvidersWithID()
		c.HTML(http.StatusBadRequest, "providers.html", gin.H{
			"User":      h.adminUser,
			"Active":    "providers",
			"providers": providers,
			"error":     err.Error(),
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/providers")
}

// POST /admin/providers/:id/edit
func (h *ProvidersHandler) Edit(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	name := c.PostForm("name")

	if err := h.store.UpdateProvider(id, name); err != nil {
		providers, _ := h.store.ListProvidersWithID()
		c.HTML(http.StatusBadRequest, "providers.html", gin.H{
			"User":      h.adminUser,
			"Active":    "providers",
			"providers": providers,
			"error":     err.Error(),
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/providers")
}

// POST /admin/providers/:id/delete
func (h *ProvidersHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	if err := h.store.DeleteProvider(id); err != nil {
		providers, _ := h.store.ListProvidersWithID()
		c.HTML(http.StatusBadRequest, "providers.html", gin.H{
			"User":      h.adminUser,
			"Active":    "providers",
			"providers": providers,
			"error":     err.Error(),
		})
		return
	}

	c.Redirect(http.StatusFound, "/admin/providers")
}