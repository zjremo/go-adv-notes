//go:build wireinject
// +build wireinject

package wire

import (
	"time"

	"golearn/wire/config"
	"golearn/wire/controller"
	"golearn/wire/dal"
	"golearn/wire/service"

	"github.com/google/wire"
)

// advancedSet 演示几种「喂参数」的方式：
//   - wire.Value：直接提供常量（DSN / region / timeout / debug）
//   - wire.Struct：把散落的值拼成 config.Options，列出字段名即只取这些字段
//     （wire.Struct(new(config.Options), "*") 表示全部字段）
//   - wire.Bind：把具体类型 *service.TimedService 绑定成 service.Service
//   - provider 返回 cleanup 和 error，由 injector 签名承接
var advancedSet = wire.NewSet(
	wire.Value(config.DSN("file:advanced.db")),
	wire.Value("cn-north-1"),
	wire.Value(2*time.Second),
	wire.Value(false),
	// 生成config.Options取后面三个字段
	wire.Struct(new(config.Options), "Region", "Timeout", "Debug"),
	dal.NewAdvancedDal,
	service.NewAdvancedService,
	wire.Bind(new(service.Service), new(*service.TimedService)),
	controller.ProviderSet,
)

// InitAdvancedHandler 返回 cleanup，是因为 NewAdvancedDal 返回了 cleanup；
// 返回 error，是因为 provider 可能失败。
func InitAdvancedHandler() (controller.Handler, func(), error) {
	wire.Build(advancedSet)
	return nil, nil, nil
}
