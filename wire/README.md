# wire（编译期依赖注入）

[google/wire](https://github.com/google/wire) 是 Google 的依赖注入（DI）代码生成工具。
你只写「谁依赖谁」，它生成普通 Go 代码把对象图拼起来——**不用反射，运行时不依赖 wire**，错误在生成阶段就暴露。

## 为什么需要

手工装配一个三层结构，`main` 里会堆出一串构造调用：

```go
d := dal.NewDal()
s := service.NewService(d)
h := controller.NewHandler(s)
h.Process()
```

层数一多、参数一改，这串代码到处跟着改，顺序错了也只在运行时才炸（拿到 nil 接口）。
两类 DI 方案的差别：

| 方案 | 代表 | 时机 | 代价 |
| --- | --- | --- | --- |
| 反射注入 | uber/dig、uber/fx | 运行时读 tag 拼图 | 启动开销、错误后置、可读性差 |
| 代码生成 | google/wire | 生成期写死调用链 | 需要生成步骤、改了签名要重新生成 |

wire 选后者：生成物就是「开发者本来会手写的那几行」，性能与手写一致。

## 安装 wire 工具

工具是独立命令，装到 `GOBIN`（为空则是 `$(go env GOPATH)/bin`）：

```shell
go install github.com/google/wire/cmd/wire@latest
```

```shell
go env GOBIN            # 有值就装这里
go env GOPATH           # GOBIN 为空时装到 <GOPATH>/bin
wire help               # 打印子命令
```

**该目录必须在 `PATH` 里**，否则生成命令会报「无法将 wire 项识别为 cmdlet」。

不想全局安装也可以临时跑（工具会被编译进模块缓存再执行）：

```shell
go run github.com/google/wire/cmd/wire@latest ./wire
```

常用子命令：

| 命令 | 作用 |
| --- | --- |
| `wire ./wire` | 等价 `wire gen ./wire`：生成 `wire_gen.go` |
| `wire check ./wire` | 只分析、不写文件，排查错误用 |
| `wire diff ./wire` | 输出「现有生成物 vs 将要生成的内容」的差异 |
| `wire show ./wire` | 打印 injector 解析出的 provider 图 |
| `wire gen -tags=...` | 追加构建标签 |

## go get 哪个包

代码里只用它的**声明**（`wire.NewSet` / `wire.Build` / `wire.Bind` 等），所以模块里要声明依赖：

```shell
cd wire/demo
go get github.com/google/wire@v0.7.0
```

`wire/demo/go.mod`：

```go
module golearn/wire

go 1.25.0

require github.com/google/wire v0.7.0
```

注意两点：

1. `github.com/google/wire` 是**依赖包**（提供 `wire.Build` 等 API），`github.com/google/wire/cmd/wire` 是**命令行工具**（安装成可执行文件）。两者版本最好一致。
2. 生成的 `wire_gen.go` **不 import wire**，运行时不依赖它；但 `go.mod` 里那条 `require` 不会被 `go mod tidy` 删掉——tidy 会统计带 build tag 的文件（`wireinject`）里的 import。

## demo 目录

```text
wire/demo/
├── go.mod                 # module golearn/wire
├── main.go                # 调 InitHandler / InitAdvancedHandler
├── config/                # 进阶示例的配置类型
│   └── config.go          #   DSN（独立类型）+ Options（供 wire.Struct 拼装）
├── controller/            # Handler ← service.Service
│   └── controller.go      #   ProviderSet = wire.NewSet(NewHandler)
├── service/               # Service ← dal.Dal
│   ├── service.go         #   ProviderSet = wire.NewSet(NewService)
│   └── advanced.go        #   TimedService + NewAdvancedService(dal.Dal, time.Duration)
├── dal/                   # Dal，无依赖
│   ├── dal.go             #   ProviderSet = wire.NewSet(NewDal)
│   └── advanced.go        #   NewAdvancedDal(dsn, opts) → (Dal, func(), error)
└── wire/
    ├── wire.go            # 基础 injector 桩（//go:build wireinject）
    ├── wire_advanced.go   # 进阶 injector：wire.Value / wire.Struct / wire.Bind
    └── wire_gen.go        # 生成物（//go:build !wireinject），含两个 injector
```

依赖方向是一条直线 `Handler → Service → Dal`，所以 injector 写起来最短：

```go
type Handler interface { Process() }
func NewHandler(s service.Service) Handler  // controller
type Service interface { Process() }
func NewService(d dal.Dal) Service          // service
type Dal interface { Process() }
func NewDal() Dal                           // dal
```

## 三个核心概念

| 概念 | 写法 | 作用 |
| --- | --- | --- |
| Provider | 普通构造函数 `func NewX(...) X` | 声明「怎么造 X」；参数就是它的依赖，必须都能被提供 |
| ProviderSet | `wire.NewSet(NewX, NewY)` | 把一组 provider 打包，可嵌套别的 Set |
| Injector | 函数体只调 `wire.Build(...)` | 声明「我要一个 X」，具体拼装由生成器填 |

provider 规则：

- 第一个返回值是**它提供的类型**；可以再返回 `error`，也可以返回 `func()` 作为清理函数（cleanup）。
- 参数不能有两个同类型（wire 按类型匹配，没法区分），需要时用 `wire.Struct` 或包一层参数结构体。
- 同一类型只能被一个 provider 提供（否则报多绑定错误）。

## wire.go 里必须写的注释

`wire/demo/wire/wire.go` 的骨架：

```go
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
```

逐行说明：

- `//go:build wireinject` 必须是**文件第一行**，与 `package` 之间空一行（再跟一行旧语法 `// +build wireinject` 兼容老工具链）。
- 标签是「双向互斥」：普通 `go build` / `go test` 看不到 wire.go（避免桩里的 `return nil` 参与编译），而 wire 工具用 `-tags=wireinject` 加载包，**只看得到**带该标签的文件。源码 `internal/wire/parse.go` 里 `BuildFlags: []string{"-tags=wireinject"}` 就是这句实现。
- 不加标签会怎样：桩当普通代码参与编译，`go build` 照样过（`wire.Build` 返回一个字符串，没人用），但 `wire ./wire` 找不到 injector，静默不生成——最难查的一类「明明写了却没生成」。
- 生成的 `wire_gen.go` 自带 `//go:build !wireinject`，与桩互补，两个文件不会同时定义 `InitHandler`。
- `//go:generate wire` 让 `go generate ./wire` 可用；不带参数时 `wire` 默认处理当前目录，等于在 `wire/demo/wire` 里执行 `wire`。
- injector 函数体**只能**是 `wire.Build(...)` 加一个 return，别塞逻辑，否则报 `injectors must consist of only the wire.Build call and an optional return`。
- `return nil` 只是让桩通过类型检查，值会被生成代码丢弃。
- 文件里**非 injector 的声明会被原样复制进生成物**——上面的 `providerSet` 因此在 `wire_gen.go` 里又出现一次。方便，但别往这文件里堆东西。

## 生成 wire_gen.go 的命令

```shell
cd C:\codes\go\go-adv-notes\wire\demo
wire ./wire          # 等价 wire gen ./wire
```

或走 generate：

```shell
cd C:\codes\go\go-adv-notes\wire\demo
go generate ./wire   # 读 wire.go 里的 //go:generate wire
```

参数含义：`wire` 后面是**包路径**（`go list` 的写法），省略时默认 `.`。injector 在子包就必须写 `./wire`；站在 `wire/demo` 直接敲裸 `wire` 会去 main 包找，什么也不会生成。

校验与运行：

```shell
wire check ./wire    # 只分析，不落盘
wire diff ./wire     # 生成物是否过期
go run .             # 在 wire/demo 下运行
```

生成物 `wire/demo/wire/wire_gen.go`：

```go
// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package wire

import (
	"github.com/google/wire"
	"golearn/wire/controller"
	"golearn/wire/dal"
	"golearn/wire/service"
)

// Injectors from wire.go:

func InitHandler() controller.Handler {
	dalDal := dal.NewDal()
	serviceService := service.NewService(dalDal)
	handler := controller.NewHandler(serviceService)
	return handler
}

// wire.go:

var providerSet = wire.NewSet(controller.ProviderSet, service.ProviderSet, dal.ProviderSet)
```

读法：

1. 开头固定头 + `DO NOT EDIT`，每次生成被覆盖，手改无效。
2. 中间 `Injectors from wire.go:` 段就是「按依赖顺序 new 一遍」，变量名是「类型名 + 构造出的名字」。本 demo 有两个 injector，所以生成物里会有多段 `Injectors from xxx.go:`，每段一个 injector。
3. 末尾 `wire.go:` 段是桩里非 injector 声明的复制品。
4. `import` 里仍留着 `github.com/google/wire`，只是因为 `providerSet` 被复制过来了；桩里没有可复制声明时，生成物完全不引用 wire。

运行结果（`go run ./wire/demo`）：

```text
This is wire learning demo
There is Handler process...
There is Service process...
This is dal process...
```

第二个 injector（`wire/demo/wire/wire_advanced.go`）的输出：

```text
This is wire advanced demo
There is Handler process...
advanced service: timeout=2s
advanced dal: dsn=file:advanced.db region=cn-north-1 timeout=2s debug=false
advanced dal: cleanup
```

注意 `cleanup` 是 `main.go` 里 `defer cleanup()` 触发的，不在注入流程中。

注意 `main.go` 的 import：本地包 `golearn/wire/wire` 包名就叫 `wire`，与 `github.com/google/wire` 同名，同时出现时必须给其中一个起别名（如 ```appwire "golearn/wire/wire"`）。本项目 main 只用到前者，直接写 `wire.InitHandler()` 没问题。

## 进阶用法

这一节的特性在 `wire/demo/wire/wire_advanced.go`（`InitAdvancedHandler`）里都有可运行实现：

- `wire.Value` 直接提供 `config.DSN`、`string`、`time.Duration`、`bool` 四个值；
- `wire.Struct(new(config.Options), "Region", "Timeout", "Debug")` 只取这三个字段拼出结构体（`Options` 里若有其它字段就是零值）；
- `dal.NewAdvancedDal` 返回 `(Dal, func(), error)`，`service.NewAdvancedService` 返回具体类型 `*TimedService`，再用 `wire.Bind` 绑到 `service.Service`；
- injector 签名写成 `(controller.Handler, func(), error)` 承接 cleanup 与 error。

接口绑定（provider 造具体类型，注入点要接口）：

```go
type FileStore struct{}
func NewFileStore() *FileStore { return &FileStore{} }

// 注入点要 Store，provider 提供 *FileStore
var StoreSet = wire.NewSet(NewFileStore, wire.Bind(new(Store), new(*FileStore)))
```

用结构体字段当 provider：

```go
type Options struct {
	Host string
	Port int
}

wire.Struct(new(Options), "*")     // 整个结构体作为一个 provider
wire.Struct(new(Options), "Host")  // 只要部分字段
wire.FieldsOf(new(Options), "Host", "Port")  // 把单个字段的类型暴露成 provider
```

常量与接口值：

```go
wire.Value(Options{Host: "localhost"})         // 提供 Options
wire.InterfaceValue(new(io.Reader), os.Stdin)  // 提供 io.Reader
```

带 error / cleanup 的 injector（provider 返回 `(T, func(), error)` 时，injector 签名要跟着有）：

```go
func InitDB(cfg Config) (*sql.DB, func(), error) {
	wire.Build(NewConfig, NewDB)
	return nil, nil, nil
}
```

其它要点：

- `wire.Build` 的多个参数会合并成一个 provider 集合（`wire.NewSet(a, b)` 同理），所以**按主题切 Set 再聚合**最省事，demo 用的就是这种写法。
- wire **不做单例**：每次调用 injector 都重新 new。需要单例就在外面包一层 `sync.Once`。
- 没有 `wire clean`；"清理"就是删掉 `wire_gen.go` 再生成。
- 生成物随时可重生成，不要为了「改一行」去手改它。

## 常见报错排查

| 报错 | 原因 | 处理 |
| --- | --- | --- |
| `no provider found for X, output of injector` | 依赖没被任何 provider 提供，或 Set 没聚合全 | 补 provider / 把对应 Set 加进 `wire.Build` |
| `unused provider "pkg.NewX"` | Set 里 provider 没被用到 | 从 Set 移除，或确认是否漏连 |
| `unused provider set` | 聚合里塞了空 Set 或没用到的 Set | 同上 |
| `provider has multiple parameters of type T` | 构造函数有两个同类型参数 | 换类型、包参数结构体、或用 `wire.Struct` |
| `wire.Bind of concrete type ... does not include a provider for ...` | 只写 Bind，没提供具体类型 | 把具体类型构造函数加进同一个 Set |
| `injectors must consist of only the wire.Build call and an optional return` | injector 函数体里有其它语句 | 逻辑挪进 provider |
| `provider for X returns cleanup but injection does not return cleanup function` | provider 返回 cleanup，injector 没对应返回位 | injector 返回值加 `func()` |
| `provider for X returns error but injection not allowed to fail` | provider 带 error，injector 没带 `error` 返回 | injector 允许返回 error |
| 命令无输出、没生成文件 | 在错误目录跑裸 `wire`，或 wire.go 少了标签 | 用 `wire ./wire`；补 `wireinject` |
| `unrecognized import path` / 拉依赖失败 | 网络或代理 | 配 `GOPROXY`，或 `GOFLAGS=-mod=mod` |

## demo 索引

| 文件 | 内容 |
| --- | --- |
| `wire/demo/dal/dal.go` | 最底层：`Dal` 接口 + `NewDal`（无依赖） |
| `wire/demo/service/service.go` | `Service` 接口 + `NewService(dal.Dal)` |
| `wire/demo/controller/controller.go` | `Handler` 接口 + `NewHandler(service.Service)` |
| `wire/demo/wire/wire.go` | 基础 injector 桩：`//go:build wireinject` + `wire.Build` |
| `wire/demo/wire/wire_advanced.go` | 进阶 injector：`wire.Value` / `wire.Struct` / `wire.Bind` / cleanup |
| `wire/demo/wire/wire_gen.go` | 生成物：两个 injector 的实现 |
| `wire/demo/config/config.go` | `DSN`（独立类型，避免与裸 string 撞车）+ `Options` |
| `wire/demo/dal/advanced.go` | `NewAdvancedDal(dsn, opts)`：带值参数、cleanup、error |
| `wire/demo/service/advanced.go` | `TimedService` + `NewAdvancedService(dal.Dal, time.Duration)` |
| `wire/demo/main.go` | 依次调用 `InitHandler()` 与 `InitAdvancedHandler()` |

运行命令（仓库根目录）：

```shell
go run ./wire/demo
```

生成/更新生成物：

```shell
cd wire/demo && wire ./wire
```

本主题没有测试文件——wire 的正确性由生成阶段保证，`go build ./wire/demo/...` 能过就说明依赖图完整。

## 参考资料

- 仓库：<https://github.com/google/wire>
- 官方指南：<https://github.com/google/wire/blob/main/docs/guide.md>
- 最佳实践：<https://github.com/google/wire/blob/main/docs/best-practices.md>
- 教程工程：<https://github.com/google/wire/tree/main/_tutorial>
