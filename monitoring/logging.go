package monitoring

import (
	"distore/auth"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// SetupLogger configures the global logger
func SetupLogger() {
	logrus.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
}

// LoggerMiddleware returns middleware for logging
func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a ResponseWriter to intercept the status
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		// Extracting user data from context
		var userID, tenantID string
		if claims, ok := r.Context().Value("claims").(*auth.Claims); ok && claims != nil {
			userID = claims.UserID
			tenantID = claims.TenantID
		}

		// Log the request
		logrus.WithFields(logrus.Fields{
			"timestamp":   start.Format(time.RFC3339),
			"method":      r.Method,
			"path":        r.URL.Path,
			"status_code": rw.statusCode,
			"duration_ms": duration.Milliseconds(),
			"client_ip":   getClientIP(r),
			"user_id":     userID,
			"tenant_id":   tenantID,
			"user_agent":  r.UserAgent(),
		}).Info("HTTP request")
	})
}

// responseWriter to intercept the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// getClientIP retrieves the client's real IP
func getClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
