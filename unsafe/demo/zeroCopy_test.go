package main

import (
	"testing"
	"unsafe"
)

func TestZeroCopy(t *testing.T) {
	s := "hello, world"
	b := string2bytes(s)

	ps, pb := unsafe.StringData(s), unsafe.SliceData(b)
	if pb != ps {
		t.Errorf("TestZeroCopy string2bytes failed, data addr of s(%p) != data addr of b(%p)\n", ps, pb)
	}
	t.Logf("TestZeroCopy string2bytes pass, data addr of s(%p) = data addr of b(%p)\n", ps, pb)

	b2 := []byte{1, 2, 3, 4}
	s2 := bytes2string(b2)
	ps2, pb2 := unsafe.StringData(s2), unsafe.SliceData(b2)
	if pb2 != ps2 {
		t.Errorf("TestZeroCopy bytes2string failed, data addr of s(%p) != data addr of b(%p)\n", ps2, pb2)
	}
	t.Logf("TestZeroCopy bytes2string pass, data addr of s(%p) = data addr of b(%p)\n", ps2, pb2)
}
