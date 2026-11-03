//go:build wireinject
// +build wireinject

//go:generate wire

package wire

import (
	"golearn/wire/controller"
	"golearn/wire/dal"
	"golearn/wire/service"

	"github.com/google/wire"
)

var providerSet = wire.NewSet(controller.ProviderSet, service.ProviderSet, dal.ProviderSet)

func InitHandler() controller.Handler {
	wire.Build(providerSet)
	return nil
}
