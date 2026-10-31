# http

`net/http` 学习模块，分客户端与服务端两部分。客户端部分已完成：用标准库发起请求、设置请求头、解码 JSON 响应；服务端部分待补充。

## 目录结构

```text
http/
├── README.md
└── demo/
    ├── go.mod              # module golearn/http
    ├── go.sum
    ├── .env                # DEEPSEEK_BASE_URL / DEEPSEEK_API_KEY（不入库，模块根）
    ├── main.go             # demo 入口
    └── client/
        └── api_test.go     # 客户端示例：TestGet、TestPost
```

依赖：`github.com/joho/godotenv`（读 `.env`）、`github.com/json-iterator/go`（JSON 编解码）。

## 请求流程

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

## demo 索引

| 位置 | 内容 |
| --- | --- |
| `client/api_test.go` `TestGet` | GET 公开示例地址，解码为 `[]map[string]any` 后逐条打印 |
| `client/api_test.go` `TestPost` | `dotenv.Load("../.env")` 取 base URL 与 api key，POST JSON 并带 Bearer 头，解码为 `map[string]any` 后遍历 `results` 取 `title` / `url` / `summary` |

## 关键结论

1. `resp.Body` 必须关闭：`defer resp.Body.Close()`，否则连接与文件描述符泄漏。
2. 请求头要显式设置：`NewRequest` 不会自动补 `Content-Type`，鉴权头也需自行加上 `Authorization: Bearer <token>`。
3. `http.Client` 应当复用而不是每次新建，连接池才有意义；零值 `&http.Client{}` 没有超时，需要时设 `Client.Timeout`。
4. 解码成 `map[string]any` 后每一层都要类型断言（`respBody["results"].([]any)`、`result.(map[string]any)`），断言失败要单独报错，不要复用前一个 `err`。
5. `jsoniter` 与 `encoding/json` 的 `Marshal` / `Unmarshal` / `Encoder` / `Decoder` 用法一致，`MarshalToString` 是它额外提供的方法。
6. POST 的 body 被读过一次就没了，重试时要重新构造 `strings.NewReader(jsonBody)`。
7. `dotenv.Load()` 的路径相对**进程工作目录**。测试二进制的 cwd 是包目录 `http/demo/client`，而 `.env` 在模块根 `http/demo`，所以 `TestPost` 里写 `dotenv.Load("../.env")`；改在 `main` 中加载时 cwd 是 `http/demo`，直接 `Load()` 即可。

## 运行

```shell
# 需要 http/demo/.env 提供 DEEPSEEK_BASE_URL、DEEPSEEK_API_KEY
go test ./http/demo/... -v
```

## 服务端部分（待补充）

计划覆盖：`ServeMux` 路由 pattern 与 `PathValue`、`http.HandlerFunc` 与中间件、`httptest` 断言状态码与响应体。

## 参考资料

1. https://pkg.go.dev/net/http
2. https://github.com/json-iterator/go
3. https://go.dev/blog/routing-enhancements
