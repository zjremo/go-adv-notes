# Reflection

Go 反射学习笔记，配套 demo 全部可跑。反射只解决一件事：**在运行时检查、构造、修改类型与值**。核心就两个字 --- `reflect.Type`（类型）和 `reflect.Value`（值），其余 API 都是它们的派生物。

## demo 索引

| 文件 | 包 | 覆盖内容 |
| --- | --- | --- |
| `demo/type/type_test.go` | `type_test` | `TestBasic` 类型信息：`Kind`/`ChanDir`/`Elem`/`Key`、`Bits`、`ConvertibleTo`、`Comparable`；`TestGetInnerMem` 结构体字段、`PkgPath`、函数签名（`NumIn`/`Out`/`In`/`IsVariadic`）、方法集（`NumMethod`/`Method`/`MethodByName`，导出方法 vs 接口全方法） |
| `demo/type/tag_test.go` | `type_test` | `reflect.TypeFor[T]()` 免实例取类型；`Field(i).Tag` + `Get`/`Lookup` 读 struct tag |
| `demo/type/tyCreate_test.go` | `type_test` | 运行时动态建类型：`ArrayOf`/`ChanOf`/`PointerTo`/`SliceOf`/`MapOf`/`FuncOf`/`StructOf` |
| `demo/value/value_test.go` | `value_test` | 可设置性入口：`ValueOf(&n)` 的 `CanSet`/`CanAddr` 都是 false，`Elem()` 后才是 true，再 `Set` |
| `demo/value/op_test.go` | `value_test` | `TestSetField` 逐层拆 `any` 链改值；`MakeFunc` + `Bind` 生成任意函数；`Call` 调方法/闭包；`SetMapIndex`（零 `Value` = 删键）、`MapRange` 迭代 |
| `demo/value/interface_test.go` | `value_test` | `Interface()` 取回动态值并断言；未导出字段调 `Interface()` 必 panic |

## 关键结论

1. **`Type` 不带值，`Value` 带类型。** `reflect.TypeOf(x)` 拿类型元信息（Kind、字段、方法、tag、布局），`reflect.ValueOf(x)` 拿值容器。
2. **可设置性只认"经由指针得到的 Elem"。** `ValueOf(变量)` 是拷贝，`CanSet() == false`；`ValueOf(&变量).Elem()` 才可写。中间多套一层 `any` 就多一次 `Elem()`，见 `TestSetField`。
3. **未导出字段被冻结。** 未导出字段的 `Value` 上 `CanInterface()` 为 false，`Interface()` 会 panic（`value obtained by accessing unexported field`）；`CanSet()` 也是 false。要突破只能用 `unsafe`，见 [unsafe 笔记](../unsafe/README.md)。
4. **tag 只是字符串。** `Tag.Get`/`Lookup` 不做校验，格式写错只能自己发现；`TypeFor[T]()` 省掉造零值那一步。
5. **`Call` 的参数必须是 `reflect.Value`，返回值也是切片。** 参数类型不匹配会 panic，不会自动转换。
6. **`MakeFunc` 的用处**：把"参数列表 + 返回值列表"在运行时接到任意 `func([]reflect.Value) []reflect.Value` 上，再 `Set` 给函数类型变量。demo 里用同一个 `InvertSlice` 生成了 `func([]int) []int` 和 `func([]string) []string` 两个函数。
7. **`Comparable`/`ConvertibleTo` 是类型层面的判断**，切片、映射不可比较（`TestBasic` 里 `tb.Comparable()` 为 false），但 `A` 数组可以。

## 测试

workspace 根目录下不能直接 `go test ./...`（见根 README），用模块路径：

```shell
go test ./reflection/demo/type/... ./reflection/demo/value/... -v
# 或
cd reflection/demo && go test ./... -v
```

## 参考资料

1. https://gfw.go101.org/article/reflection.html
2. https://golang.design/go-questions/stdlib/reflect/what/
3. https://www.cnblogs.com/jiujuan/p/17142703.html
