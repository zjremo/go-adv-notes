# unsafe.Pointer

`unsafe` 绕开 Go 的类型系统与内存安全：能在运行时按地址读写任意内存、在 `*T` 与 `unsafe.Pointer` 之间自由转换、按字节偏移访问结构体私有字段。代价是编译器不再做任何检查，越界、错位、类型不匹配均为未定义行为，因此本目录代码定位为运行时结构的学习示例，而非可直接复制的生产代码。

## 三条基本规则

| 转换 | 是否允许 | 说明 |
| --- | --- | --- |
| `*T` ⇄ `unsafe.Pointer` | 任意类型，无限次 | 不做检查，仅在编译期更换类型 |
| `unsafe.Pointer` ⇄ `uintptr` | 允许 | `uintptr` 是整数，GC 不视其为指针 |
| `unsafe.Pointer` 直接算术 | 不允许 | `p + 8` 无法编译，必须先转 `uintptr` |

因此地址运算统一写成：

```go
*(*T)(unsafe.Pointer(uintptr(unsafe.Pointer(&x)) + offset))
//  ____/ ___________________/ ___________________________/
//   读 T       转回指针             原地址 → uintptr → 加偏移
```

`uintptr` 的存活期限定在单个表达式内。若将其存入变量并在后续语句中转回 `unsafe.Pointer`，中途发生 GC 会得到悬垂指针，官方 `unsafe` 文档对此有明确禁止。

## demo 索引

| 文件 | 内容 |
| --- | --- |
| `demo/op.go` | `GetSliceLen`/`GetSliceCap`：以 `sliceHeader` 字段为基准，用 `unsafe.Offsetof` 取偏移后读取；`GetMapLen`：先取出 map 变量中的指针，再按 `mapHeader` 字段偏移读取 |
| `demo/op_test.go` | `TestSliceOp` 与 `len`/`cap` 对拍；`TestMapOp` 与 `len` 对拍；`TestModifyPriMember` 修改私有字段 |
| `demo/model/programmer.go` | `Programmer{name, age, language}` 全私有字段，并以 `OffsetOfLanguage()` 返回字段偏移 |
| `demo/zeroCopy.go` | `string2bytes`/`bytes2string`：基于 `unsafe.Slice` 与 `unsafe.String` 的零拷贝互转 |
| `demo/zeroCopy_test.go` | `TestZeroCopy`：比对 `StringData`/`SliceData` 地址，确认未发生拷贝 |
| `demo/main.go` | demo 入口 |

## slice 的内存布局

slice 变量内部依次保存数据指针、长度、容量，三者内联在变量本身中：

```go
type sliceHeader struct {
	data unsafe.Pointer // 数据指针
	len  int            // 长度
	cap  int            // 容量
}
```

`op.go` 以该结构作为字段基准计算偏移：

```go
func GetSliceLen[T GenericInt](s []T) T {
	offset := unsafe.Offsetof(sliceHeader{}.len)
	return T(*(*int)(unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + offset)))
}

func GetSliceCap[T GenericInt](s []T) T {
	offset := unsafe.Offsetof(sliceHeader{}.cap)
	return T(*(*int)(unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + offset)))
}
```

- `&s` 即头部起始地址，加上 `len`/`cap` 的字段偏移即可定位到对应整数。
- 偏移由编译器通过 `unsafe.Offsetof` 计算，字段声明顺序或平台指针宽度变化时自动跟随，代码中不出现固定字节数。
- 读取宽度固定为 `int`，再由泛型参数 `T` 承接返回值；`int`/`int32`/`int64` 均按调用方声明转换。

## map 的内存布局

map 变量只保存一个指针，指向运行时的 map 结构，因此访问结构内部字段需要先解出该指针：

```go
type mapHeader struct {
	count int // 运行时的元素个数，len(map) 读取的正是该字段
}

func GetMapLen[K string | GenericInt, V GenericInt](m map[K]V) int {
	offset := unsafe.Offsetof(mapHeader{}.count)
	p := *(*unsafe.Pointer)(unsafe.Pointer(&m))
	return *(*int)(unsafe.Pointer(uintptr(p) + offset))
}
```

- slice 的头部内联在变量中，而 map 变量本身即指针，这是两者访问方式不同的根本原因。
- 首字段名称随版本变化：Go 1.23 及以前为 `hmap.count int`；Go 1.24 起改用 Swiss table，`internal/runtime/maps/map.go` 中 `type Map struct` 的首字段为 `used uint64`，并注明该字段必须位于首位（`len()` 内置函数依赖此布局）；编译器在 `ssagen` 中对 `case ir.OLEN` 取首字段后转换为 `int`。
- 示例按 `int` 读取首字，与 64 位平台上的 `uint64` 宽度一致。nil map 经 unsafe 解引用会 panic，而内置 `len` 对 nil map 返回 0。

## 私有成员访问与修改

私有性属于编译期可见性约束，内存布局上与导出字段无差别。字段偏移由定义该字段的包计算并导出：

```go
// model 包内
func OffsetOfLanguage() uintptr {
	return unsafe.Offsetof(Programmer{}.language)
}

// demo 中按偏移取地址并写入
p := model.NewProgrammer("XiaoMing", 22, "go")
lang := (*string)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + model.OffsetOfLanguage()))
*lang = "Golang"
```

- 私有字段的选择器无法在包外书写，因此偏移由 `model` 包自行计算后以函数形式暴露。
- 结构体字段顺序或类型调整时，`OffsetOfLanguage` 的返回值随之更新，调用方无需改动。
- 反射侧对应能力见 [reflection 笔记](../reflection/README.md)：`reflect.Value.UnsafeAddr` 不校验未导出标志，`reflect.NewAt` 可将只读 `Value` 转为可写。

## 字符串和 []byte 的零拷贝切换

`string` 与 `slice` 的底层结构前两个字同构（`reflect.StringHeader`/`reflect.SliceHeader` 已废弃，仅用于说明布局）：

```go
type StringHeader struct {
	Data uintptr // 数据指针
	Len  int     // 长度
}

type SliceHeader struct {
	Data uintptr // 数据指针
	Len  int     // 长度
	Cap  int     // 容量
}
```

共享 `Data` 与 `Len` 即可完成零拷贝转换：

```go
// string -> []byte
func string2bytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// []byte -> string
func bytes2string(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
```

- Go 1.20 起提供官方 API `unsafe.StringData`、`unsafe.SliceData`、`unsafe.String`、`unsafe.Slice`，无需自行覆盖头部结构。
- `TestZeroCopy` 通过 `unsafe.StringData(s) == unsafe.SliceData(b)` 断言地址一致，证明未发生复制。
- 转换得到的 `[]byte` 仍指向原字符串的只读内存，写入属于未定义行为；反向转换得到的 `string` 与原 slice 共享底层内存，其内容会随 slice 变化。

## 运行

```shell
go test ./unsafe/demo/... -v
go test -run TestZeroCopy ./unsafe/demo/... -v
go run ./unsafe/demo
```

## 参考资料

1. https://golang.design/go-questions/stdlib/unsafe/pointers/
2. https://pkg.go.dev/unsafe