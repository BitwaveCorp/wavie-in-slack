package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bitwavecorp/wavie-in-slack/services/rag_svc/internal/api"
	"github.com/bitwavecorp/wavie-in-slack/services/rag_svc/internal/config"
)

func main() {
	// Create context
	ctx := context.Background()
	
	// Load configuration
	cfg := config.LoadConfig()
	
	// Validate required configuration
	if cfg.GCP.ProjectID == "" || cfg.GCP.StorageBucket == "" || cfg.OpenAI.APIKey == "" {
		log.Fatal("Required configuration missing. Please set GCP_PROJECT_ID, GCP_STORAGE_BUCKET, and OPENAI_API_KEY")
	}
	
	if cfg.Vertex.IndexID == "" || cfg.Vertex.IndexEndpointID == "" || cfg.Vertex.DeployedIndexID == "" {
		log.Fatal("Required Vertex AI configuration missing. Please set VERTEX_INDEX_ID, VERTEX_INDEX_ENDPOINT_ID, and VERTEX_DEPLOYED_INDEX_ID")
	}
	
	// Create and start server
	server, err := api.NewServer(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	
	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-c
		log.Println("Shutting down server...")
		if err := server.Close(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		os.Exit(0)
	}()
	
	// Start server
	log.Fatal(server.Start())
}
