//go:generate mockgen -source=repo.go -destination=../mocks/repo_mock.go -package=mocks
package models

type User struct {
	ID   int
	Name string
}

type UserRepo interface {
	GetUser(id int) (*User, error)
}
