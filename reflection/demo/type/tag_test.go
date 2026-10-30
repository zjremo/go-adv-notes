package type_test

import (
	"reflect"
	"testing"
)

type T struct {
	X    int  `max:"99" min:"0" default:"0"`
	Y, Z bool `optional:"yes"`
}

func TestTagGet(t *testing.T) {
	// ty := reflect.TypeOf(T{})
	// 使用TypeFor指明类型可以不用显式创建一个对象
	ty := reflect.TypeFor[T]()
	// 获取标签
	x := ty.Field(0).Tag
	y := ty.Field(1).Tag
	z := ty.Field(2).Tag
	t.Log(reflect.TypeOf(x))
	// value, bool
	v, present := x.Lookup("max")
	t.Log(len(v), present)
	t.Log(x.Get("max"))
	t.Log(x.Lookup("optional"))
	t.Log(y.Lookup("optional"))
	t.Log(z.Lookup("optional"))
}
