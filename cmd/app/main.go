// cmd/app/main.go
package main

import (
	"log"
	"os"

	"gaia/internal/brain"
	"gaia/internal/providers/mistral"
	"gaia/internal/router"
	"gaia/internal/state"

	"github.com/joho/godotenv"
)

func main() {
	// 1. Cargar variables de entorno
	if err := godotenv.Load(); err != nil {
		log.Println("No se encontró el archivo .env, usando variables del sistema")
	}

	adminUser := os.Getenv("GAIA_ADMIN_USER")
	adminPasswordHash := os.Getenv("GAIA_ADMIN_PASSWORD_HASH")

	if adminUser == "" || adminPasswordHash == "" {
		log.Println("ADVERTENCIA: GAIA_ADMIN_USER o GAIA_ADMIN_PASSWORD_HASH están vacíos en el .env")
	}

	// 2. Inicializar el "Cerebro" y dependencias de GAIA
	stateStore := state.NewMemoryStore()
	llmProvider := mistral.NewClient()
	gaiaBrain := brain.NewBrain(llmProvider, stateStore)

	// 3. Configurar todas las rutas (la magia ocurre aquí dentro)
	appRouter := router.Setup(adminUser, adminPasswordHash, gaiaBrain)

	// 4. Iniciar el servidor
	log.Println("GAIA iniciando en http://localhost:8080")
	if err := appRouter.Run(":8080"); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}