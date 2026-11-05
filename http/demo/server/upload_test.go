package server_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"golearn/http/server"
)

// newUploadRequest 用 multipart.Writer 构造请求体：fields 是普通字段，
// files 是文件字段（字段名、文件名、内容）。
func newUploadRequest(t *testing.T, target string, fields [][2]string, files [][3]string) *http.Request {
	t.Helper()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, f := range fields {
		if err := mw.WriteField(f[0], f[1]); err != nil {
			t.Fatalf("write field %q: %v", f[0], err)
		}
	}
	for _, f := range files {
		fw, err := mw.CreateFormFile(f[0], f[1])
		if err != nil {
			t.Fatalf("create form file %q: %v", f[0], err)
		}
		if _, err := fw.Write([]byte(f[2])); err != nil {
			t.Fatalf("write file %q: %v", f[1], err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestUploadSingle(t *testing.T) {
	mux := server.NewMux(server.NewStore())

	req := newUploadRequest(t, "/upload",
		[][2]string{{"note", "ignored"}},
		[][3]string{{"file", "hello.txt", "hello upload"}},
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got["filename"] != "hello.txt" {
		t.Fatalf("filename = %v", got["filename"])
	}
	// JSON 数字解到 any 统一是 float64，断言时要转换。
	if int(got["size"].(float64)) != len("hello upload") {
		t.Fatalf("size = %v", got["size"])
	}
	if int(got["written"].(float64)) != len("hello upload") {
		t.Fatalf("written = %v", got["written"])
	}
}

func TestUploadMissingFile(t *testing.T) {
	mux := server.NewMux(server.NewStore())

	req := newUploadRequest(t, "/upload", [][2]string{{"note", "no file here"}}, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestUploadMultiple(t *testing.T) {
	mux := server.NewMux(server.NewStore())

	req := newUploadRequest(t, "/upload-multi",
		[][2]string{{"note", "批量上传"}},
		[][3]string{
			{"files", "a.txt", "aaa"},
			{"files", "b.txt", "bbbb"},
		},
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got struct {
		Count int `json:"count"`
		Files []struct {
			Filename string `json:"filename"`
			Size     int64  `json:"size"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Count != 2 || len(got.Files) != 2 {
		t.Fatalf("count = %d, files = %+v", got.Count, got.Files)
	}
	// 分片按请求体顺序返回，所以结果也是 a、b 的稳定顺序。
	if got.Files[0].Filename != "a.txt" || got.Files[0].Size != 3 {
		t.Fatalf("files[0] = %+v", got.Files[0])
	}
	if got.Files[1].Filename != "b.txt" || got.Files[1].Size != 4 {
		t.Fatalf("files[1] = %+v", got.Files[1])
	}
}

func TestUploadMultiRejectsNonMultipart(t *testing.T) {
	mux := server.NewMux(server.NewStore())

	req := httptest.NewRequest(http.MethodPost, "/upload-multi", bytes.NewBufferString(`{"file":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
