package server_test

import (
	"bytes"
	"encoding/json"
	"golearn/http/server"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestHandlerBasics(t *testing.T) {
	mux := server.NewMux(server.NewStore())

	// httptest 不走真实端口，内存内模拟，速度响应极快
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	// 直接调用接口方法
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	// Header 值是普通字符串，空格/大小写都参与比较，必须与 writeJSON 里完全一致。
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestCreateAndGet(t *testing.T) {
	srv := httptest.NewServer(server.NewMux(server.NewStore()))
	defer srv.Close()

	// srv.URL base_url
	resp, err := http.Post(srv.URL+"/users", "application/json", bytes.NewBufferString(`{"name":"carol","email":"carol@example.com"}`))
	if err != nil {
		t.Fatal("post failed, err:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var created server.User
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created failed, err: %v", err)
	}
	if created.ID == 0 || created.Name != "carol" {
		t.Fatalf("created = %+v", created)
	}

	resp2, err := http.Get(srv.URL + "/users/" + strconv.Itoa(created.ID))
	if err != nil {
		t.Fatalf("get failed, err: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}
	var got server.User
	if err := json.NewDecoder(resp2.Body).Decode(&got); err != nil {
		t.Fatalf("decode user failed, err: %v", err)
	}
	if got != created {
		t.Fatalf("got = %+v, want %+v", got, created)
	}
}

func TestBadRequest(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"invalid json", `not-json`},
		{"missing name", `{"email":"x@example.com"}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mux := server.NewMux(server.NewStore())
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(c.body))
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}

			// String returns the contents of the unread portion of the buffer
			// as a string. If the [Buffer] is a nil pointer, it returns "<nil>".
			//
			// To build strings more efficiently, see the [strings.Builder] type.
			if !strings.Contains(rec.Body.String(), `"error"`) {
				t.Fatalf("body = %q", rec.Body.String())
			}
		})
	}
}

func TestNotFoundAndDelete(t *testing.T) {
	mux := server.NewMux(server.NewStore())

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/9999", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get 9999 status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/users/1", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("204 should have empty body, got %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/1", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get deleted status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
