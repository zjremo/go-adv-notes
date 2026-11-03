package controller

import (
	"fmt"
	"golearn/wire/service"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewHandler)

type Handler interface {
	Process()
}

func NewHandler(s service.Service) Handler {
	return &handler{
		Service: s,
	}
}

type handler struct {
	service.Service
}

func (h *handler) Process() {
	fmt.Println("There is Handler process...")
	h.Service.Process()
}
