package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(url string) *Client {
	c := New(url, "sk-test", http.DefaultClient)
	c.sleep = func(context.Context, time.Duration) error { return nil }
	return c
}

const okBody = `{"model":"m1","choices":[{"message":{"content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"cost":0.001}}`

func TestChatSuccess(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("bad request %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(okBody))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL + "/")
	c.IsOpenRouter = true
	resp, err := c.Chat(context.Background(), Request{
		Model: "m1", Models: []string{"m1", "m2"},
		Messages:       []Message{{Role: "user", Content: "x"}},
		ResponseFormat: &ResponseFormat{Type: "json_schema"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hi" || resp.Usage.Cost != 0.001 || resp.Model != "m1" {
		t.Errorf("unexpected response %+v", resp)
	}
	if got["usage"] == nil || got["provider"] == nil || got["models"] == nil {
		t.Errorf("OpenRouter fields missing from request: %v", got)
	}
}

func TestChatNonOpenRouterDropsExtras(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(okBody))
	}))
	defer srv.Close()
	c := newTestClient(srv.URL)
	if _, err := c.Chat(context.Background(), Request{Model: "m", Models: []string{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"models", "usage", "provider"} {
		if _, ok := got[k]; ok {
			t.Errorf("field %q sent to non-OpenRouter endpoint", k)
		}
	}
}

func TestChatRetriesThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.WriteHeader(429)
			w.Write([]byte(`{"error":{"message":"slow down"}}`))
		case 2:
			w.WriteHeader(503)
		default:
			w.Write([]byte(okBody))
		}
	}))
	defer srv.Close()
	resp, err := newTestClient(srv.URL).Chat(context.Background(), Request{Model: "m"})
	if err != nil || resp.Content != "hi" || calls.Load() != 3 {
		t.Fatalf("resp=%+v err=%v calls=%d", resp, err, calls.Load())
	}
}

func TestChatDoesNotRetryAuthError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"message":"No auth credentials found","code":401}}`))
	}))
	defer srv.Close()
	_, err := newTestClient(srv.URL).Chat(context.Background(), Request{Model: "m"})
	var api *APIError
	if !errors.As(err, &api) || api.Status != 401 || calls.Load() != 1 {
		t.Fatalf("err=%v calls=%d", err, calls.Load())
	}
	if Hint(err) == "" {
		t.Error("expected a hint for 401")
	}
}

func TestChatErrorInsideOKResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":{"message":"Insufficient credits","code":402}}`))
	}))
	defer srv.Close()
	_, err := newTestClient(srv.URL).Chat(context.Background(), Request{Model: "m"})
	var api *APIError
	if !errors.As(err, &api) || api.Status != 402 {
		t.Fatalf("err=%v", err)
	}
}

func TestChatHonorsCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body) // lets the server notice the client hanging up
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := newTestClient(srv.URL).Chat(ctx, Request{Model: "m"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v, want deadline exceeded", err)
	}
}
