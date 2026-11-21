package transport

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func setShutdownTimeouts(t *testing.T, drain, shutdown time.Duration) {
	t.Helper()
	oldDrain, oldShutdown := drainTimeout, shutdownTimeout
	drainTimeout, shutdownTimeout = drain, shutdown
	t.Cleanup(func() { drainTimeout, shutdownTimeout = oldDrain, oldShutdown })
}

// startServe runs Serve with handler on a loopback listener and returns the
// base URL, a cancel func that triggers shutdown, and Serve's result channel.
func startServe(t *testing.T, handler http.Handler) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, NewServer(ln.Addr().String(), handler), ln) }()
	return "http://" + ln.Addr().String(), cancel, done
}

func waitServe(t *testing.T, done <-chan error, within time.Duration) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(within):
		t.Fatalf("Serve did not return within %v", within)
		return nil
	}
}

func TestServe_EndsEventStreams(t *testing.T) {
	setShutdownTimeouts(t, 50*time.Millisecond, 5*time.Second)
	streamEnded := make(chan struct{})
	url, cancel, done := startServe(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: ready\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(streamEnded)
	}))

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if line, err := bufio.NewReader(resp.Body).ReadString('\n'); err != nil || !strings.HasPrefix(line, "event: ready") {
		t.Fatalf("stream not established: %q, %v", line, err)
	}

	cancel()
	if err := waitServe(t, done, 2*time.Second); err != nil {
		t.Fatalf("Serve() = %v, want clean shutdown", err)
	}
	select {
	case <-streamEnded:
	default:
		t.Error("stream handler's context was not cancelled")
	}
}

func TestServe_DrainsInFlightRequests(t *testing.T) {
	setShutdownTimeouts(t, 2*time.Second, 5*time.Second)
	started := make(chan struct{})
	url, cancel, done := startServe(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-time.After(200 * time.Millisecond):
			io.WriteString(w, "done")
		case <-r.Context().Done():
			http.Error(w, "cancelled", http.StatusServiceUnavailable)
		}
	}))

	body := make(chan string, 1)
	go func() {
		resp, err := http.Get(url)
		if err != nil {
			body <- err.Error()
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		body <- string(b)
	}()
	<-started
	cancel()

	if got := <-body; got != "done" {
		t.Errorf("in-flight request got %q, want it to finish", got)
	}
	if err := waitServe(t, done, 2*time.Second); err != nil {
		t.Fatalf("Serve() = %v, want clean shutdown", err)
	}
}

func TestServe_GivesUpOnStuckHandlers(t *testing.T) {
	setShutdownTimeouts(t, 20*time.Millisecond, 100*time.Millisecond)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	started := make(chan struct{})
	url, cancel, done := startServe(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-release // ignores its context
	}))

	go func() {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
		}
	}()
	<-started
	cancel()

	start := time.Now()
	if err := waitServe(t, done, 2*time.Second); err != nil {
		t.Fatalf("Serve() = %v, want nil after a forced shutdown", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("forced shutdown took %v", elapsed)
	}
}

func TestServe_ReturnsListenerErrors(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln.Close()
	err = Serve(context.Background(), NewServer("", http.NotFoundHandler()), ln)
	if err == nil {
		t.Fatal("expected an error from a closed listener")
	}
}
