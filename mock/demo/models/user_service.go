package models

type UserService struct {
	repo UserRepo
}

func NewUserService(r UserRepo) *UserService {
	return &UserService{
		repo: r,
	}
}

func (s *UserService) GetName(id int) (string, error) {
	u, err := s.repo.GetUser(id)
	if err != nil {
		return "", err
	}
	return u.Name, nil
}
