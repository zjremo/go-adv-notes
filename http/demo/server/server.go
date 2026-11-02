package server

import (
	"net/http"
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

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = jsoniter.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
