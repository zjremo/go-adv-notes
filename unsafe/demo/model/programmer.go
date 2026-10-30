package model

import "unsafe"

type Programmer struct {
	name     string
	age      int
	language string
}

func NewProgrammer(name string, age int, language string) *Programmer {
	return &Programmer{
		name:     name,
		age:      age,
		language: language,
	}
}

// OffsetOfLanguage 返回 language 字段相对结构体起始地址的字节偏移。
func OffsetOfLanguage() uintptr {
	return unsafe.Offsetof(Programmer{}.language)
}
