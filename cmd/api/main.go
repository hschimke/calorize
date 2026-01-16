package main

import (
	"log"
	"net/http"
	"os"

	"azule.info/calorize/internal/api"
	"azule.info/calorize/internal/database"
)

func main() {
	dbPath := "calorize.db"
	if os.Getenv("DB_PATH") != "" {
		dbPath = os.Getenv("DB_PATH")
	}

	if err := database.InitDB(dbPath); err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}
	defer database.Close()

	log.Println("Database initialized successfully at", dbPath)

	// Start Server
	server := api.NewServer()

	port := "8383"
	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, server); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
