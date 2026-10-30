package main

import (
	"golearn/unsafe/model"
	"testing"
	"unsafe"
)

func TestSliceOp(t *testing.T) {
	s := make([]int, 9, 20)
	targetLen, targetCap := len(s), cap(s)

	if len := GetSliceLen(s); len != targetLen {
		t.Errorf("TestSliceOp GetSliceLen not pass, len = %d, targetLen = %d\n", len, targetLen)
	}
	t.Logf("TestSliceOp GetSliceLen len = %d, pass!\n", targetLen)

	if cap := GetSliceCap(s); cap != targetCap {
		t.Errorf("TestSliceOp GetSliceCap not pass, len = %d, targetCap = %d\n", cap, targetCap)
	}
	t.Logf("TestSliceOp GetSliceCap cap = %d, pass!\n", targetCap)
}

func TestMapOp(t *testing.T) {
	m := make(map[string]int)
	m["qcrao"] = 100
	m["stefno"] = 18

	targetCount := len(m)
	if count := GetMapLen(m); count != targetCount {
		t.Errorf("TestMapOp GetMapLen not pass, count = %d, targetCount = %d\n", count, targetCount)
	}
	t.Logf("TestMapOp GetMapLen count = %d, pass!\n", targetCount)
}

func TestModifyPriMember(t *testing.T) {
	p := model.NewProgrammer("XiaoMing", 22, "go")
	t.Logf("p: %v\n", p)
	lang := (*string)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + model.OffsetOfLanguage()))
	t.Logf("lang: %s\n", *lang)
	*lang = "Golang"
	t.Logf("After modified, p: %v\n", p)
}