package main

import (
	"log"

	"github.com/itcrow/docker-compose-wrapper/internal/app"
)

func main() {
	cmd := app.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
