package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"

	jsoniter "github.com/json-iterator/go"
)

// User
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}

type Store struct {
	mu     sync.RWMutex
	users  map[int]User
	nextID int
}

func NewStore() *Store {
	s := &Store{
		users:  make(map[int]User, 0),
		nextID: 1,
	}
	s.Create(User{Name: "alice", Email: "alice@example.com"})
	s.Create(User{Name: "bob"})
	return s
}

func (s *Store) List() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	return users
}

func (s *Store) Get(id int) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[id]
	return u, ok
}

func (s *Store) Create(u User) User {
	s.mu.Lock()
	defer s.mu.Unlock()

	u.ID = s.nextID
	s.nextID++
	s.users[u.ID] = u
	return u
}

func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[id]; !ok {
		return false
	}
	delete(s.users, id)
	return true
}

func NewMux(store *Store) *http.ServeMux {
	// 类似Python中的BluePrint
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.List())
	})

	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		var in User
		if err := jsoniter.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		if in.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		writeJSON(w, http.StatusCreated, store.Create(in))
	})

	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}
		u, ok := store.Get(id)
		if !ok {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeJSON(w, http.StatusOK, u)
	})

	mux.HandleFunc("DELETE /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "id must be an integer")
			return
		}
		if !store.Delete(id) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// 单文件上传：multipart/form-data，字段名固定为 file。
	mux.HandleFunc("POST /upload", func(w http.ResponseWriter, r *http.Request) {
		// maxMemory 只决定「多少字节留在内存」，超出部分由标准库落到系统临时文件，
		// 它并不是请求体的上限；真正的上限要用 MaxBytesReader（见多文件例子）。
		const maxMemory = 10 << 20
		if err := r.ParseMultipartForm(maxMemory); err != nil {
			writeUploadError(w, err)
			return
		}

		// FormFile 取第一个名为 file 的字段；同名字段有多个时后面的会被忽略。
		file, fh, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "form field \"file\" is required")
			return
		}
		defer file.Close()

		// 示例统一写临时文件，返回前删掉，避免测试/演示污染工作目录。
		dst, err := os.CreateTemp("", "upload-*")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create temp file failed")
			return
		}
		defer os.Remove(dst.Name())
		defer dst.Close()

		// io.Copy 内部用 32KB 缓冲循环 Read/Write，等价于手写 for 循环。
		written, err := io.Copy(dst, file)
		if err != nil {
			writeUploadError(w, err)
			return
		}

		// 所有读写错误都处理完再写响应：writeJSON 之后再 writeError 只会写不出去。
		writeJSON(w, http.StatusOK, map[string]any{
			"filename": fh.Filename, // 客户端提交的原始文件名
			"size":     fh.Size,     // 表单头里声明的文件大小
			"written":  written,     // 实际拷贝成功的字节数
		})
	})

	// 多文件上传：字段名不限，FileName() 非空的分片都当文件处理。
	mux.HandleFunc("POST /upload-multi", func(w http.ResponseWriter, r *http.Request) {
		// 整个请求体的硬上限：超限后底层 Read 返回 *http.MaxBytesError，映射成 413。
		const maxBody = 32 << 20
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)

		// MultipartReader 是流式接口：不把整个表单读进内存/临时文件，边读分片边处理。
		mr, err := r.MultipartReader()
		if err != nil {
			writeUploadError(w, err)
			return
		}

		dir, err := os.MkdirTemp("", "uploads-*")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "create upload dir failed")
			return
		}
		defer os.RemoveAll(dir)

		type uploaded struct {
			Filename string `json:"filename"`
			Size     int64  `json:"size"`
		}

		files := make([]uploaded, 0, 2)
		for {
			part, err := mr.NextPart()
			if err == io.EOF { // 所有分片都读完了
				break
			}
			if err != nil {
				writeUploadError(w, err)
				return
			}

			// 普通表单字段（如 note=demo）没有文件名，直接跳过。
			if part.FileName() == "" {
				continue
			}

			dst, err := os.CreateTemp(dir, "part-*")
			if err != nil {
				writeError(w, http.StatusInternalServerError, "create temp file failed")
				return
			}
			written, err := io.Copy(dst, part)
			closeErr := dst.Close()
			if err != nil {
				writeUploadError(w, err)
				return
			}
			if closeErr != nil {
				writeError(w, http.StatusInternalServerError, "close file failed")
				return
			}

			files = append(files, uploaded{Filename: part.FileName(), Size: written})
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"count": len(files),
			"files": files,
		})
	})

	return mux
}

// writeUploadError 把「请求体超过 MaxBytesReader 上限」与「multipart 格式/读取错误」
// 区分开：前者是 413，后者是客户端提交的数据有问题，返回 400。
func writeUploadError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "upload exceeds limit: "+tooLarge.Error())
		return
	}
	writeError(w, http.StatusBadRequest, "upload failed: "+err.Error())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = jsoniter.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
