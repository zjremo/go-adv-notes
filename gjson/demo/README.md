# gjson demo

用 `json-iterator` 造一份 JSON，再用 gjson 按路径取值。完整原理见 [../README.md](../README.md)。

## 跑起来

```shell
# 仓库根目录
go test ./gjson/demo/... -v
```

预期输出（`t.Log` 会带上行号）：

```text
=== RUN   TestGjson
    get_test.go:28: name: ZhangSan
    get_test.go:31: age: 22
    get_test.go:35: city: ShangHai
    get_test.go:39: hobby[0]: play basketball
    get_test.go:45: not exist!
hobby item: play basketball
hobby item: read book
--- PASS: TestGjson (0.00s)
PASS
```

## 文件职责

| 文件 | 职责 |
| --- | --- |
| `get_test.go` | 5 个用例：单字段、`info.city` 嵌套、`hobby.0` 下标、`Exists()`、`ForEach` 遍历 |
| `main.go` | 入口占位，方便 `go run ./gjson/demo` 时也有输出 |

## 小提示

- `t.Log("age:", age)` 打印出 `22`，是因为 `gjson.Result` 实现了 `String()`；换成 `age.Int()` 得到 `int64`。
- `gjson.Get` 取不到时返回空 `Result`，`String()` 为空串、`Exists()` 为 `false`，不会报错也不会 panic。
