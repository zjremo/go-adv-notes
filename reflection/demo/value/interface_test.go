package value_test

import (
	"reflect"
	"testing"
	"time"
)

// TestInterface 测试Interface()获取动态值
func TestInterface(t *testing.T) {
	vx := reflect.ValueOf(123)
	vy := reflect.ValueOf("abc")
	vz := reflect.ValueOf([]bool{false, true})
	vt := reflect.ValueOf(time.Time{})

	x := vx.Interface().(int)
	y := vy.Interface().(string)
	z := vz.Interface().([]bool)
	m := vt.MethodByName("IsZero").Interface().(func() bool)
	t.Log(x, y, z, m())

	// Interface returns v's current value as an interface{}.
	// It is equivalent to:
	//
	//	var i interface{} = (v's underlying value)
	//
	// It panics if the Value was obtained by accessing
	// unexported struct fields.
	type T struct{ x int }
	tt := &T{3}
	v := reflect.ValueOf(tt).Elem().Field(0)
	t.Log(v)
	// t.Log(v.Interface()) // panic，字段未导出，会panic
}
