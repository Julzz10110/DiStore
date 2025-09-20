package main

import (
	"context"
	"encoding/json"
	"flag"
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
	"distore/monitoring"
	"distore/replication"
	"distore/storage"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Parse command line arguments
	configFile := flag.String("config", "config.json", "Path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Error loading config from %s: %v", *configFile, err)
	}

	// Set up logging
	monitoring.SetupLogger()

	// Init storage
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
	var authService auth.AuthServiceInterface
	if cfg.Auth.Enabled {
		authService, err = auth.NewAuthService(&cfg.Auth)
		if err != nil {
			log.Printf("Warning: using simple auth service due to error: %v", err)
			authService = auth.NewSimpleAuthService(cfg.Auth.TokenDuration)
		}
		log.Printf("Authentication enabled")
	} else {
		authService = nil
		log.Printf("Authentication disabled")
	}

	// Init metrics
	metrics := monitoring.NewMetrics()
	healthChecker := monitoring.NewHealthChecker(store, replicator)

	// Init handlers
	handlers := api.NewHandlers(store, replicator, authService)

	router := mux.NewRouter()

	// Metrics endpoint
	router.Handle("/metrics", metrics.Handler()).Methods("GET")

	// Health endpoints
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Simple health check
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("GET")

	router.HandleFunc("/health/details", healthChecker.Handler).Methods("GET")

	// Public endpoints
	public := router.PathPrefix("").Subrouter()
	public.HandleFunc("/health", handlers.HealthHandler).Methods("GET")

	// Auth endpoints
	if cfg.Auth.Enabled {
		public.HandleFunc("/auth/token", handlers.TokenHandler).Methods("POST")
	}

	// Protected endpoints
	protected := router.PathPrefix("").Subrouter()
	if cfg.Auth.Enabled && authService != nil {
		protected.Use(auth.AuthMiddleware(authService))
		protected.Use(auth.RBACMiddleware(auth.RoleRead))
		protected.Use(auth.TenantMiddleware)
		protected.Use(auth.KeyAccessMiddleware)
	} else {
		// use public middleware if authentication is disabled
		protected.Use(auth.PublicMiddleware)
	}

	protected.HandleFunc("/set", handlers.SetHandler).Methods("POST")
	protected.HandleFunc("/get/{key}", handlers.GetHandler).Methods("GET")
	protected.HandleFunc("/delete/{key}", handlers.DeleteHandler).Methods("DELETE")
	protected.HandleFunc("/keys", handlers.GetAllHandler).Methods("GET")

	// Internal endpoints for replication
	internal := router.PathPrefix("/internal").Subrouter()
	internal.HandleFunc("/set", handlers.InternalSetHandler).Methods("POST")
	internal.HandleFunc("/delete/{key}", handlers.InternalDeleteHandler).Methods("DELETE")

	// Middleware chain
	router.Use(monitoring.LoggerMiddleware)
	router.Use(createMetricsMiddleware(metrics))
	router.Use(loggingMiddleware)

	// Run background tasks for metrics
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			metrics.UpdateStorageMetrics(store)
			metrics.UpdateReplicationMetrics(replicator)
		}
	}()

	// Launch the server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: router,
	}

	// Run Prometheus metrics (if enabled)
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
		log.Printf("Metrics available on /metrics")
		log.Printf("Health checks available on /health and /health/details")

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

// createMetricsMiddleware creates middleware for collecting metrics
func createMetricsMiddleware(metrics *monitoring.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create ResponseWriter to intercept the status
			rw := &monitoring.ResponseWriter{ResponseWriter: w, StatusCode: 200}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			metrics.ObserveRequest(r.Method, r.URL.Path, rw.StatusCode, duration)

			if rw.StatusCode >= 400 {
				errorType := "client_error"
				if rw.StatusCode >= 500 {
					errorType = "server_error"
				}
				metrics.ObserveError(r.Method, r.URL.Path, errorType)
			}
		})
	}
}

func startPrometheusMetrics(port int) {
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Prometheus metrics available on :%d/metrics", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
