package main

import "unsafe"

// string -> []byte
func string2bytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// []byte -> string
func bytes2string(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
