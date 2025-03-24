package main

import (
	"fmt"
	"log"
	"smart-docs/core/server"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	newServer := server.NewServer()
	err := newServer.ListenAndServe()
	if err != nil {
		panic(fmt.Sprintf("Cannot start server: %v", err))
	}
}
