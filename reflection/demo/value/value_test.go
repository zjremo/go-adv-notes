package value_test

import (
	"reflect"
	"testing"
)

func TestValueBaisc(t *testing.T) {
	n := 123
	p := &n
	vp := reflect.ValueOf(p)
	t.Log(vp.CanSet(), vp.CanAddr()) // false false
	vn := vp.Elem()
	t.Log(vn.CanSet(), vn.CanAddr()) // true true
	vn.Set(reflect.ValueOf(789))
	t.Log(n)
}
