# gjson 笔记

`github.com/tidwall/gjson`：**只读**的 JSON 取值库。不解析成 struct、不校验 schema，直接按路径从原始 JSON 字符串里挖出你要的值。

适合：

- 只要一两个字段，不想为它定义整套 struct；
- JSON 结构不固定（日志、第三方接口、配置）；
- 大 JSON 里只取一小块，不想整体 `Unmarshal`。

不适合：写回/修改 JSON（用 [`sjson`](https://github.com/tidwall/sjson)）、需要严格类型校验和错误定位的场景。

## 安装

```shell
go get github.com/tidwall/gjson@v1.20.0
```

demo 里还用了 `json-iterator/go` 把 `map` 编成 JSON 字符串 —— gjson 只负责读，造 JSON 这一步换成 `encoding/json` 也一样。所以 `go.mod` 同时有这两个依赖：

```text
require github.com/tidwall/gjson v1.20.0
require github.com/json-iterator/go v1.1.12
require github.com/tidwall/match v1.1.1 // indirect   # 通配符匹配
require github.com/tidwall/pretty v1.2.0 // indirect  # @pretty 等修饰符
```

## demo 目录

```text
gjson/demo/
├── go.mod        # module golearn/gjson
├── go.sum
├── main.go       # 入口占位
├── get_test.go   # 5 个基本用法，测试即示例
└── README.md     # 三步跑起来
```

运行：

```shell
go test ./gjson/demo/... -v
```

`get_test.go` 覆盖的 5 件事：单字段、嵌套字段、数组下标、key 是否存在、遍历数组。

## 核心 API

| API | 签名 | 说明 |
| --- | --- | --- |
| `Get` | `Get(json, path string) Result` | 从字符串取路径，最常用 |
| `GetBytes` | `GetBytes(json []byte, path string) Result` | `[]byte` 版本，省掉转 `string` 的分配 |
| `GetMany` | `GetMany(json string, path ...string) []Result` | 多个路径一次调用；内部就是对每个 path 调一次 `Get`，不是一次扫描 |
| `Parse` | `Parse(json string) Result` | 拿到根 `Result`，之后用 `Result.Get` 链式取 |
| `ParseBytes` | `ParseBytes(json []byte) Result` | 同上，`[]byte` 版 |
| `Valid` | `Valid(json string) bool` | 全量语法校验，要「确认不是坏 JSON」时才用 |
| `ValidBytes` | `ValidBytes(json []byte) bool` | 同上 |
| `ForEachLine` | `ForEachLine(json string, fn func(Result) bool)` | 逐行读 JSON Lines |

`Parse` 是**惰性**的：不会扫描整个文档，也不会告诉你 JSON 是否合法；路径取不到就返回一个「不存在」的 `Result`。

### Result

```go
type Result struct {
	Type    Type    // Null / False / Number / String / True / JSON
	Raw     string  // 原始 JSON 片段
	Str     string  // 字符串值
	Num     float64 // 数字值
	Index   int     // Raw 在原 JSON 中的起始下标，0 表示未知
	Indexes []int   // 含 # 查询时，所有命中的下标
}
```

常用方法：

| 方法 | 返回 | 说明 |
| --- | --- | --- |
| `String()` | string | 字符串→内容；数字→原始文本；布尔→`"true"`/`"false"`；对象/数组→`Raw` |
| `Int()` / `Uint()` | int64 / uint64 | 64 位精确读整数 |
| `Float()` | float64 | 数字；字符串会尝试 `ParseFloat` |
| `Bool()` | bool | `true`/`false`，字符串 `"true"` 也认 |
| `Time()` | time.Time | 只认 **RFC3339**，解析失败返回零值（不报错） |
| `Value()` | any | `bool` / `float64` / `string` / `nil` / `map[string]any` / `[]any` |
| `Array()` | []Result | 非数组返回单元素切片；`null` 或不存在返回空切片 |
| `Map()` | map[string]Result | 对象转 map |
| `Get(path)` | Result | 在结果上继续按路径取 |
| `ForEach(fn)` | — | 遍历数组或对象；`fn` 返回 `false` 提前停 |
| `Exists()` | bool | 注意：**`null` 也返回 `true`**，只有「路径没取到」才是 `false` |

## 路径语法

下面用 `get_test.go` 里那份 JSON 演示：

```json
{
  "name": "ZhangSan",
  "age": 22,
  "hobby": ["play basketball", "read book"],
  "info": {"city": "ShangHai"}
}
```

### 基本与数组

| 路径 | 结果 | 说明 |
| --- | --- | --- |
| `name` | `"ZhangSan"` | 对象字段 |
| `info.city` | `"ShangHai"` | `.` 逐层下钻 |
| `hobby` | `[...]` | 整个数组 |
| `hobby.0` | `"play basketball"` | 下标从 0 开始 |
| `hobby.#` | `2` | `#` 单独用表示数组长度 |
| `friends.#.age` | `[44,68,47]` | 对象数组投影（`friends` 取自官方语法示例） |

### 通配符与转义

| 路径 | 说明 |
| --- | --- |
| `child*.2` | `*` 匹配任意个字符 |
| `c?ildren.0` | `?` 匹配一个字符 |
| `fav\.movie` | key 本身含 `.` 时用 `\` 转义（Go 里写字符串要 `"fav\\.movie"`，或直接用反引号） |

### 查询

`#(...)` 找第一个匹配，`#(...)#` 找全部匹配：

```text
friends.#(last=="Murphy").first     → "Dale"
friends.#(last=="Murphy")#.first    → ["Dale","Jane"]
friends.#(age>45)#.last             → ["Craig","Murphy"]
children.#(%"*a*")#                → ["Sara","Jack"]
```

操作符：

| 操作符 | 含义 |
| --- | --- |
| `==` `!=` | 相等 / 不等 |
| `<` `<=` `>` `>=` | 数值或字符串比较 |
| `%` | 通配符匹配（like） |
| `!%` | 不匹配（not like） |
| `~true` `~false` `~null` `~*` | 把值先按「真值/假值/空/是否存在」转成布尔再比 |

查询可以嵌套：`friends.#(nets.#(=="fb"))#.first`。

### `.` 与 `|` 的区别

两者都是分隔符，差别在 `#` 或 `#(...)#` 之后：

- `.first`：对结果里的**每个元素**应用 `first`（投影）；
- `|first`：对整个结果应用一次 —— 结果是数组时 `first` 取不到，通常要配 `|0`、`|#` 这类下标/长度。

```text
friends.#(last=="Murphy")#.first   → ["Dale","Jane"]
friends.#(last=="Murphy")#|first   → 不存在
friends.#(last=="Murphy")#|0       → 第一个匹配对象
friends.#(last=="Murphy")#|#       → 2
```

### 多路径与字面量

`{...}` 拼对象、`[...]` 拼数组，可以在一次查询里挑多个字段；字段名可以显式指定：

```text
{name.first,age,"the_murphys":friends.#(last=="Murphy")#.first}
→ {"first":"Tom","age":37,"the_murphys":["Dale","Jane"]}
```

以 `!` 开头的是 JSON 字面量，用来往多路径结果里塞常量：

```text
{name.first,"company":!"Happysoft","employed":!true}
```

### 修饰符

`@name` 是路径组件，对当前 JSON 做一次转换，可以继续串：`children|@reverse|0`。

| 修饰符 | 作用 |
| --- | --- |
| `@reverse` | 反转数组/对象成员顺序 |
| `@ugly` | 去掉所有空白 |
| `@pretty` | 美化，可带参数 `@pretty:{"sortKeys":true}` |
| `@this` | 返回当前元素（常用来拿根节点） |
| `@valid` | 校验 JSON 是否合法 |
| `@flatten` | 拍平嵌套数组 |
| `@join` | 多个对象合并成一个 |
| `@keys` / `@values` | 取对象的键 / 值数组 |
| `@tostr` / `@fromstr` | 把 JSON 包成字符串 / 从字符串解包 |
| `@group` / `@dig` | 按字段分组 / 不用完整路径搜索 |

自定义修饰符：

```go
gjson.AddModifier("case", func(json, arg string) string {
	if arg == "upper" {
		return strings.ToUpper(json)
	}
	return json
})
// 路径：children.@case:upper
```

### JSON Lines

多行 JSON（每行一个独立文档）可以用 `..` 前缀把它当成数组，或直接用 `ForEachLine`：

```go
gjson.ForEachLine(json, func(line gjson.Result) bool {
	fmt.Println(line.String())
	return true // 返回 false 提前结束
})
```

## 类型与精度

- `Result` 内部只区分 6 种 `Type`：`Null`、`False`、`Number`、`String`、`True`、`JSON`（对象/数组都算 `JSON`）。
- `String()` 对整数返回**原始文本**（`"22"`），不会变成 `22.000000`。
- `Int()`/`Uint()` 能精确读满 64 位；`Float()`/`Value()` 走 `float64`，超过 `2^53` 的整数会丢精度——这种字段用 `Int()`/`Uint()` 或直接读 `Raw`。
- `Exists()` 判断的是「路径是否命中」：JSON 里 `"a": null` 时 `Get(json,"a").Exists()` 为 `true`；想区分 `null` 用 `Type == gjson.Null`。
- `Value()` 的类型映射：`bool` / `float64` / `string` / `nil` / `map[string]any` / `[]any`，所以断言数字要用 `float64`。
- `Time()` 只解析 RFC3339，失败返回 `time.Time{}`，不会给你 error。

## 和 encoding/json 怎么选

| 场景 | 选择 |
| --- | --- |
| 只有几个字段、结构不固定 | gjson |
| 要整体映射进 struct 并做类型/必填校验 | `encoding/json`（或 jsoniter） |
| 超大 JSON 里取一小块 | gjson（`GetBytes` + 子串，几乎零分配） |
| 需要修改 JSON 再写回 | `sjson`，gjson 只读 |
| 需要精确错误位置 | `encoding/json` 的 `SyntaxError` |

## 坑

1. `Get` 不做整体校验：JSON 坏掉时可能静默返回空结果，依赖完整性时先 `gjson.Valid`。
2. 路径 key 含 `.`、`*`、`?` 必须转义，否则会被当成分隔符/通配符。
3. 数字精度：默认 `float64`，大整数用 `Int()`/`Uint()`。
4. `#` 与 `#(...)#` 语义不同：前者长度，后者「全部匹配」。
5. `null` 也算存在，别用 `Exists()` 当「非空」判断。
6. key 和通配符匹配都是大小写敏感的。

## demo 索引

| 文件 | 内容 |
| --- | --- |
| `gjson/demo/get_test.go` | `TestGjson`：单字段、嵌套、数组下标、`Exists`、`ForEach` |
| `gjson/demo/main.go` | 入口占位，打印一行 |
| `gjson/demo/README.md` | 三步跑起来 + 预期输出 |

## 参考资料

- 仓库与 API：<https://github.com/tidwall/gjson>
- 路径语法全文：<https://github.com/tidwall/gjson/blob/master/SYNTAX.md>
- 在线试验：<https://gjson.dev>
- 同作者的写回库：<https://github.com/tidwall/sjson>
