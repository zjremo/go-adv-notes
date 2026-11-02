package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPathValue：{id} 占位符由 r.PathValue 取出，取到的是已解码的值。
func TestPathValue(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "id=%s", r.PathValue("id"))
	})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/42", nil))

	if got := rec.Body.String(); got != "id=42" {
		t.Fatalf("body = %q, want %q", got, "id=42")
	}
}

// TestMethodNotAllowed：路径存在但方法不匹配时，ServeMux 自动返回 405 并带 Allow 头。
func TestMethodNotAllowed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/users", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, http.MethodGet) {
		t.Fatalf("allow = %q, want contains GET", allow)
	}
}

// TestNotFoundRoute：没有任何 pattern 匹配时返回 404。
func TestNotFoundRoute(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestPrecedence：更具体的 pattern 优先，/users/me 不会被 /users/{id} 抢走。
func TestPrecedence(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "wildcard")
	})
	mux.HandleFunc("GET /users/me", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "me")
	})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/me", nil))

	if got := rec.Body.String(); got != "me" {
		t.Fatalf("body = %q, want %q", got, "me")
	}
}

// TestWildcardTail：{path...} 吃掉剩余全部路径段，只能出现在 pattern 末尾。
func TestWildcardTail(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /files/{path...}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.PathValue("path"))
	})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/a/b/c.txt", nil))

	if got := rec.Body.String(); got != "a/b/c.txt" {
		t.Fatalf("body = %q, want %q", got, "a/b/c.txt")
	}
}
