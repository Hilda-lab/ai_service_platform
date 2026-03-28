package main

import (
	"log"

	"ai-service-platform/backend/internal/api/http/router"
)

func main() {
	r := router.NewRouter()
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("server failed to start: %v", err)
	}
}
