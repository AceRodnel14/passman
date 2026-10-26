package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// version is the release; override at build time with
// -ldflags "-X main.version=1.2.3".
var version = "0.2.0"

type Config struct {
	DatabaseURL   string
	ListenAddr    string
	SessionMaxAge time.Duration
	SecureCookie  bool
	// BackupAllIPs are the direct peer addresses allowed to call /api/backup-all.
	// Empty disables the endpoint.
	BackupAllIPs []netip.Prefix
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// parseAge accepts Go durations ("336h") plus a day suffix ("14d").
func parseAge(s string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid SESSION_MAX_AGE %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid SESSION_MAX_AGE %q", s)
	}
	return d, nil
}

func loadConfig() Config {
	cfg := Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		ListenAddr:   env("LISTEN_ADDR", ":8080"),
		SecureCookie: env("SECURE_COOKIE", "false") == "true",
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required, e.g. postgres://user:pass@host:5432/passman")
	}
	age, err := parseAge(env("SESSION_MAX_AGE", "14d"))
	if err != nil {
		log.Fatal(err)
	}
	cfg.SessionMaxAge = age
	if cfg.BackupAllIPs, err = parseCIDRs(os.Getenv("BACKUP_ALL_ALLOWED_IPS")); err != nil {
		log.Fatalf("BACKUP_ALL_ALLOWED_IPS: %v", err)
	}
	return cfg
}

func connect(cfg Config) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	if err := db.Ping(ctx); err != nil {
		log.Fatalf("ping database: %v", err)
	}
	if err := migrate(ctx, db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	return db
}

func main() {
	// The same binary is installed as /usr/local/bin/reset-password.
	if filepath.Base(os.Args[0]) == "reset-password" {
		os.Exit(runResetPassword(os.Args[1:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		os.Exit(runResetPassword(os.Args[2:]))
	}

	cfg := loadConfig()
	app := newApp(cfg, connect(cfg))
	go app.reapSessions()

	if done, err := isSetUp(context.Background(), app.db); err == nil && !done {
		log.Printf("first run: open passman in a browser to set the admin password and get the master key")
	}
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("passman listening on %s (sessions last %s)", cfg.ListenAddr, cfg.SessionMaxAge)
	log.Fatal(srv.ListenAndServe())
}
