package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"ai-api-portal/backend/internal/db"
	"ai-api-portal/backend/internal/httpapi"
)

type healthResponse struct {
	Status string `json:"status"`
}

func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		return "8080"
	}
	return port
}

func getDBPath() string {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		return "./data.db"
	}
	return dbPath
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	database, err := db.Open(ctx, getDBPath())
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	if err := db.ApplyMigrations(ctx, database); err != nil {
		log.Fatalf("failed to apply migrations: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)
	httpapi.RegisterRoutes(mux, database)

	addr := ":" + getPort()
	log.Printf("backend server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
