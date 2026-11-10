package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gaia/internal/db"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	path := os.Getenv("GAIA_DB_PATH")
	if path == "" {
		path = "./data/gaia.db"
	}

	// Crear la carpeta si no existe
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatalf("no pude crear %s: %v", filepath.Dir(path), err)
	}

	if err := db.Create(path); err != nil {
		log.Fatalf("error: %v", err)
	}

	fmt.Printf("✅ DB creada en %s\n", path)
	fmt.Println("Ahora podés correr: go run ./cmd/app")
}