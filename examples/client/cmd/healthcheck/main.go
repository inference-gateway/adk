package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/inference-gateway/adk/client"
)

func main() {
	fmt.Println("🏥 Starting Health Check Example...")

	// Create client
	client := client.NewClient("http://localhost:8080")

	// Monitor agent health
	ctx := context.Background()

	// Single health check
	health, err := client.GetHealth(ctx)
	if err != nil {
		log.Printf("Health check failed: %v", err)
		return
	}

	fmt.Printf("Agent health: %s\n", health.Status)

	// Periodic health monitoring
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		health, err := client.GetHealth(ctx)
		if err != nil {
			log.Printf("Health check failed: %v", err)
			continue
		}

		switch health.Status {
		case "healthy":
			fmt.Printf("[%s] Agent is healthy\n", time.Now().Format("15:04:05"))
		case "degraded":
			fmt.Printf("[%s] Agent is degraded - some functionality may be limited\n", time.Now().Format("15:04:05"))
		case "unhealthy":
			fmt.Printf("[%s] Agent is unhealthy - may not be able to process requests\n", time.Now().Format("15:04:05"))
		default:
			fmt.Printf("[%s] Unknown health status: %s\n", time.Now().Format("15:04:05"), health.Status)
		}
	}
}
