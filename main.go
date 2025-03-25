package main

import (
	"fmt"
	"log"
	"os"
	"smart-docs/core/server"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	dirs := []string{
		"./data/files",
		"./data/images",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			panic(fmt.Sprintf("Cannot create directory %s: %v", dir, err))
		}
	}

	newServer := server.NewServer()
	err := newServer.ListenAndServe()
	if err != nil {
		panic(fmt.Sprintf("Cannot start server: %v", err))
	}
}
