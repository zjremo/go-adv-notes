package type_test

import (
	"reflect"
	"testing"
)

func TestTypeCreate(t *testing.T) {
	ta := reflect.ArrayOf(5, reflect.TypeFor[int]())
	t.Log(ta)
	tc := reflect.ChanOf(reflect.SendDir, ta)
	t.Log(tc)
	tp := reflect.PointerTo(ta)
	t.Log(tp)
	ts := reflect.SliceOf(tp)
	t.Log(ts)
	tm := reflect.MapOf(ta, tc)
	t.Log(tm)
	tf := reflect.FuncOf([]reflect.Type{ta}, []reflect.Type{tp, tc}, false)
	t.Log(tf)
	tt := reflect.StructOf([]reflect.StructField{
		{Name: "Age", Type: reflect.TypeFor[string]()},
	})
	t.Log(tt)
	t.Log(tt.NumField())
}
