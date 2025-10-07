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
	"distore/cluster"
	"distore/config"
	"distore/monitoring"
	"distore/replication"
	"distore/storage"

	"github.com/gorilla/mux"
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

	// Init base storage
	var baseStore storage.Storage
	if cfg.DataDir != "" {
		baseStore, err = storage.NewDiskStorage(cfg.DataDir)
		if err != nil {
			log.Fatalf("Error creating disk storage: %v", err)
		}
	} else {
		baseStore = storage.NewMemoryStorage()
	}
	defer baseStore.Close()

	// Wrapping storage with advanced capabilities
	store := wrapStorageWithAdvancedFeatures(baseStore, cfg)

	// Init replication
	replicator := replication.NewReplicator(cfg.Nodes, cfg.ReplicaCount)

	// Initialize rebalancer with self address
	selfAddr := fmt.Sprintf("localhost:%d", cfg.HTTPPort)
	rebalancer := cluster.NewRebalancer(store, replicator, selfAddr)

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
	// inject rebalancer (optional)
	// NOTE: rebalancer field is optional, set directly
	handlers.Rebalancer = rebalancer

	router := mux.NewRouter()

	// Metrics endpoint
	router.Handle("/metrics", metrics.Handler()).Methods("GET")

	// Health endpoints
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
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

	// Advanced data operations endpoints
	advanced := router.PathPrefix("/advanced").Subrouter()
	if cfg.Auth.Enabled {
		advanced.Use(auth.AuthMiddleware(authService))
		advanced.Use(auth.RBACMiddleware(auth.RoleWrite))
	} else {
		advanced.Use(auth.PublicMiddleware) // important for working without authentication
	}

	advanced.HandleFunc("/ttl", handlers.TTLHandler).Methods("POST")
	advanced.HandleFunc("/increment", handlers.IncrementHandler).Methods("POST")
	advanced.HandleFunc("/batch", handlers.BatchHandler).Methods("POST")
	advanced.HandleFunc("/cas", handlers.CASHandler).Methods("POST")
	advanced.HandleFunc("/performance/stats", handlers.PerformanceStatsHandler).Methods("GET")
	advanced.HandleFunc("/cache/preload", handlers.CachePreloadHandler).Methods("POST")
	advanced.HandleFunc("/lock/{key}", handlers.AcquireLockHandler).Methods("POST")
	advanced.HandleFunc("/lock/{key}", handlers.ReleaseLockHandler).Methods("DELETE")

	// Protected endpoints
	protected := router.PathPrefix("").Subrouter()
	if cfg.Auth.Enabled && authService != nil {
		protected.Use(auth.AuthMiddleware(authService))
		protected.Use(auth.RBACMiddleware(auth.RoleRead))
		protected.Use(auth.TenantMiddleware)
		protected.Use(auth.KeyAccessMiddleware)
	} else {
		protected.Use(auth.PublicMiddleware)
	}

	protected.HandleFunc("/set", handlers.SetHandler).Methods("POST")
	protected.HandleFunc("/get/{key}", handlers.GetHandler).Methods("GET")
	protected.HandleFunc("/delete/{key}", handlers.DeleteHandler).Methods("DELETE")
	protected.HandleFunc("/keys", handlers.GetAllHandler).Methods("GET")

	// Internal endpoints for replication
	internal := router.PathPrefix("/internal").Subrouter()
	internal.Use(auth.PublicMiddleware)
	internal.HandleFunc("/set", handlers.InternalSetHandler).Methods("POST")
	internal.HandleFunc("/delete/{key}", handlers.InternalDeleteHandler).Methods("DELETE")
	internal.HandleFunc("/get/{key}", handlers.InternalGetHandler).Methods("GET")

	// Admin endpoints
	admin := router.PathPrefix("/admin").Subrouter()
	if cfg.Auth.Enabled && authService != nil {
		admin.Use(auth.AuthMiddleware(authService))
		admin.Use(auth.RBACMiddleware(auth.RoleAdmin))
	} else {
		admin.Use(auth.PublicMiddleware)
	}
	admin.HandleFunc("/nodes", handlers.ListNodesHandler).Methods("GET")
	admin.HandleFunc("/nodes", handlers.AddNodeHandler).Methods("POST")
	admin.HandleFunc("/nodes/{node}", handlers.RemoveNodeHandler).Methods("DELETE")
	admin.HandleFunc("/rebalance", handlers.TriggerRebalanceHandler).Methods("POST")
	admin.HandleFunc("/config", handlers.GetConfigHandler).Methods("GET")
	admin.HandleFunc("/config", handlers.UpdateConfigHandler).Methods("PATCH")
	admin.HandleFunc("/backup", handlers.BackupHandler).Methods("POST")
	admin.HandleFunc("/restore", handlers.RestoreHandler).Methods("POST")

	// Middleware chain
	router.Use(monitoring.LoggerMiddleware)
	router.Use(createMetricsMiddleware(metrics))
	router.Use(loggingMiddleware)

	// Run background tasks for metrics
	go startBackgroundTasks(store, replicator, metrics)

	// Launch the server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: router,
	}

	go func() {
		log.Printf("Server starting on port %d", cfg.HTTPPort)
		log.Printf("Advanced features enabled: TTL, Atomic ops, Batch ops, CAS")
		log.Printf("Endpoints available:")
		log.Printf("  POST   /advanced/ttl")
		log.Printf("  POST   /advanced/increment")
		log.Printf("  POST   /advanced/batch")
		log.Printf("  POST   /advanced/cas")
		log.Printf("  POST   /advanced/lock/{key}")
		log.Printf("  DELETE /advanced/lock/{key}")

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

	// Graceful shutdown
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

// wrapStorageWithAdvancedFeatures wraps basic storage with advanced features
func wrapStorageWithAdvancedFeatures(baseStore storage.Storage, cfg *config.Config) storage.Storage {
	store := baseStore

	// 1. Add TTL support (if enabled)
	if cfg.Advanced.TTLEnabled {
		cleanupInterval := time.Duration(cfg.Advanced.CleanupInterval) * time.Second
		if cleanupInterval == 0 {
			cleanupInterval = 1 * time.Minute
		}
		ttlStore := storage.NewTTLStorage(store, cleanupInterval)
		store = ttlStore
		log.Printf("TTL support enabled (cleanup interval: %v)", cleanupInterval)
	}

	// 2. Add performance optimizations
	if cfg.Performance.Enabled {
		// hot data caching
		if cfg.Performance.CacheSize > 0 {
			cacheTTL := time.Duration(cfg.Performance.CacheTTL) * time.Second
			if cacheTTL == 0 {
				cacheTTL = 5 * time.Minute
			}
			cacheStore := storage.NewCacheStorage(
				store,
				storage.LRU,
				cfg.Performance.CacheSize,
				cacheTTL,
			)
			store = cacheStore
			log.Printf("Cache enabled (size: %d, TTL: %v)",
				cfg.Performance.CacheSize, cacheTTL)
		}

		// data compression
		if cfg.Performance.CompressionEnabled {
			threshold := cfg.Performance.CompressionThreshold
			if threshold == 0 {
				threshold = 1024 // 1KB by default
			}
			compressedStore := storage.NewCompressedStorage(
				store,
				storage.CompressionGZIP,
				threshold,
			)
			store = compressedStore
			log.Printf("Compression enabled (threshold: %d bytes)", threshold)
		}

		// Bloom filter for quick negative checks
		if cfg.Performance.BloomFilterEnabled {
			expectedElements := cfg.Performance.ExpectedElements
			if expectedElements == 0 {
				expectedElements = 10000
			}
			optimizedStore := storage.NewOptimizedStorage(store, expectedElements)
			store = optimizedStore
			log.Printf("Bloom filter enabled (expected elements: %d)", expectedElements)
		}

		// Write-ahead log for durability
		if cfg.Performance.WALEnabled && cfg.DataDir != "" {
			walStore, err := storage.NewWALStorage(store, cfg.DataDir)
			if err == nil {
				store = walStore
				log.Printf("Write-ahead log enabled")
			} else {
				log.Printf("WAL initialization failed: %v", err)
			}
		}
	}

	// 3. Add atomic operations (if enabled)
	if cfg.Advanced.AtomicEnabled {
		atomicStore := storage.NewAtomicStorage(store)
		store = atomicStore
		log.Printf("Atomic operations enabled")
	}

	// 4. Add batch operations (if enabled)
	if cfg.Advanced.BatchEnabled {
		batchStore := storage.NewBatchStorage(store)
		store = batchStore
		log.Printf("Batch operations enabled")
	}

	// 5. Add CAS support (if enabled)
	if cfg.Advanced.CASEnabled || cfg.Advanced.LockingEnabled {
		casStore := storage.NewCASStorage(store)
		store = casStore
		log.Printf("CAS and locking support enabled")
	}

	return store
}

// startBackgroundTasks starts background tasks
func startBackgroundTasks(store storage.Storage, replicator *replication.Replicator, metrics *monitoring.Metrics) {
	// Metrics update every 30 seconds
	metricsTicker := time.NewTicker(30 * time.Second)
	defer metricsTicker.Stop()

	for range metricsTicker.C {
		metrics.UpdateStorageMetrics(store)
		metrics.UpdateReplicationMetrics(replicator)
	}
}

func createMetricsMiddleware(metrics *monitoring.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &monitoring.ResponseWriter{ResponseWriter: w, StatusCode: 200}
			next.ServeHTTP(rw, r)
			duration := time.Since(start)
			metrics.ObserveRequest(r.Method, r.URL.Path, rw.StatusCode, duration)
		})
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}
