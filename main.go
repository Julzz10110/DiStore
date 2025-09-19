package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"distore/api"
	"distore/config"
	"distore/replication"
	"distore/storage"

	"github.com/gorilla/mux"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Init the storage
	var store storage.Storage
	if cfg.DataDir != "" {
		store, err = storage.NewDiskStorage(cfg.DataDir)
		if err != nil {
			log.Fatalf("Error creating disk storage: %v", err)
		}
	} else {
		store = storage.NewMemoryStorage()
	}
	defer store.Close()

	// Init the replication
	replicator := replication.NewReplicator(cfg.Nodes, cfg.ReplicaCount)

	// Init handlers
	handlers := api.NewHandlers(store, replicator)

	// Setting up routes with gorilla/mux for more precise routing
	router := mux.NewRouter()

	// Public endpoints
	router.HandleFunc("/set", handlers.SetHandler).Methods("POST")
	router.HandleFunc("/get/{key}", handlers.GetHandler).Methods("GET")
	router.HandleFunc("/delete/{key}", handlers.DeleteHandler).Methods("DELETE")
	router.HandleFunc("/keys", handlers.GetAllHandler).Methods("GET")
	router.HandleFunc("/health", handlers.HealthHandler).Methods("GET")

	// Internal endpoints for replication
	router.HandleFunc("/internal/set", handlers.InternalSetHandler).Methods("POST")
	router.HandleFunc("/internal/delete/{key}", handlers.InternalDeleteHandler).Methods("DELETE")

	// Middleware for logging
	router.Use(loggingMiddleware)

	// Launch the server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: router,
	}

	go func() {
		log.Printf("Server starting on port %d", cfg.HTTPPort)
		log.Printf("Available endpoints:")
		log.Printf("  POST   /set")
		log.Printf("  GET    /get/{key}")
		log.Printf("  DELETE /delete/{key}")
		log.Printf("  GET    /keys")
		log.Printf("  GET    /health")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for signals for a graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// Middleware for request logging
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
