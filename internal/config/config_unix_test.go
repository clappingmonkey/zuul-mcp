//go:build unix

package config

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLoad_SecretFileFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	isolateEnv(t, map[string]string{"ZUUL_AUTH_TOKEN_FILE": path})

	done := make(chan error, 1)
	go func() {
		_, err := Load()
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("Load() error = %v, want a regular file error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Load() blocked on a FIFO")
	}
}

// If the path is swapped for a FIFO after the type check, opening it must
// not block (no writer is ever connected here).
func TestSecretOpenFlags_DoNotBlockOnFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		f, err := os.OpenFile(path, secretOpenFlags, 0)
		if err == nil {
			f.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("OpenFile() = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("opening a FIFO with secretOpenFlags blocked")
	}
}
