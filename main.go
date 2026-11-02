package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// version is the release; override at build time with
// -ldflags "-X main.version=1.2.3".
var version = "1.0.0"

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

// connect opens the pool, pings and migrates once. It is the one-shot,
// fail-fast path used by the reset-password CLI.
func connect(cfg Config) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := openDB(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

// openDB connects, pings and migrates. Errors never include the URL: pgx
// parse errors can echo it, and it carries the password.
func openDB(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}
	db, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

const (
	backoffMin = time.Second
	backoffMax = 30 * time.Second
)

// connectLoop retries openDB with capped exponential backoff until it
// succeeds or ctx ends, then publishes the pool and starts background work.
func (a *App) connectLoop(ctx context.Context) {
	if a.cfg.DatabaseURL == "" {
		log.Print("error: DATABASE_URL is not set, e.g. postgres://user:pass@host:5432/passman; /readyz stays 503")
		return
	}
	delay := backoffMin
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		db, err := openDB(attemptCtx, a.cfg.DatabaseURL)
		cancel()
		if err == nil {
			a.pool.Store(db)
			break
		}
		if ctx.Err() != nil {
			return
		}
		log.Printf("database not ready (retrying in %s): %v", delay, err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(delay*2, backoffMax)
	}
	log.Print("database connected")
	go a.reapSessions(ctx)
	if done, err := isSetUp(ctx, a.db()); err == nil && !done {
		log.Printf("first run: open passman in a browser to set the admin password and get the master key")
	}
}

func main() {
	// The same binary is installed as /usr/local/bin/reset-password.
	if filepath.Base(os.Args[0]) == "reset-password" {
		os.Exit(runResetPassword(os.Args[1:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		os.Exit(runResetPassword(os.Args[2:]))
	}
	os.Exit(run())
}

func run() int {
	cfg := loadConfig()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := newApp(cfg, nil)
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		log.Print(err)
		return 1
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	log.Printf("passman %s listening on %s (sessions last %s)", version, cfg.ListenAddr, cfg.SessionMaxAge)

	connDone := make(chan struct{})
	go func() { defer close(connDone); app.connectLoop(ctx) }()

	code := 0
	select {
	case err := <-errc:
		log.Print(err)
		code = 1
	case <-ctx.Done():
		log.Print("shutting down")
	}
	stop()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		log.Printf("graceful shutdown timed out, closing: %v", err)
		_ = srv.Close()
	}
	<-connDone
	if db := app.db(); db != nil {
		db.Close()
	}
	return code
}
