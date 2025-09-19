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
	"distore/auth"
	"distore/config"
	"distore/replication"
	"distore/storage"

	"github.com/gorilla/mux"
)

func main() {
	cfg, err := config.LoadConfig("config.test.json")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Init store
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

	// Init replication
	replicator := replication.NewReplicator(cfg.Nodes, cfg.ReplicaCount)

	// Init authentication
	var authService *auth.AuthService
	if cfg.Auth.Enabled {
		authService, err = auth.NewAuthService(&cfg.Auth)
		if err != nil {
			log.Fatalf("Error creating auth service: %v", err)
		}
	}

	// Init handlers
	handlers := api.NewHandlers(store, replicator, authService)

	router := mux.NewRouter()

	// Public endpoints - access without authentication
	public := router.PathPrefix("").Subrouter()
	public.Use(auth.PublicMiddleware)
	public.HandleFunc("/health", handlers.HealthHandler).Methods("GET")

	// Auth endpoints
	if cfg.Auth.Enabled {
		public.HandleFunc("/auth/token", handlers.TokenHandler).Methods("POST")
	}

	// Protected endpoints - require authentication
	protected := router.PathPrefix("").Subrouter()
	if cfg.Auth.Enabled {
		protected.Use(auth.AuthMiddleware(authService))
		protected.Use(auth.RBACMiddleware(auth.RoleRead))
		protected.Use(auth.TenantMiddleware)
		protected.Use(auth.KeyAccessMiddleware)
	}

	protected.HandleFunc("/set", handlers.SetHandler).Methods("POST")
	protected.HandleFunc("/get/{key}", handlers.GetHandler).Methods("GET")
	protected.HandleFunc("/delete/{key}", handlers.DeleteHandler).Methods("DELETE")
	protected.HandleFunc("/keys", handlers.GetAllHandler).Methods("GET")

	// Internal endpoints for replication - access without authentication
	internal := router.PathPrefix("/internal").Subrouter()
	internal.Use(auth.PublicMiddleware)
	internal.HandleFunc("/set", handlers.InternalSetHandler).Methods("POST")
	internal.HandleFunc("/delete/{key}", handlers.InternalDeleteHandler).Methods("DELETE")

	router.Use(loggingMiddleware)

	// Launch the server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: router,
	}

	// Run Prometheus metrics if enabled
	if cfg.PrometheusPort > 0 {
		go startPrometheusMetrics(cfg.PrometheusPort)
	}

	go func() {
		log.Printf("Server starting on port %d", cfg.HTTPPort)
		if cfg.TLS.Enabled {
			log.Printf("TLS enabled with cert: %s, key: %s", cfg.TLS.CertFile, cfg.TLS.KeyFile)
		}
		if cfg.Auth.Enabled {
			log.Printf("Authentication enabled")
		}

		var err error
		if cfg.TLS.Enabled {
			err = server.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		} else {
			err = server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

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

func startPrometheusMetrics(port int) {
	log.Printf("Prometheus metrics available on :%d/metrics", port)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
