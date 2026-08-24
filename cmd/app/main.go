package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	router.LoadHTMLGlob("internal/admin/templates/*.html")

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

	router.GET("/admin/dashboard", func(c *gin.Context) {
		c.HTML(http.StatusOK, "dashboard.html", nil)
	})
	
	router.Static("/admin/static", "internal/admin/static")

	router.Run(":8080")
}