// internal/router/routes.go
package router

import (
	"log"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"

	adminPkg "gaia/internal/admin"
	"gaia/internal/admin/service"
	"gaia/internal/brain"
	"gaia/internal/handlers"
	"gaia/internal/tenant"
)

// Setup configura y devuelve el router de Gin con todas las rutas registradas
func Setup(adminUser, adminPasswordHash string, gaiaBrain *brain.Brain, store *tenant.Store) *gin.Engine {
	router := gin.Default()

	// 1. Configuración de vistas y estáticos (Admin Panel)
	router.LoadHTMLGlob("internal/admin/templates/*.html")
	router.Static("/admin/static", "internal/admin/static")
	router.Static("/examples", "examples")

	// 2. Inicializar Handlers
	apiHandler := handlers.NewAPIHandler(gaiaBrain)
	healthHandler := handlers.NewHealthHandler()
	tenantsHandler := adminPkg.NewTenantsHandler(store, adminUser)
	keysHandler := adminPkg.NewKeysHandler(store, adminUser)
	providersHandler := adminPkg.NewProvidersHandler(store, adminUser)

	// 3. Rutas Públicas y de Salud
	router.GET("/health", healthHandler.Health)

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    "GAIA",
			"version": "0.1.0",
			"status":  "running",
		})
	})

	router.GET("/docs", func(c *gin.Context) {
		c.HTML(http.StatusOK, "docs.html", nil)
	})

	// 4. API de GAIA (Protegida por Middleware)
	v1 := router.Group("/v1/:tenant")
	v1.Use(TenantAuth(store)) // <-- MIDDLEWARE APLICADO AQUÍ
	{
		v1.POST("/analyze", apiHandler.Analyze)
		v1.POST("/synthesize", apiHandler.Synthesize)
		v1.POST("/normalize", apiHandler.Normalize)
	}

	// 5. Panel de Administración (Rutas públicas de auth)
	router.GET("/admin/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{"error": ""})
	})

	router.POST("/admin/login", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		log.Printf("DEBUG usuario recibido: %q", username)
		log.Printf("DEBUG usuario esperado: %q", adminUser)

		userOK := username == adminUser
		passwordOK := service.CheckPassword(password, adminPasswordHash)

		if !userOK || !passwordOK {
			c.HTML(http.StatusUnauthorized, "login.html", gin.H{
				"error": "Credenciales inválidas",
			})
			return
		}

		c.SetCookie("gaia_session", "authenticated", 3600, "/", "", false, true)
		c.Redirect(http.StatusFound, "/admin/dashboard")
	})

	// 6. Panel de Administración (Rutas protegidas)
	adminGroup := router.Group("/admin")
	adminGroup.Use(requireAuth)
	{
		adminGroup.GET("/dashboard", func(c *gin.Context) {
			tenants := store.List()

			tenantCount := len(tenants)
			activeCount := 0
			providers := make(map[string]bool)
			for _, t := range tenants {
				if t.IsActive() {
					activeCount++
				}
				for _, p := range t.Providers {
					providers[p.ProviderName] = true
				}
			}

			// Ordenar por ID descendente (más recientes primero)
			sort.Slice(tenants, func(i, j int) bool {
				return tenants[i].ID > tenants[j].ID
			})
			recent := tenants
			if len(recent) > 5 {
				recent = recent[:5]
			}

			c.HTML(http.StatusOK, "dashboard.html", gin.H{
				"User":              adminUser,
				"Active":            "dashboard",
				"tenantCount":       tenantCount,
				"tenantActiveCount": activeCount,
				"providerCount":     len(providers),
				"recentTenants":     recent,
			})
		})

		adminGroup.POST("/logout", func(c *gin.Context) {
			c.SetCookie("gaia_session", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/admin/login")
		})

		// Tenants
		adminGroup.GET("/tenants", tenantsHandler.List)
		adminGroup.POST("/tenants", tenantsHandler.Create)
		adminGroup.GET("/tenants/:slug", tenantsHandler.Detail)        
		adminGroup.POST("/tenants/:slug/edit", tenantsHandler.Update)  
		adminGroup.POST("/tenants/:slug/delete", tenantsHandler.Delete) 
		adminGroup.POST("/tenants/:slug/rotate", tenantsHandler.Rotate) 

		// Keys
		adminGroup.GET("/keys", keysHandler.List)
		adminGroup.POST("/keys", keysHandler.Create)
		adminGroup.POST("/keys/provider", keysHandler.LinkProvider) 
		adminGroup.POST("/keys/:id/toggle", keysHandler.ToggleKey) 
		adminGroup.POST("/keys/:id/delete", keysHandler.Delete)

		// Providers
		adminGroup.GET("/providers", providersHandler.List)
		adminGroup.POST("/providers", providersHandler.Create)
		adminGroup.POST("/providers/:id/edit", providersHandler.Edit)
		adminGroup.POST("/providers/:id/delete", providersHandler.Delete)
	}

	return router
}

// requireAuth es el middleware para proteger las rutas de admin
func requireAuth(c *gin.Context) {
	_, err := c.Cookie("gaia_session")
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/login")
		c.Abort()
		return
	}
	c.Next()
}