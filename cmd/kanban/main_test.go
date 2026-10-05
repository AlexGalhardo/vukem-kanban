package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVersionFlag(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"--version"}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != version {
		t.Fatalf("version = %q", out.String())
	}
}

func TestInvalidFlags(t *testing.T) {
	if err := run(context.Background(), []string{"--log-level", "loud"}, io.Discard, io.Discard); err == nil {
		t.Fatal("expected error for invalid log level")
	}
	if err := run(context.Background(), []string{"--nope"}, io.Discard, io.Discard); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

// TestServeAndShutdown covers the desktop contract: port 0, the LISTENING
// line on stdout, a working server and a clean stop on context cancel.
func TestServeAndShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pr, pw := io.Pipe()
	dbPath := filepath.Join(t.TempDir(), "nested", "kanban.db")
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"--addr", "127.0.0.1:0", "--db", dbPath, "--log-level", "error"}, pw, io.Discard)
		_ = pw.Close()
	}()

	line, err := bufio.NewReader(pr).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	addr, ok := strings.CutPrefix(strings.TrimSpace(line), "LISTENING ")
	if !ok {
		t.Fatalf("first line = %q", line)
	}
	go func() { _, _ = io.Copy(io.Discard, pr) }()

	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("healthz = %d", res.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server did not stop")
	}
}
