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

	// Inicializar el cerebro (sin state store)
	llmProvider := mistral.NewClient()
	gaiaBrain := brain.NewBrain(llmProvider)

	appRouter := router.Setup(adminUser, adminPasswordHash, gaiaBrain)

	log.Println("GAIA iniciando en http://localhost:8080")
	if err := appRouter.Run(":8080"); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}