package type_test

import (
	"reflect"
	"testing"
)

// TestBasic 类型信息基本获取
func TestBasic(t *testing.T) {
	type A [16]int16
	var c <-chan map[A][]byte

	tc := reflect.TypeOf(c)
	t.Log(tc.Kind())    // chan
	t.Log(tc.ChanDir()) // chan direction，write or read

	// Elem returns a type's element type.
	// It panics if the type's Kind is not Array, Chan, Map, Pointer, or Slice.
	tm := tc.Elem() // map[A][]byte

	// Key returns a map type's key type.
	// It panics if the type's Kind is not Map.
	ta, tb := tm.Key(), tm.Elem() // ta: A tb: []byte
	t.Log(tm.Kind(), ta.Kind(), tb.Kind())
	tx, ty := ta.Elem(), tb.Elem()

	t.Log(tx.Kind(), ty.Kind()) // tx: int16 ty: uint8
	t.Log(tx.Bits(), ty.Bits()) // 16, 8
	t.Log(tx.ConvertibleTo(ty)) // true
	t.Log(tb.ConvertibleTo(ta)) // false

	// 切片类型和映射类型不可比较
	t.Log(tb.Comparable()) // false
	t.Log(tm.Comparable()) // false
	t.Log(ta.Comparable()) // true
	t.Log(tc.Comparable()) // true
}

type F func(string, int) bool

func (f F) m(s string) bool {
	return f(s, 32)
}

func (f F) M() {
}

type I interface {
	m(s string) bool
	M()
}

// TestGetInnerMem 获取内部成员
func TestGetInnerMem(t *testing.T) {
	var x struct {
		F F
		i I
	}
	tx := reflect.TypeOf(x)
	t.Log(tx.Kind())        // struct
	t.Log(tx.NumField())    // 2
	t.Log(tx.Field(1).Name) // i
	// 路径（PkgPath）是非导出字段（或者方法）的内在属性。
	t.Log(tx.Field(0).PkgPath)
	t.Log(tx.Field(1).PkgPath)

	tf, ti := tx.Field(0).Type, tx.Field(1).Type
	t.Log(tf.Kind())
	// 函数类型是否是变参函数
	// func A(num ...int)
	t.Log(tf.IsVariadic())

	// 函数的输入参数有几个，输出结果有几个
	// 获取函数的参数类型
	t.Log(tf.NumIn(), tf.NumOut()) // 2 1
	t0, t1, t2 := tf.In(0), tf.In(1), tf.Out(0)
	// string int bool
	t.Log(t0.Kind(), t1.Kind(), t2.Kind())

	// For a non-interface type, it returns the number of exported methods.
	//
	// For an interface type, it returns the number of exported and unexported methods.
	t.Log(tf.NumMethod(), ti.NumMethod()) // 1, 2
	t.Log(tf.Method(0).Name)              // M
	t.Log(ti.Method(1).Name)              // m
	_, ok1 := tf.MethodByName("m")
	_, ok2 := tf.MethodByName("m")
	t.Log(ok1, ok2)
}
