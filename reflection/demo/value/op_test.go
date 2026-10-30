package value_test

import (
	"log"
	"reflect"
	"testing"
)

// TestSetField 字段设置测试
func TestSetField(t *testing.T) {
	z := 123
	y := &z
	var x any = y
	v := reflect.ValueOf(&x)
	// 一层层拆包，最终拿到z对应的
	vx := v.Elem()
	t.Log("vx.Type:", vx.Type())
	vy := vx.Elem()
	t.Log("vy.Type:", vy.Type())
	vz := vy.Elem()
	t.Log("vz.Type:", vz.Type())
	vz.Set(reflect.ValueOf(789))
	t.Log(z) // 789
}

func InvertSlice(args []reflect.Value) []reflect.Value {
	inSlice, n := args[0], args[0].Len()
	// 类似make(type, len, capacity)
	outSlice := reflect.MakeSlice(inSlice.Type(), 0, n)
	for i := n - 1; i >= 0; i-- {
		element := inSlice.Index(i)
		outSlice = reflect.Append(outSlice, element)
	}
	return []reflect.Value{outSlice}
}

func Bind(p any, f func([]reflect.Value) []reflect.Value) {
	invert := reflect.ValueOf(p).Elem()
	log.Println(invert.Type())
	invert.Set(reflect.MakeFunc(invert.Type(), f))
}

// TestSetFunc 函数类型设置
func TestSetFunc(t *testing.T) {
	var invertInts func([]int) []int
	Bind(&invertInts, InvertSlice)
	t.Log(invertInts([]int{2, 3, 5}))

	var invertStrs func([]string) []string
	Bind(&invertStrs, InvertSlice)
	t.Log(invertStrs([]string{"GO", "C"}))
}

type S struct {
	A int
	B int
}

func (s S) AddSubThenScale(val int) (int, int) {
	return s.A*val + s.B*val, s.A*val - s.B*val
}

// TestFuncCall 函数调用
func TestFuncCall(t *testing.T) {
	s := S{5, 2}
	vt := reflect.ValueOf(s)
	vm := vt.MethodByName("AddSubThenScale")
	// 类似Java中的Method.Invoke，执行函数调用
	results := vm.Call([]reflect.Value{reflect.ValueOf(3)})
	t.Log(results[0].Int(), results[1].Int())

	neg := func(x int) int {
		return -x
	}
	vf := reflect.ValueOf(neg)
	t.Log(vf.Call([]reflect.Value{reflect.ValueOf(21)})[0].Int())
	t.Log(vf.Call([]reflect.Value{vt.FieldByName("A")})[0].Int())
}

// TestMapOp 映射操作
func TestMapOp(t *testing.T) {
	valueOf := reflect.ValueOf
	m := map[string]int{
		"Unix":    1973,
		"Windows": 1984,
	}

	v := valueOf(m)
	v.SetMapIndex(reflect.ValueOf("Unix"), reflect.Value{}) // 给value设置为反射零值代表抹除这个键
	v.SetMapIndex(reflect.ValueOf("Windows"), reflect.ValueOf(1993))
	// 获取Map迭代器
	iter := v.MapRange()
	for iter.Next() {
		t.Log(iter.Key(), "\t:", iter.Value())
	}
}
