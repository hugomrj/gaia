package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"gaia/internal/admin/service"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No se encontró el archivo .env")
	}

	adminUser := os.Getenv("GAIA_ADMIN_USER")
	adminPasswordHash := os.Getenv("GAIA_ADMIN_PASSWORD_HASH")

	router := gin.Default()

	router.LoadHTMLGlob("internal/admin/templates/*.html")

	router.Static("/admin/static", "internal/admin/static")

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    "GAIA",
			"version": "0.1.0",
			"status":  "running",
		})
	})

	router.GET("/admin/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", nil)
	})




	router.POST("/admin/login", func(c *gin.Context) {

		username := c.PostForm("username")
		password := c.PostForm("password")
	
		log.Printf("DEBUG usuario recibido: %q", username)
		log.Printf("DEBUG usuario esperado: %q", adminUser)
		log.Printf("DEBUG password recibida: %t", password != "")
		log.Printf("DEBUG hash cargado: %t", adminPasswordHash != "")
	
		userOK := username == adminUser
		passwordOK := service.CheckPassword(password, adminPasswordHash)
	
		log.Printf("DEBUG usuario correcto: %t", userOK)
		log.Printf("DEBUG contraseña correcta: %t", passwordOK)
	
		if !userOK || !passwordOK {
			c.HTML(http.StatusUnauthorized, "login.html", gin.H{
				"error": "Credenciales inválidas",
			})
			return
		}
	
		c.Redirect(http.StatusFound, "/admin/dashboard")
	})


	


	router.GET("/admin/dashboard", func(c *gin.Context) {
		c.HTML(http.StatusOK, "dashboard.html", nil)
	})

	router.Run(":8080")
}