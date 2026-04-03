package main

import (
	"log"

	"github.com/DioSaputra28/vps-nat/internal/app"
)

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatalf("failed to bootstrap application: %v", err)
	}

	defer func() {
		if err := application.Close(); err != nil {
			log.Printf("failed to close application cleanly: %v", err)
		}
	}()

	log.Printf("starting %s on %s:%d", application.Config.App.Name, application.Config.HTTP.Host, application.Config.HTTP.Port)

	if err := application.Run(); err != nil {
		log.Fatalf("application stopped with error: %v", err)
	}
}
