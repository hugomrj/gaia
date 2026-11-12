package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	force := flag.Bool("force", false, "Borrar sin pedir confirmación")
	flag.Parse()

	_ = godotenv.Load()

	dbPath := os.Getenv("GAIA_DB_PATH")
	if dbPath == "" {
		dbPath = "./data/gaia.db"
	}

	// Archivos generados por SQLite (el principal y los de WAL)
	files := []string{dbPath, dbPath + "-shm", dbPath + "-wal"}

	if !*force {
		fmt.Printf("⚠️  Esto borrará la base de datos en %s y sus archivos auxiliares.\n", dbPath)
		fmt.Print("¿Estás seguro? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		resp, _ := reader.ReadString('\n')
		resp = strings.ToLower(strings.TrimSpace(resp))
		if resp != "y" && resp != "s" && resp != "si" {
			fmt.Println("Cancelado.")
			return
		}
	}

	deletedAny := false
	for _, f := range files {
		err := os.Remove(f)
		if err == nil {
			fmt.Printf("🗑️  Borrado: %s\n", f)
			deletedAny = true
		} else if !os.IsNotExist(err) {
			log.Printf("Error al borrar %s: %v", f, err)
		}
	}

	if !deletedAny {
		fmt.Println("No se encontró ninguna base de datos para borrar en esa ruta.")
	} else {
		fmt.Println("✅ Base de datos eliminada. Ahora podés correr 'go run ./cmd/db-init'.")
	}
}