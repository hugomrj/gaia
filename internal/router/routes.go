// internal/router/routes.go
package router

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"gaia/internal/admin/service"
	"gaia/internal/brain"
	"gaia/internal/handlers"
)

// Setup configura y devuelve el router de Gin con todas las rutas registradas
func Setup(adminUser, adminPasswordHash string, gaiaBrain *brain.Brain) *gin.Engine {
	router := gin.Default()

	// 1. Configuración de vistas y estáticos (Admin Panel)
	router.LoadHTMLGlob("internal/admin/templates/*.html")
	router.Static("/admin/static", "internal/admin/static")
	router.Static("/examples", "examples")


	// 2. Inicializar Handlers
	apiHandler := handlers.NewAPIHandler(gaiaBrain)
	healthHandler := handlers.NewHealthHandler()

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


	// 4. API de GAIA (Aquí entran y salen los datos del cliente)
	v1 := router.Group("/v1")
	{
		// Cerebro 1: Analiza la intención y devuelve function_name + parámetros
		v1.POST("/analyze", apiHandler.Analyze)

		// Cerebro 2: Sintetiza la respuesta final con los datos del cliente
		v1.POST("/synthesize", apiHandler.Synthesize)

		// Normalizer: convierte valores crudos a formato estándar
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
	admin := router.Group("/admin")
	admin.Use(requireAuth)
	{
		admin.GET("/dashboard", func(c *gin.Context) {
			c.HTML(http.StatusOK, "dashboard.html", gin.H{"user": adminUser})
		})

		admin.POST("/logout", func(c *gin.Context) {
			c.SetCookie("gaia_session", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/admin/login")
		})
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

