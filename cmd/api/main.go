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

	// CORS Middleware
	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		server.ServeHTTP(w, r)
	})

	port := "8383"
	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, corsHandler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
