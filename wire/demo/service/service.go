package service

import (
	"fmt"
	"golearn/wire/dal"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewService)

type Service interface {
	Process()
}

func NewService(d dal.Dal) Service {
	return &service{
		Dal: d,
	}
}

type service struct {
	dal.Dal
}

func (s *service) Process() {
	fmt.Println("There is Service process...")
	s.Dal.Process()
}
