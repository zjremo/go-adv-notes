# http

`net/http` 学习模块，分客户端与服务端两部分，两侧都有可运行代码。

## 目录结构

```text
http/
├── README.md
└── demo/
    ├── go.mod              # module golearn/http
    ├── go.sum
    ├── .env                # DEEPSEEK_BASE_URL / DEEPSEEK_API_KEY（不入库，模块根）
    ├── main.go             # demo 入口
    ├── client/
    │   └── api_test.go     # 客户端：TestGet、TestPost
    ├── server/
    │   ├── server.go       # Store 内存仓库 + NewMux 路由与 JSON 响应
    │   ├── middleware.go   # Middleware/Chain、Logging、Recoverer、TraceID
    │   ├── server_test.go  # 直连 ServeHTTP 与 httptest.NewServer
    │   ├── mux_test.go     # pattern 语义：PathValue、405、优先级、通配
    │   ├── middleware_test.go
    │   └── run/
    │       └── main.go     # 真实监听 + 优雅关闭
    └── graceful/
        ├── main.go         # 端到端：起服务 → 客户端调用 → 优雅暂停
        └── main_test.go    # 断言 Shutdown 等在途慢请求跑完
```

依赖：`github.com/joho/godotenv`（读 `.env`）、`github.com/json-iterator/go`（JSON 编解码）。

## 客户端

### 请求流程

```go
req, err := http.NewRequest(http.MethodPost, baseURL, strings.NewReader(jsonBody))
if err != nil {
	t.Fatal("NewRequest failed, err:", err)
}

req.Header.Set("Content-Type", "application/json")
req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

client := &http.Client{}
resp, err := client.Do(req)
if err != nil {
	t.Fatal("call req failed, err:", err)
}
defer resp.Body.Close()

var respBody map[string]any
if err := jsoniter.NewDecoder(resp.Body).Decode(&respBody); err != nil {
	t.Fatal("decode respBody failed, err:", err)
}
```

| 步骤 | API |
| --- | --- |
| 构造请求 | `http.NewRequest(method, url, body)`，body 为 `io.Reader` |
| 设置请求头 | `req.Header.Set(k, v)` |
| 发送 | `(*http.Client).Do(req)` |
| 释放 | `defer resp.Body.Close()` |
| 解码 | `jsoniter.NewDecoder(resp.Body).Decode(&v)` |

### demo 索引

| 位置 | 内容 |
| --- | --- |
| `client/api_test.go` `TestGet` | GET 公开示例地址，解码为 `[]map[string]any` 后逐条打印 |
| `client/api_test.go` `TestPost` | `dotenv.Load("../.env")` 取 base URL 与 api key，POST JSON 并带 Bearer 头，解码为 `map[string]any` 后遍历 `results` 取 `title` / `url` / `summary` |

### 客户端结论

1. `resp.Body` 必须关闭：`defer resp.Body.Close()`，否则连接与文件描述符泄漏。
2. 请求头要显式设置：`NewRequest` 不会自动补 `Content-Type`，鉴权头也需自行加上 `Authorization: Bearer <token>`。
3. `http.Client` 应当复用而不是每次新建，连接池才有意义；零值 `&http.Client{}` 没有超时，需要时设 `Client.Timeout`。
4. 解码成 `map[string]any` 后每一层都要类型断言（`respBody["results"].([]any)`、`result.(map[string]any)`），断言失败要单独报错。
5. `jsoniter` 与 `encoding/json` 的 `Marshal` / `Unmarshal` / `Encoder` / `Decoder` 用法一致，`MarshalToString` 是它额外提供的方法。
6. POST 的 body 被读过一次就没了，重试时要重新构造 `strings.NewReader(jsonBody)`。
7. `dotenv.Load()` 的路径相对**进程工作目录**。测试二进制的 cwd 是包目录 `http/demo/client`，而 `.env` 在模块根 `http/demo`，所以 `TestPost` 里写 `dotenv.Load("../.env")`；改在 `main` 中加载时 cwd 是 `http/demo`，直接 `Load()` 即可。

## 服务端

### 接口

| 方法与路径 | 作用 | 状态码 |
| --- | --- | --- |
| `GET /healthz` | 健康检查 | 200 |
| `GET /users` | 列出全部用户 | 200 |
| `POST /users` | 创建用户，body 为 `{"name":..,"email":..}` | 201；body 非法或 `name` 为空时 400 |
| `GET /users/{id}` | 按 id 查询 | 200；id 非整数 400；不存在 404 |
| `DELETE /users/{id}` | 按 id 删除 | 204；不存在 404 |

### handler 写法

```go
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
```

`http.Handler` 只有一个方法 `ServeHTTP(w http.ResponseWriter, r *http.Request)`；`http.HandlerFunc` 把普通函数转成 `Handler`，`HandleFunc` 再帮你注册到 `ServeMux`。

### 一个请求在服务端的一生

```text
net.Listen(:8080) → accept 连接
  → 连接上读请求 → http.Server 组装 *http.Request
  → Chain：Logging → Recoverer → TraceID → mux
  → ServeMux 按 pattern 匹配 → 命中 handler
  → writeJSON：Set 头 → WriteHeader → encode body
  → 响应写回 → 连接回 idle，等下一个请求（keep-alive）或关闭
```

### 中间件

| 中间件 | 作用 | 关键点 |
| --- | --- | --- |
| `Logging(logger)` | 记录方法、路径、状态码、耗时 | 必须包一层 `statusRecorder` 才能读到状态码 |
| `Recoverer(logger)` | `defer recover()`，panic → 500 | 响应已开始写出时只能记日志 |
| `TraceID()` | 读 `X-Trace-ID`，缺省生成时间串；写响应头 + 塞进 context | `r.WithContext` 返回新请求，要替换 `r` |
| `Chain(h, A, B)` | 逆序包装成 `A(B(h))` | 请求顺序 A → B → h，返回顺序 h → B → A |

### 启动与关闭（`server/run/main.go`）

```go
srv := &http.Server{
	Addr:              ":8080",
	Handler:           handler,
	ReadHeaderTimeout: 5 * time.Second,
}

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()

go func() {
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatalf("serve failed: %v", err)
	}
}()

<-ctx.Done()
stop() // 恢复 SIGINT 默认处置：关闭卡住时第二次 Ctrl+C 能强杀

shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
_ = srv.Shutdown(shutdownCtx)
```

| `http.Server` 字段 | 限制什么 | 建议 |
| --- | --- | --- |
| `ReadHeaderTimeout` | 读完请求头的总时长 | 必设，防 Slowloris 慢速攻击 |
| `ReadTimeout` | 读完整请求（头 + body）的时长 | 有 `ReadHeaderTimeout` 后可只对 body 设 |
| `WriteTimeout` | 从读完请求头到写完响应的时长 | 按接口最慢耗时设 |
| `IdleTimeout` | keep-alive 空闲连接的存活时长 | 缺省复用 `ReadTimeout` |
| `MaxHeaderBytes` | 请求头字节上限 | 缺省 1MB |

优雅关闭语义（源码 `C:\software\go\src\net\http\server.go:3150-3186`）：

1. 进来立刻关 listener（3154），`ListenAndServe` 返回 `http.ErrServerClosed`，新连接被拒。
2. 之后是轮询循环：每轮 `closeIdleConns()` 检查是否全部连接 idle（3176），间隔 1ms 起翻倍、上限 500ms（3128）。
3. 传入的 `ctx` **只约束 Shutdown 自己等多久**：`<-ctx.Done()` 就返回 `ctx.Err()`（3180-3181），**不会取消任何 handler 的 `r.Context()`**。
4. 超时返回时在途连接仍在跑；需要强断就再 `srv.Close()`（你的 `main` 超时后直接退出，OS 会兜底断开）。
5. hijacked 连接（WebSocket 等）Shutdown 不管，要用 `RegisterOnShutdown` 自己通知。

### 端到端示例：起服务 → 客户端调用 → 优雅暂停（`demo/graceful`）

`main.go` 把前面所有零件串成一条真实链路，跑一次就能看到「关闭时在途请求不被掐断」：

```text
net.Listen("127.0.0.1:0")     随机端口，避免和 :8080 冲突
  → srv.Serve(ln) goroutine   后台监听
  → 客户端 GET /healthz、POST /users、GET /users/{id}
  → 慢请求 GET /slow?d=3s     另一个 goroutine，模拟在途业务
  → <-ctx.Done()              1s 后自动触发（真实服务是 Ctrl+C）
  → srv.Shutdown(5s ctx)      停收新连接，等慢请求跑完（约 2s）
  → 新连接被拒                用 DisableKeepAlives 的 client 验证
```

关键点：

- 服务端复用 `server.NewMux`，只在上面多挂一条 `GET /slow`，业务代码与关停代码分离。
- `Shutdown` 返回耗时 ≈ 2s，说明它**确实在等**那个 3s 的请求；`TestRunGracefulShutdown` 用「总耗时 > 2s」把这个行为固定下来。
- 关闭后再请求必须失败：`Shutdown` 关闭 listener 后，新连接连不上；`DisableKeepAlives` 避免复用旧连接导致误判。
- 慢接口不读 `r.Context()`，所以不会被取消——这正是「优雅」：业务跑完再退，而不是中途掐断。
- `encoding/json` 把 JSON 数字解到 `any` 时**统一是 `float64`**：`created["id"].(int)` 必然失败并得到 0（表现为请求打到 `/users/0` 拿 404），要断言 `.(float64)` 并检查 `ok`。

### 测试清单

| 测试 | 覆盖点 |
| --- | --- |
| `TestHandlerBasics` | `NewRecorder` 直连 `ServeHTTP`：200、`Content-Type`、body 内容 |
| `TestCreateAndGet` | `httptest.NewServer` 真实 TCP 往返：POST 201 → GET 回读一致 |
| `TestBadRequest` | JSON 非法 / `name` 为空 → 400，body 含 `"error"` |
| `TestNotFoundAndDelete` | 不存在 → 404；DELETE → 204 且 body 为空；删除后 GET → 404 |
| `TestPathValue` | `PathValue("id")` 直接取到路径参数 |
| `TestMethodNotAllowed` | 方法不匹配自动 405，`Allow` 头含 GET |
| `TestNotFoundRoute` | 无匹配 pattern → 404 |
| `TestPrecedence` | `/users/me` 优先于 `/users/{id}` |
| `TestWildcardTail` | `/files/{path...}` 跨段通配 |
| `TestChainOrder` | 中间件进出顺序：A → B → handler → B → A |
| `TestRecoverer` | handler panic → 500 |
| `TestTraceID` | `X-Trace-ID` 同时出现在响应头与 handler 的 context |
| `TestLogging` | 日志里有方法、路径、状态码 |
| `TestRunGracefulShutdown` | 端到端：慢请求在途时关闭，断言 Shutdown 等它跑完才返回 |

### 服务端结论

**路由与匹配**

1. pattern（Go 1.22+）三段式：`[METHOD] [HOST]/[PATH]`。`{id}` 匹配单个非空段，`{path...}` 跨段且只能放末尾，`{$}` 表示精确到结尾。
2. `PathValue("id")` 返回**已解码**的值；占位符不匹配空段，`/users/` 不会命中 `/users/{id}`。
3. 方法不匹配自动 405 + `Allow` 头；一条都不匹配才 404，不用自己判断。
4. 冲突 pattern 在**注册时 panic**；更具体的 pattern 优先于通配。

**请求与响应**

5. 写响应顺序固定：`Header().Set` → `WriteHeader(code)` → 写 body。`WriteHeader` 之后改头无效；不调 `WriteHeader` 则首次 `Write` 隐含 200。
6. 没设 `Content-Type` 时，`Write` 会对前 512 字节做 MIME 嗅探，猜错了客户端就解析失败——JSON 接口一定显式设置。
7. `Content-Type` 是普通字符串，**空格、大小写都参与比较**：`"application/json;charset=utf-8"` ≠ `"application/json; charset=utf-8"`。断言时写死同一常量，或用 `mime.ParseMediaType` 归一化。
8. 204/304 不带 body；`writeJSON` 只用于有 body 的响应。
9. 错误响应统一形状 `{"error": "..."}`，方便客户端统一解析；生产代码应先用 `http.MaxBytesReader` 限制请求体大小，并区分「JSON 非法」与「字段校验失败」两类 400。

**中间件**

10. 形状是 `func(http.Handler) http.Handler`；`Chain(h, A, B)` 调用顺序 A → B → h，返回时逆序。
11. 记录状态码必须包一层 `ResponseWriter`（`statusRecorder`），原生接口不提供读回状态码的方法。
12. `recover()` 只能在 `defer` 里生效；`Recoverer` 防止单个请求 panic 带崩进程，响应已写出时只能记日志。
13. 跨中间件传值用 `context.WithValue` + `r.WithContext`；业务参数不要塞 context。

**测试**

14. 两种粒度：`httptest.NewRecorder` 直接 `ServeHTTP`，又快又无端口；`httptest.NewServer` 起真实监听返回 `URL`，覆盖客户端与网络行为。
15. 测试用的 `NewStore()` 每次新建，测试之间互不污染；需要共享数据时抽 fixture。

**运行与关闭**

16. 生产用 `http.Server` 而不是 `http.ListenAndServe`：才能设各类超时、`ErrorLog`，也才能 `Shutdown`。
17. 收到 SIGINT → `Shutdown` 停收新连接、等在途请求；`ListenAndServe` 返回 `http.ErrServerClosed`，这是唯一可以忽略的错误（`errors.Is` 判断）。
18. `Shutdown(ctx)` 的 ctx 只是超时兜底，不取消 handler；到点后进程退出，残余连接由 OS 强断。

## 运行

```shell
# 全部测试；客户端测试需要 http/demo/.env 提供 DEEPSEEK_BASE_URL、DEEPSEEK_API_KEY
go test ./http/demo/... -v

# 只跑服务端（不需要 .env）
go test -v ./http/demo/server

# 启动服务端 demo，监听 :8080
go run ./http/demo/server/run

# 端到端示例：自动起服务、调接口、优雅暂停（约 3s 跑完）
go run ./http/demo/graceful

# 另一个终端验证
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/users
curl -X POST http://127.0.0.1:8080/users -H "Content-Type: application/json" -d '{"name":"carol"}'
curl http://127.0.0.1:8080/users/1
curl -X DELETE http://127.0.0.1:8080/users/1
```

## 参考资料

1. https://pkg.go.dev/net/http
2. https://github.com/json-iterator/go
3. https://go.dev/blog/routing-enhancements
4. https://pkg.go.dev/net/http/httptest
