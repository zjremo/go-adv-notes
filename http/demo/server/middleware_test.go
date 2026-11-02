package server_test

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golearn/http/server"
)

// TestChainOrder：Chain(h, A, B) 的执行顺序是 A 进入 → B 进入 → h → B 退出 → A 退出。
func TestChainOrder(t *testing.T) {
	var order []string
	mark := func(name string) server.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name+"-before")
				next.ServeHTTP(w, r)
				order = append(order, name+"-after")
			})
		}
	}

	h := server.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	}), mark("a"), mark("b"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"a-before", "b-before", "handler", "b-after", "a-after"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

// TestRecoverer：Handler panic 时返回 500，进程不退出。
func TestRecoverer(t *testing.T) {
	logger := log.New(io.Discard, "", 0)
	h := server.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}), server.Recoverer(logger))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

// TestTraceID：中间件把 trace id 写进响应头与 context，下游能读到同一个值。
func TestTraceID(t *testing.T) {
	h := server.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, server.TraceIDFromContext(r.Context()))
	}), server.TraceID())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Trace-ID", "trace-123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Trace-ID"); got != "trace-123" {
		t.Fatalf("response header = %q, want trace-123", got)
	}
	if got := rec.Body.String(); got != "trace-123" {
		t.Fatalf("context value = %q, want trace-123", got)
	}
}

// TestLogging：statusRecorder 捕获真实状态码，日志里能看到状态与耗时。
func TestLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	h := server.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}), server.Logging(logger))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/users", nil))

	out := buf.String()
	if !strings.Contains(out, "GET /users -> 418") {
		t.Fatalf("log = %q, want contains GET /users -> 418", out)
	}
}
