package api

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// LogRequests logs every request once it's answered: method, path, status, size, duration and who sent it.
func LogRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.RequestURI(),
			"status", rec.status,
			"bytes", rec.bytes,
			"ms", time.Since(start).Milliseconds(),
			"from", clientIP(r),
		)
	})
}

// clientIP is who sent the request. Behind the cloudflare tunnel every request arrives from the
// tunnel itself, so cloudflare's header with the visitor's address is used when it's there.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip, _, _ := strings.Cut(fwd, ",")
		return strings.TrimSpace(ip)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// recorder notes the status and size of a response as it's written.
type recorder struct {
	http.ResponseWriter
	status int
	bytes  int
	wrote  bool
}

func (r *recorder) WriteHeader(code int) {
	if !r.wrote {
		r.status, r.wrote = code, true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	r.wrote = true
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the real writer, e.g. to flush.
func (r *recorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
