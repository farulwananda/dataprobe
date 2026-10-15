package graph

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/farulwananda/dataprobe/internal/model"
)

// Server runs the local loopback relationship graph viewer.
type Server struct {
	snapshot   *model.Snapshot
	token      string
	listener   net.Listener
	httpServer *http.Server
	addr       string
}

// Config configures the graph web server.
type Config struct {
	Snapshot *model.Snapshot
	Port     int
}

// NewServer initializes the local graph viewer server.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Snapshot == nil {
		return nil, errors.New("snapshot is required")
	}

	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed generating session token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	s := &Server{
		snapshot: cfg.Snapshot,
		token:    token,
	}

	webSubFS, err := fs.Sub(WebAssets, "web")
	if err != nil {
		return nil, fmt.Errorf("failed creating sub filesystem for web assets: %w", err)
	}
	fileServer := http.FileServer(http.FS(webSubFS))

	mux := http.NewServeMux()

	// 1. Static embedded assets
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// 2. Main HTML view
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; frame-ancestors 'none';")
		htmlData, err := fs.ReadFile(webSubFS, "index.html")
		if err != nil {
			http.Error(w, "index.html missing from embedded assets", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(htmlData)
	})

	// 3. Sanitized snapshot data API (strictly loopback + same-origin + token-validated)
	mux.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		// SEC-05: Verify local loopback peer and Host header
		peer, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || !isLoopback(peer) {
			http.Error(w, "forbidden: local viewer only", http.StatusForbidden)
			return
		}

		reqHost, _, _ := net.SplitHostPort(r.Host)
		if reqHost == "" {
			reqHost = r.Host
		}
		if !isLoopback(reqHost) {
			http.Error(w, "forbidden: invalid Host header", http.StatusForbidden)
			return
		}

		// SEC-05: Verify Origin if request carries one
		if origin := r.Header.Get("Origin"); origin != "" {
			if !strings.HasPrefix(origin, "http://127.0.0.1:") && !strings.HasPrefix(origin, "http://localhost:") {
				http.Error(w, "forbidden: cross-origin access disallowed", http.StatusForbidden)
				return
			}
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "forbidden: cross-site access disallowed", http.StatusForbidden)
			return
		}
		reqToken := r.URL.Query().Get("token")
		if reqToken == "" {
			reqToken = r.Header.Get("X-Dataprobe-Token")
		}
		if reqToken != s.token {
			http.Error(w, "invalid or missing session token", http.StatusUnauthorized)
			return
		}

		// Security: No wildcard CORS
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")

		_ = json.NewEncoder(w).Encode(s.snapshot)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed binding local listener to %s: %w", addr, err)
	}

	s.listener = ln
	s.addr = ln.Addr().String()

	s.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return s, nil
}

// Addr returns the bound TCP address.
func (s *Server) Addr() string {
	return s.addr
}

// URL returns the full authenticated local URL to open in a browser.
func (s *Server) URL() string {
	return fmt.Sprintf("http://%s/?token=%s", s.addr, s.token)
}

// Serve starts listening for HTTP requests (blocking until Close).
func (s *Server) Serve() error {
	return s.httpServer.Serve(s.listener)
}

// Close gracefully stops the web server.
func (s *Server) Close() error {
	return s.httpServer.Close()
}

// OpenBrowser launches the user's default browser to the viewer URL.
func OpenBrowser(urlStr string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", urlStr)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", urlStr)
	default:
		cmd = exec.Command("xdg-open", urlStr)
	}
	_ = cmd.Start()
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
