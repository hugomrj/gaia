// cmd/app/main.go
package main

import (
	"log"
	"os"

	"gaia/internal/brain"
	"gaia/internal/providers/mistral"
	"gaia/internal/router"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No se encontró el archivo .env, usando variables del sistema")
	}

	adminUser := os.Getenv("GAIA_ADMIN_USER")
	adminPasswordHash := os.Getenv("GAIA_ADMIN_PASSWORD_HASH")

	if adminUser == "" || adminPasswordHash == "" {
		log.Println("ADVERTENCIA: GAIA_ADMIN_USER o GAIA_ADMIN_PASSWORD_HASH están vacíos en el .env")
	}

	// Puerto configurable (default 8080)
	port := os.Getenv("GAIA_PORT")
	if port == "" {
		port = "8080"
	}

	// Inicializar el cerebro
	llmProvider := mistral.NewClient()
	gaiaBrain := brain.NewBrain(llmProvider)

	appRouter := router.Setup(adminUser, adminPasswordHash, gaiaBrain)

	log.Printf("GAIA iniciando en http://localhost:%s", port)
	if err := appRouter.Run(":" + port); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}