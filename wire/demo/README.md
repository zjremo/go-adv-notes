# wire demo

三层依赖 `Handler → Service → Dal` 的最小 wire 示例，外加一个进阶示例（`wire.Value` / `wire.Struct` / `wire.Bind` + cleanup/error）。完整原理见 [../README.md](../README.md)。

## 三步跑起来

```shell
# 1. 装生成器（一次即可）
go install github.com/google/wire/cmd/wire@latest

# 2. 生成 wire/wire_gen.go
cd wire/demo
wire ./wire

# 3. 运行
go run .
```

预期输出：

```text
This is wire learning demo
There is Handler process...
There is Service process...
This is dal process...
This is wire advanced demo
There is Handler process...
advanced service: timeout=2s
advanced dal: dsn=file:advanced.db region=cn-north-1 timeout=2s debug=false
advanced dal: cleanup
```

## 生成物从哪来

- `wire/wire.go`：基础 injector 桩，顶部两行 `//go:build wireinject` / `// +build wireinject` 让普通编译忽略它；
- `wire/wire_advanced.go`：进阶 injector 桩（`InitAdvancedHandler`），标签相同，两个桩由同一次 `wire ./wire` 一起处理；
- `wire/wire_gen.go`：wire 生成的实现（两个 injector 各一段），顶部是 `//go:build !wireinject`，与桩互补；
- 改依赖关系后重新执行 `wire ./wire`（或 `go generate ./wire`），不要手改生成物。

## 文件职责

| 文件 | 职责 |
| --- | --- |
| `dal/dal.go` | `NewDal()`，无依赖，链尾 |
| `service/service.go` | `NewService(dal.Dal)` |
| `controller/controller.go` | `NewHandler(service.Service)` |
| `wire/wire.go` | `wire.Build(providerSet)` 声明要一个 `controller.Handler` |
| `wire/wire_advanced.go` | 进阶：`wire.Value` 提供值、`wire.Struct` 只取 `Options` 的三个字段、`wire.Bind` 绑接口 |
| `config/config.go` | `DSN` 独立类型 + `Options{Region, Timeout, Debug}` |
| `dal/advanced.go` | `NewAdvancedDal(dsn, opts)` → `(Dal, func(), error)` |
| `service/advanced.go` | `TimedService` + `NewAdvancedService(dal.Dal, time.Duration)` |
| `wire/wire_gen.go` | 生成的两个 injector 实现 |
| `main.go` | 依次调用 `InitHandler()` 与 `InitAdvancedHandler()`（后者 defer cleanup） |
