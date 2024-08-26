package main

import (
	"fmt"
	"smart-docs/core/server"
)

func main() {
	newServer := server.NewServer()
	err := newServer.ListenAndServe()
	if err != nil {
		panic(fmt.Sprintf("Cannot start server: %v", err))
	}
}
