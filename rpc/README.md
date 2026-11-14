# rpc —— 标准库 net/rpc 入门

用标准库 `net/rpc` 把「本地方法调用」变成「TCP 上的远程调用」，看清 RPC 的四个基本要素：**服务名 + 方法名、参数、返回值、错误**；同时认清 `net/rpc` 的能力边界（gob 编解码、无 context、不跨语言），为之后理解 gRPC 打底。

- 服务端：`rpc.NewServer()` → `Register` → `Listen` → `Accept` + `ServeConn`
- 客户端：`rpc.Dial` → `Call("Type.Method", args, &reply)`
- 编解码：默认 gob，两端共用 `models` 包保证类型一致

## 目录结构

```text
rpc/
├── README.md
└── demo/
    ├── go.mod              # module golearn/rpc（只依赖标准库）
    ├── main.go             # 占位入口
    ├── models/
    │   └── model.go        # 两端共用的参数/返回值类型
    ├── server/
    │   └── main.go         # 注册 Caculator，监听 127.0.0.1:7999
    └── client/
        └── main.go         # Dial 后依次调用 Add / Multiply
```

## 先跑起来（两个终端）

服务端常驻：

```shell
go run ./rpc/demo/server
# 2026/11/14 23:20:00 RPC Server listening on 127.0.0.1:7999
```

另开一个终端跑客户端（进程跑完即退出）：

```shell
go run ./rpc/demo/client
# 2026/11/14 23:20:03 Add result: 50
# 2026/11/14 23:20:03 Multiply result: 600
```

两个目录都是独立的 `main` 包，但属于同一个 module（`golearn/rpc`），因此客户端可以 import `golearn/rpc/models`。服务端先起、客户端后跑；端口写死在 `server/main.go` 的 `127.0.0.1:7999`。

## 一次调用发生了什么

```text
client.Call("Caculator.Add", args, &reply)
  │
  ├─ gob 编码请求头 Request{ServiceMethod:"Caculator.Add", Seq:1}
  ├─ gob 编码请求体 args
  │
  ├─ TCP（长连接，多次 Call 复用同一条连接）
  ▼
server: Accept → go svr.ServeConn(conn)
  ├─ readRequest：按 "." 切成服务名 Caculator + 方法名 Add
  ├─ serviceMap["Caculator"].method["Add"]
  ├─ new(reply) 分配回包对象，反射调用 Caculator.Add(args, reply)
  └─ 回包 Response{ServiceMethod, Seq, Error} + reply 的 gob 编码
  │
  ▼
client：用 Seq 在 pending map 里找回对应的 Call，唤醒 Done channel
```

关键事实（都能在源码里对上）：

| 事实 | 位置 |
| --- | --- |
| 请求头只有 `ServiceMethod` 和 `Seq`，没有参数名、没有 IDL | `net/rpc/server.go:172-176` |
| 同一连接上多个 Call 靠 `Seq` 多路复用，`pending` map 按序号找回 | `net/rpc/client.go:71-101` |
| 服务端每读到一个请求就 `go service.call(...)`，**同一 service 的方法会并发执行** | `net/rpc/server.go:480` |
| `ServerCodec` 可换编解码（`ServeCodec`），但方法签名规则不变 | `net/rpc/server.go:447-467` |

所以 `Caculator{}` 无状态才安全；一旦 service 里有共享字段，必须自己加锁。

## 代码导读

### models/model.go：两端共享的类型

```go
type CaculateArgs struct{ A, B int }
type CaculateReply struct{ Result int }
```

- 字段必须**导出**（大写），gob 只编解码导出字段；`a, b int` 会被静默丢弃，服务端收到的永远是 0。
- 类型单独放一个包，被 server 和 client 同时 import：gob 靠「类型名 + 字段名」匹配，两端结构必须一致（字段多出来的会被忽略，缺字段保持零值）。
- 命名沿用了现有代码的拼写（`Caculate` 是 `Calculate` 的笔误）。服务名由**类型名**决定，所以 `Caculator.Add` 里的 `Caculator` 必须和这个类型名一字不差；要改名得两端一起改。

### server/main.go：四步起服务

```go
svr := rpc.NewServer()          // 1. 不要用 rpc.DefaultServer，各服务各一份更清晰
err := svr.Register(caculator)  // 2. 注册服务，必须传指针（见下一节）
listener, err := net.Listen("tcp", "127.0.0.1:7999")  // 3. 监听
for {
    conn, err := listener.Accept() // 4. BIO：每个连接一个 goroutine
    go svr.ServeConn(conn)
}
```

- `Register` 的错误必须检查：类型没导出、一个合规方法都没有、服务名重复，都会在这里返回。
- `ServeConn` 阻塞到连接断开（客户端 `Close` 或 EOF）；连接内的每个请求再由它内部的 goroutine 并发处理。
- 这里 `Accept` 出错直接 `return err`（未区分临时错误），listener 关闭后进程随之退出——教学够用，生产要区分 `net.Error.Temporary()` 并退避重试。
- `main` 里 `log.Fatal` 会直接 `os.Exit(1)`，defer 不执行。

### client/main.go：三步发起调用

```go
conn, err := rpc.Dial("tcp", "127.0.0.1:7999") // 1. net.Dial + NewClient，无超时
defer conn.Close()
args := models.CaculateArgs{A: 20, B: 30}       // 2. 值传递，服务端拿到副本
var reply models.CaculateReply
err = conn.Call("Caculator.Add", args, &reply)  // 3. 同步阻塞；reply 必须传指针
```

- `Call` 同步等结果；异步用 `conn.Go("Caculator.Add", args, &reply, done)`，返回 `*rpc.Call`，结果从 `done` 取。`done` **必须有缓冲**（`cap(done) > 0`），否则 `Go` 内部直接 `log.Panic("rpc: done channel is unbuffered")`。
- `Call` 没有 `context` 参数，不能取消、不能传 deadline；要超时只能给底层连接设 deadline（`conn.Call` 用的是同一个 conn），或者换 gRPC。
- `reply` 传值（`reply` 而非 `&reply`）不会编译错，`Call` 返回的错误形如 `reading body gob: attempt to decode into a non-pointer`。

## 注册规则：哪些方法能被远程调用

`net/rpc` 用反射扫描类型的方法表，**不合规的方法被静默跳过**，只有当「一个合规方法都没有」时才让 `Register` 返回错误（源码 `net/rpc/server.go:285-345`）：

| 条件 | 不满足时 |
| --- | --- |
| 服务类型名导出（服务名 = 类型名） | `rpc.Register: type X is not exported` |
| 方法名导出 | 跳过（记日志的前提是 `logRegisterError`，默认 false） |
| 方法签名形如 `func (t *T) M(args T1, reply *T2) error` | 跳过 |
| 恰好 2 个入参（不含 receiver）、1 个出参 | 跳过并提示 `needs exactly three` / `needs exactly one` |
| 第二个参数是**指针** | 跳过并提示 `reply type ... is not a pointer` |
| 参数类型、回包类型导出或内建 | 跳过并提示 `not exported` |
| 返回类型必须是 `error` | 跳过并提示 `must be error` |
| 同一类型注册两次 | `rpc: service already defined: X` |

两个高频坑：

1. **`Register(值)` 而不是 `Register(&值)`**：`*T` 的方法集包含 `T` 的所有方法，反过来不行；方法定义在指针接收者上时，传值注册会得到「no exported methods of suitable type」，报错里其实带了提示 `hint: pass a pointer to value of that type`。
2. **方法签名不对却「注册成功」**：`Register` 只在「全都不合规」时报错，部分不合规只是被跳过。客户端报 `rpc: can't find method Caculator.XXX` 时，先回头查方法名首字母、第二个参数是不是指针。

服务名默认取类型名；要自定义用 `svr.RegisterName(caculator, "Svc")`，客户端相应写 `"Svc.Add"`。

## 错误与回包

- **业务错误**：handler 返回非 nil `error`，服务端把 `err.Error()` 塞进响应头的 `Error` 字段，客户端拿到的 `err` 类型是 `rpc.ServerError`（本质是 string）。原始错误类型、包装链、`errors.Is/As` 全部丢失，只能比字符串。
- **出错时不回包**：文档明确「If an error is returned, the reply parameter will not be sent back」，所以不能指望读到一半结果。
- **网络错误**：连接断开后未完成的调用统一变成 `rpc.ErrShutdown`（`net/rpc/client.go:26`），可与 `rpc.ServerError` 做类型断言区分「业务失败」和「链路失败」。
- **handler panic 会带走整个进程**：`service.call` 里没有 `defer recover`（`net/rpc/server.go:374-392`），goroutine panic 直接终止 server。要在方法内部自己 recover，或别把不可信输入直接丢给会 panic 的逻辑。

## 能力边界：net/rpc vs gRPC

`net/rpc` 的包文档第一句就写着 **The net/rpc package is frozen and is not accepting new features.** 它适合内网 Go↔Go 的小工具与教学，不适合跨语言生产服务：

| 维度 | `net/rpc` | gRPC |
| --- | --- | --- |
| 编解码 | gob（Go 专属） | protobuf（跨语言） |
| 接口定义 | 无 IDL，靠字符串 + 反射 | `.proto` + 代码生成 |
| 超时/取消 | 无 `context` | 原生 deadline + cancel |
| 流式 | 无 | 单向/双向四种流 |
| 中间件 | 无拦截器，自己包 `ClientCodec`/`ServerCodec` | interceptor |
| HTTP 支持 | `DialHTTP` 走 CONNECT 握手切协议，不是 REST | grpc-web / 网关 |
| 维护状态 | frozen | 活跃 |

RPC 的通用语义（stub、序列化、多路复用、错误传播）在两者身上是一致的，学完这个 demo 再切 gRPC，主要增量是 IDL 与 `context`。

## 测试与调试

**不起两个进程也能测**：`net.Pipe()` 造一条内存连接，一端 `ServeConn`，另一端 `NewClient`，可以放进 `_test.go`：

```go
serverConn, clientConn := net.Pipe()
svr := rpc.NewServer()
_ = svr.Register(new(Caculator))
go svr.ServeConn(serverConn)
client := rpc.NewClient(clientConn)
defer client.Close()

var reply models.CaculateReply
err := client.Call("Caculator.Add", models.CaculateArgs{A: 1, B: 2}, &reply)
// err == nil, reply.Result == 3
```

调试建议：

- 想看明文协议可换 `net/rpc/jsonrpc`（JSON-RPC 1.0），`jsonrpc.NewServerCodec` 配 `ServeCodec`，客户端用 `jsonrpc.NewClient`；代价是失去 gob 的类型自描述，且同样没有 `context`。
- 抓包看到的是 gob 二进制，不要试图肉眼解析；用日志在 handler 里打参数更直接。
- 端口写死时重复启动会报 `bind: Only one usage of each socket address`（Windows）/ `address already in use`（Linux）。

## 常见坑速查

| 现象 | 原因 |
| --- | --- |
| `connection refused` | 只跑了 client；先起 server |
| `rpc: can't find service Caculator.Add` | 服务名错：`ServiceMethod` 要拆成 `服务名.方法名`，服务名默认是类型名 |
| `rpc: can't find method Caculator.XXX` | 方法名拼错，或方法签名不合规被跳过 |
| `reading body gob: attempt to decode into a non-pointer` | `Call` 的第三个参数传了值而不是指针 |
| `Register` 返回「has no exported methods of suitable type」 | 传值注册了指针接收者方法；改传 `&` |
| 参数到了服务端全是零值 | 字段没导出，gob 跳过 |
| 客户端一直卡住 | `Dial` 与 `Call` 都没有超时；连接断了但没设 deadline |
| server 突然整个退出 | handler panic，`net/rpc` 不 recover |
| 并发调用偶发脏数据 | 同一 service 被多个请求 goroutine 并发调用，共享状态没加锁 |

## 运行命令

| 目的 | 命令 |
| --- | --- |
| 启动服务端（常驻 :7999） | `go run ./rpc/demo/server` |
| 另开终端跑客户端 | `go run ./rpc/demo/client` |
| 进入模块后运行（子包） | `cd rpc/demo && go run ./server` |
| 跑本模块测试（暂无 `_test.go`） | `go test ./rpc/demo/... -v` |

## 参考资料

- 标准库源码：`C:\software\go\src\net\rpc\server.go`、`C:\software\go\src\net\rpc\client.go`
- `go doc net/rpc`：方法注册的五条规则、错误回传语义
- `go doc encoding/gob`：导出字段、类型自描述、`gob.Register` 与接口字段
- `net/rpc/jsonrpc`：换 JSON 编解码的最小改法
