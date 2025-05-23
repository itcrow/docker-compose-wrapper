package main

import (
	"log"

	"github.com/batishchev/docker-manager/internal/app"
)

func main() {
	cmd := app.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
