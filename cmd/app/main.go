// cmd/app/main.go
package main

import (
	"log"
	"os"

	"gaia/internal/brain"
	"gaia/internal/db"
	"gaia/internal/providers/mistral"
	"gaia/internal/router"
	"gaia/internal/tenant"

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
	if len(adminPasswordHash) != 60 {
		log.Printf("⚠️  ADVERTENCIA: hash bcrypt debe tener 60 chars, tiene %d", len(adminPasswordHash))
	}

	port := os.Getenv("GAIA_PORT")
	if port == "" {
		port = "8080"
	}

	// 1. Abrir DB
	dbPath := os.Getenv("GAIA_DB_PATH")
	if dbPath == "" {
		dbPath = "./data/gaia.db"
	}
	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("No se pudo abrir la DB (%s): %v\nCorré primero: go run ./cmd/db-init", dbPath, err)
	}
	defer database.Close()

	// 2. Store (carga tenants en memoria)
	encryptKeys := os.Getenv("ENCRYPT_KEYS") == "true"
	store, err := tenant.NewStore(database, encryptKeys)
	if err != nil {
		log.Fatalf("No se pudo inicializar el store: %v", err)
	}
	log.Printf("GAIA cargó %d tenant(s) en memoria", len(store.List()))

	// 3. Cerebro
	llmProvider := mistral.NewClient()
	gaiaBrain := brain.NewBrain(llmProvider)

	// 4. Router
	appRouter := router.Setup(adminUser, adminPasswordHash, gaiaBrain, store)

	log.Printf("GAIA iniciando en http://localhost:%s", port)
	if err := appRouter.Run(":" + port); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}