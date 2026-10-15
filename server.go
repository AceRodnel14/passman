package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed all:web/dist
var distFS embed.FS

type App struct {
	cfg     Config
	db      *pgxpool.Pool
	limiter *limiter
}

type authedHandler func(http.ResponseWriter, *http.Request, *Session)

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })

	mux.HandleFunc("GET /api/state", a.handleState)
	mux.HandleFunc("POST /api/setup", a.handleSetup)
	mux.HandleFunc("POST /api/register", a.handleRegister)
	mux.HandleFunc("POST /api/login", a.handleLogin)
	mux.HandleFunc("POST /api/logout", a.handleLogout)
	mux.HandleFunc("POST /api/account/password", a.auth(a.handleChangePassword))

	mux.HandleFunc("GET /api/vault", a.auth(a.handleVault))
	mux.HandleFunc("GET /api/passwords/{id}", a.auth(a.handleGetPassword))
	mux.HandleFunc("POST /api/passwords", a.auth(a.handleSavePassword))
	mux.HandleFunc("PUT /api/passwords/{id}", a.auth(a.handleSavePassword))
	mux.HandleFunc("DELETE /api/passwords/{id}", a.auth(a.handleDeletePassword))
	mux.HandleFunc("PUT /api/passwords/{id}/favorite", a.auth(a.handleFavorite("passwords")))
	mux.HandleFunc("POST /api/folders", a.auth(a.handleCreateFolder))
	mux.HandleFunc("PUT /api/folders/{id}", a.auth(a.handleRenameFolder))
	mux.HandleFunc("DELETE /api/folders/{id}", a.auth(a.handleDeleteFolder))
	mux.HandleFunc("PUT /api/folders/{id}/favorite", a.auth(a.handleFavorite("folders")))
	mux.HandleFunc("GET /api/generate", a.auth(a.handleGenerate))
	mux.HandleFunc("POST /api/backup", a.auth(a.handleBackup))
	mux.HandleFunc("POST /api/restore", a.auth(a.handleRestore))

	mux.HandleFunc("GET /api/admin/users", a.admin(a.handleListUsers))
	mux.HandleFunc("POST /api/admin/users/{id}/password", a.admin(a.handleAdminResetPassword))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeError(w, http.StatusNotFound, "Not found.") })
	mux.Handle("/", spaHandler())
	return securityHeaders(sameOrigin(mux))
}

func (a *App) auth(h authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := a.currentSession(r)
		if err != nil {
			serverError(w, err)
			return
		}
		if s == nil {
			a.clearCookie(w)
			writeError(w, http.StatusUnauthorized, "Your session has expired. Please log in again.")
			return
		}
		h(w, r, s)
	}
}

func (a *App) admin(h authedHandler) http.HandlerFunc {
	return a.auth(func(w http.ResponseWriter, r *http.Request, s *Session) {
		if !s.IsAdmin {
			writeError(w, http.StatusForbidden, "Admins only.")
			return
		}
		h(w, r, s)
	})
}

// sameOrigin rejects cross-site state-changing requests. Combined with
// SameSite=Lax cookies this is the CSRF protection.
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" {
				if u, err := url.Parse(o); err != nil || u.Host != r.Host {
					writeError(w, http.StatusForbidden, "Cross-origin request rejected.")
					return
				}
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				writeError(w, http.StatusForbidden, "Cross-origin request rejected.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// spaHandler serves the built frontend; unknown paths get index.html so
// client-side routes like /folders/<id> can be bookmarked and refreshed.
func spaHandler() http.Handler {
	dist, err := fs.Sub(distFS, "web/dist")
	if err != nil {
		panic(err)
	}
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		log.Fatal("frontend not built: web/dist/index.html missing")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" && p != "index.html" {
			if data, err := fs.ReadFile(dist, p); err == nil {
				if ct := mime.TypeByExtension(path.Ext(p)); ct != "" {
					w.Header().Set("Content-Type", ct)
				}
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				w.Write(data)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	writeError(w, http.StatusInternalServerError, "Something went wrong on the server.")
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "Expected JSON.")
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request.")
		return false
	}
	return true
}
