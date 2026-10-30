# go-adv-notes

Go 进阶知识点学习仓库。**一个主题一个 module**，用 `go.work` 聚合成单个工作区：

- 每个主题的笔记写在 `<主题>/README.md`；
- 可运行代码放在 `<主题>/demo/`。

每个 `demo/` 都能单独 `cd` 进去当普通 `module` 编译运行，也能在仓库根目录用 workspace 一次性全部跑通。

## 目录结构

```text
go-adv-notes/
├── go.work                     # workspace 定义：登记所有主题模块
├── README.md
├── reflection/                 # 主题：reflect
│   ├── README.md               # 笔记：Type/Value 两件套 + demo 索引
│   └── demo/
│       ├── go.mod              # module golearn/reflection
│       ├── main.go             # demo 入口
│       ├── type/               # reflect.Type：类型信息、struct tag、动态建类型
│       │   ├── type_test.go
│       │   ├── tag_test.go
│       │   └── tyCreate_test.go
│       └── value/              # reflect.Value：可设置性、MakeFunc、Call、Map
│           ├── value_test.go
│           ├── op_test.go
│           └── interface_test.go
└── unsafe/                     # 主题：unsafe.Pointer
    ├── README.md               # 笔记：指针转换、私有成员、零拷贝
    └── demo/
        ├── go.mod              # module golearn/unsafe
        ├── main.go             # demo 入口
        ├── op.go               # 用 unsafe 读 slice header 的 len/cap
        ├── op_test.go
        ├── zeroCopy.go         # string ⇄ []byte 零拷贝转换
        ├── zeroCopy_test.go
        └── model/
            └── programmer.go   # 私有字段的结构体，供访问私有成员实验用
```

约定（新增主题时保持一致）：

| 项目 | 规则 |
| --- | --- |
| 目录 | `<主题>/demo`，主题名用小写英文 |
| module | `golearn/<主题>` |
| 笔记 | `<主题>/README.md` |
| 代码 | 只放在 `<主题>/demo`，测试与被测代码同包 |

## go.work 使用

根目录**只有** `go.work`，没有 `go.mod` —— 这是理解本仓库所有命令的前提。

```go
go 1.25.0

use (
	./reflection/demo
	./unsafe/demo
)
```

### 常用命令

| 目的 | 命令 |
| --- | --- |
| 确认当前生效的 workspace 文件 | `go env GOWORK` |
| 列出工作区内的所有模块 | `go list -m` |
| 列出模块名与磁盘路径 | `go list -m -f '{{.Path}} => {{.Dir}}'` |
| 跑指定模块的测试 | `go test ./reflection/demo/... -v` |
| 跑 unsafe demo 的测试 | `go test ./unsafe/demo/... -v` |
| 模块内使用通配符 | `cd unsafe/demo && go test ./... -v` |
| 跑全部模块的测试 | `go test -v $(go list -m -f '{{.Dir}}/...')` |
| 运行某个 demo | `go run ./unsafe/demo` |
| 登记一个新模块 | `go work use ./x/demo` |
| 批量登记目录树下所有模块 | `go work use -r .` |
| 移除一个模块 | `go work edit -dropuse ./x/demo` |
| 查看 go.work 的结构化内容 | `go work edit -json` |
| 把工作区依赖版本写回各模块 go.mod | `go work sync` |

> 「跑全部模块」命令行依赖 `go list` 的输出做参数展开，仓库路径含空格时会失效；遇到这种情况就逐个模块跑，或改用 `cd <主题>/demo`。

### 根目录不能用 `./...`直接测试

```text
$ go test ./... -v
pattern ./...: directory prefix . does not contain modules listed in go.work or their selected dependencies
```

`./...` 的含义是「当前目录及子目录下的包」，Go **需要先把 `.` 归属到某个 `module`** 才能展开它。而 workspace 根目录本身不在 `use` 列表里，于是无法解析。

两种正确姿势：

```shell
# 1. 显式写模块路径
go test ./unsafe/demo/... -v

# 2. 进入模块，./... 照常可用
cd unsafe/demo && go test ./... -v
```

**不要在根目录补一个 `go.mod` 来解决这个报错**：嵌套 module（`reflection/demo`、`unsafe/demo`）会被父 module 的 `./...` 排除，报错只是换成 `matched no packages`，反而把 workspace 的依赖视图搅乱。

## 新增一个主题

```shell
mkdir -p generics/demo
cd generics/demo
go mod init golearn/generics
cd ../..

go work use ./generics/demo   # 登记到 go.work
go list -m                    # 确认已出现在模块列表里

go test ./generics/demo/... -v
```

忘了 `go work use` 的典型症状：`go list -m` 里看不到它，模块内的 `./...` 照样能跑，但从根目录显式指定路径会报「matched no packages」。

## 主题进度

| 主题 | module | 状态 |
| --- | --- | --- |
| [reflection](reflection/README.md) | `golearn/reflection` | 笔记 + demo（Type 信息/标签/动态建类型、Value 可设置性与 MakeFunc/Call/Map） |
| [unsafe](unsafe/README.md) | `golearn/unsafe` | 笔记 + demo（slice/map header 布局、string ⇄ []byte 零拷贝、私有成员读写、两条转换规则与 uintptr 纪律） |