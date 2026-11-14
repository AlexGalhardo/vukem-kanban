// Command kanban serves the Kanban web UI backed by a SQLite file. It is a
// vukem application: `kanban` serves, and any vukem command (migrate,
// route:list, about...) works as `kanban <command>`.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	vukem "github.com/AlexGalhardo/vukem"

	"github.com/AlexGalhardo/trello-golang-htmx/bootstrap"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run keeps the flags the desktop shell, the e2e server and the smoke test
// use (--addr, --db, --log-level, --version) and maps them onto the
// configuration vukem reads (HOST, PORT, DATABASE_URL, LOG_LEVEL), which
// stays available as environment variables.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return vukem.RunMain(ctx, append([]string{"kanban"}, args...), lookupEnv(nil), bootstrap.New,
			vukem.WithVersion(version), vukem.WithStdout(stdout), vukem.WithStderr(stderr), envDefaults())
	}

	fs := flag.NewFlagSet("kanban", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "", "listen address, default 127.0.0.1:8080; port 0 picks a free port")
	dbPath := fs.String("db", "", "SQLite database file, default kanban.db")
	logLevel := fs.String("log-level", "", "debug, info, warn or error")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		_, err := fmt.Fprintln(stdout, version)
		return err
	}

	flagEnv := map[string]string{}
	if *addr != "" {
		host, port, err := net.SplitHostPort(*addr)
		if err != nil {
			return fmt.Errorf("invalid --addr: %w", err)
		}
		flagEnv["HOST"], flagEnv["PORT"] = host, port
	}
	if *logLevel != "" {
		// vukem falls back to info for an unknown level; a typo should fail.
		var level slog.Level
		if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
			return fmt.Errorf("invalid --log-level: %w", err)
		}
		flagEnv["LOG_LEVEL"] = *logLevel
	}
	if *dbPath != "" {
		if dir := filepath.Dir(*dbPath); dir != "." {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("create data dir: %w", err)
			}
		}
		flagEnv["DATABASE_URL"] = "file:" + filepath.ToSlash(*dbPath)
	}

	// The desktop shell waits for this exact line to learn the chosen port.
	ready := make(chan string, 1)
	go func() {
		select {
		case a := <-ready:
			_, _ = fmt.Fprintf(stdout, "LISTENING %s\n", a)
		case <-ctx.Done():
		}
	}()
	return vukem.RunMain(ctx, []string{"kanban"}, lookupEnv(flagEnv), bootstrap.New,
		vukem.WithVersion(version), vukem.WithStdout(stdout), vukem.WithStderr(stderr), vukem.WithReady(ready),
		envDefaults())
}

func envDefaults() vukem.Option {
	return vukem.WithEnvDefaults(map[string]string{
		"HOST": "127.0.0.1", "PORT": "8080", "DATABASE_URL": "file:kanban.db",
	})
}

// lookupEnv resolves a key from the flags first, then the process
// environment. The app runs as production (JSON logs, no debug pages) even
// when nobody configured it, so APP_ENV defaults to that, and APP_KEY to a
// random per-process key: the kanban keeps no sessions or encrypted data,
// but vukem refuses to boot a production app without one.
func lookupEnv(flags map[string]string) func(string) (string, bool) {
	var key string
	return func(name string) (string, bool) {
		if v, ok := flags[name]; ok {
			return v, true
		}
		if v, ok := os.LookupEnv(name); ok {
			return v, true
		}
		switch name {
		case "APP_ENV":
			return "production", true
		case "APP_KEY":
			if key == "" {
				b := make([]byte, 32)
				_, _ = rand.Read(b)
				key = "base64:" + base64.StdEncoding.EncodeToString(b)
			}
			return key, true
		}
		return "", false
	}
}
