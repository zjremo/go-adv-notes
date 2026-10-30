package main

import (
	"unsafe"
)

// sliceHeader 描述 slice 变量内部的隐式布局：
// data 指针 + len + cap 三者内联在 slice 变量本身里，
// 因此 &s 就是头部起始地址，字段偏移交给 unsafe.Offsetof 计算。
type sliceHeader struct {
	data unsafe.Pointer
	len  int
	cap  int
}

type GenericInt interface {
	int | int32 | int64
}

func GetSliceLen[T GenericInt](s []T) T {
	offset := unsafe.Offsetof(sliceHeader{}.len)
	return T(*(*int)(unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + offset)))
}

func GetSliceCap[T GenericInt](s []T) T {
	offset := unsafe.Offsetof(sliceHeader{}.cap)
	return T(*(*int)(unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + offset)))
}

// mapHeader 对应运行时 map 结构的首字段，len(map) 读的正是它。
type mapHeader struct {
	count int
}

func GetMapLen[K string | GenericInt, V GenericInt](m map[K]V) int {
	// map 变量本身存的就是指向运行时 map 结构的指针，先取出该指针，再按偏移读 count。
	offset := unsafe.Offsetof(mapHeader{}.count)
	p := *(*unsafe.Pointer)(unsafe.Pointer(&m))
	return *(*int)(unsafe.Pointer(uintptr(p) + offset))
}
