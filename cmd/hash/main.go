package main

import (
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	var password string

	fmt.Print("Contraseña: ")
	fmt.Scanln(&password)

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nHash:")
	fmt.Println(string(hash))
}


// go run ./cmd/hash