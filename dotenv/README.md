# dotenv

`.env` 文件加载：把 `KEY=VALUE` 形式的配置注入进程环境变量，代码里统一用 `os.Getenv` 读取。Go 标准库没有对应实现，本模块 demo 使用 [github.com/joho/godotenv](https://github.com/joho/godotenv) `v1.5.1`。

## .env 文件格式

```shell
# 注释以 # 开头
name=zjremo
job=student
age=22

# 允许 export 前缀，与 shell 脚本兼容
export TOKEN=abc123

# 值中含空格、# 等字符时用引号包裹；双引号内的 \n 会被解析为换行
MOTTO="hello world"
```

规则要点：

| 写法 | 说明 |
| --- | --- |
| `KEY=value` | 键名前的空格忽略，未加引号的值会去掉首尾空白 |
| `export KEY=value` | 前缀 `export` 会被丢弃，仅取键值 |
| `# ...` | 整行注释；引号内的 `#` 不视为注释 |
| `KEY="a b"` | 双引号、单引号都会剥掉引号，保留内部内容 |

## godotenv API

| 函数 | 作用 | 是否写入环境变量 |
| --- | --- | --- |
| `Load(files...)` | 读取文件，默认 `.env` | 是，且**不覆盖**已存在的变量 |
| `Overload(files...)` | 同上 | 是，**覆盖**已存在的变量 |
| `Read(files...)` | 返回 `map[string]string` | 否 |
| `Unmarshal(str)` | 解析字符串为 map | 否 |
| `Marshal(map)` | 反向生成 `.env` 文本 | 否 |
| `Parse(io.Reader)` | 从任意 `io.Reader` 解析 | 否 |

## demo 索引

| 文件 | 内容 |
| --- | --- |
| `demo/dotenv_test.go` | `TestDotenv`：`dotenv.Load()` 后依次校验 `name`、`job`、`age`，数字用 `strconv.Atoi` 转换 |
| `demo/.env` | 本地测试用文件，不进版本库（见「关键结论」第 4 条） |
| `demo/main.go` | demo 入口 |
| `demo/go.mod` | `module golearn/dotenv`，依赖 `godotenv v1.5.1` |

## 关键结论

1. `Load()` 不覆盖已存在的环境变量。若 CI 或父进程已设置同名变量，文件中的值不会生效；需要强制以文件为准时用 `Overload()`。
2. `Load()` 的相对路径以**进程工作目录**为准。`go test ./dotenv/demo/...` 会把测试进程的工作目录切到包目录，所以 `demo/.env` 能被直接找到；而 `go run ./dotenv/demo` 的工作目录仍是调用者所在目录，需要显式传路径：`Load("dotenv/demo/.env")`。
3. 环境变量本质都是字符串。布尔、数字、时长等类型由业务代码自行解析，demo 中 `age` 使用 `strconv.Atoi`。
4. `.env` 属于本地配置，不应提交到版本库；仓库应提交 `.env.example` 描述所需变量，并在 `.gitignore` 中忽略 `.env`。
5. 需要一次性加载多个文件（如 `.env` + `.env.local`）时，按顺序传入：`Load(".env", ".env.local")`，后者的值优先（对未在环境中存在的变量而言）。

## 运行

```shell
# 测试会切到包目录，Load() 能找到 demo/.env
go test ./dotenv/demo/... -v

# 进入模块内运行
cd dotenv/demo && go test ./... -v
```

## 参考资料

1. https://github.com/joho/godotenv