package main

import (
	"fmt"
	"log"
	"net/http"

	"kuberan/internal/database"
)

func main() {
	fmt.Println("Starting Kuberan backend server...")

	// Initialize database configuration
	dbConfig, err := database.NewConfig()
	if err != nil {
		log.Fatalf("Failed to load database configuration: %v", err)
	}

	// Create database manager
	dbManager, err := database.NewManager(dbConfig)
	if err != nil {
		log.Fatalf("Failed to create database manager: %v", err)
	}

	// Run migrations
	if err := dbManager.Migrate(); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok", "message": "Kuberan API is running"}`))
	})

	log.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
