package server

import (
	"fmt"
	"net/http"
	"smart-docs/core/db"
	"time"
)

type Server struct {
	port int
	db   db.Service
}

func NewServer() *http.Server {
	NewServer := &Server{
		port: 8080,
		db:   db.New(),
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(),
		IdleTimeout:  5 * time.Minute,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
