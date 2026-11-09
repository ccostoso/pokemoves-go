package pokeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestClientSuccess forces the basic request, response decoding and the data unmarshal.
func TestClientSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"data":{}}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)

	var out struct{}
	err := c.Do(context.Background(), "query", nil, &out)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestClientGraphQLErrors forces the errors check, the case that matters most.
func TestClientGraphQLErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // 2xx is required to reach the errors check at all
		fmt.Fprint(w, `{"data":null,"errors":[{"message":"pokemon not found"}]}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)

	var out struct{}
	err := c.Do(context.Background(), "query", nil, &out)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

// TestClientNon2xx forces the status check.
func TestClientNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // non-2xx status
		fmt.Fprint(w, `{"data":null}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)

	var out struct{}
	err := c.Do(context.Background(), "query", nil, &out)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

// TestClientSendsQueryAndVars forces the request body to actually carry the query and variables.
func TestClientSendsQueryAndVars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
			return
		}
		var reqBody struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if reqBody.Query != "query" {
			t.Fatalf("expected query 'query', got %s", reqBody.Query)
		}
		if reqBody.Variables["key"] != "value" {
			t.Fatalf("expected variable key=value, got %v", reqBody.Variables)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("expected Content-Type 'application/json', got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"data":{}}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)

	var out struct{}
	err := c.Do(context.Background(), "query", map[string]any{"key": "value"}, &out)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestClientBadJSON ensures that the client handles invalid JSON responses gracefully.
func TestClientBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"data":`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)

	var out struct{}
	err := c.Do(context.Background(), "query", nil, &out)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

// TestClientBodyTooLarge ensures that the client handles responses with excessively large bodies correctly.
func TestClientBodyTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":{"x":"%s"}}`, strings.Repeat("a", 2048))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	c.maxBytes = 1024

	var out struct{}
	err := c.Do(context.Background(), "query", nil, &out)
	if !errors.Is(err, ErrUpstream) || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected a 'too large' ErrUpstream, got %v", err)
	}
}

// TestClientNullData ensures that the client handles responses with null data correctly.
func TestClientNullData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"data":null}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)

	var out struct{}
	err := c.Do(context.Background(), "query", nil, &out)

	// Expect ErrUpstream since null data should be treated as a missing result.
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
}

// The client's own Timeout fires even though the caller set no deadline.
func TestClientTimeout(t *testing.T) {
	srv := newHangingServer(t)

	c := NewClient(srv.URL)
	c.http.Timeout = 50 * time.Millisecond

	var out struct{}
	start := time.Now()
	err := c.Do(context.Background(), "query", nil, &out)

	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("expected to give up in ~50ms, took %v", elapsed)
	}
}

// The caller's context deadline fires first, because it is much shorter than the client's 10s.
func TestClientContextDeadline(t *testing.T) {
	srv := newHangingServer(t)

	c := NewClient(srv.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var out struct{}
	start := time.Now()
	err := c.Do(ctx, "query", nil, &out)

	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("expected ErrUpstream, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("expected to give up in ~50ms, took %v", elapsed)
	}
}

// newHangingServer returns a server whose handler never responds until cleanup.
func newHangingServer(t *testing.T) *httptest.Server {
	t.Helper() // mark this function as a test helper so failures are reported at the caller's line
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // block until the test is over
	}))
	t.Cleanup(func() {
		close(release) // let any stuck handler return first...
		srv.Close()    // ...because Close waits for handlers to finish
	})
	return srv
}
