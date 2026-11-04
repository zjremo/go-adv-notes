package main

import (
	"fmt"
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/tidwall/gjson"
)

func TestGjson(t *testing.T) {
	m := map[string]any{
		"name":  "ZhangSan",
		"age":   22,
		"hobby": []string{"play basketball", "read book"},
		"info": map[string]any{
			"city": "ShangHai",
		},
	}

	s, err := jsoniter.MarshalToString(m)
	if err != nil {
		t.Fatalf("jsoniter marshal failed, m: %#v, err: %v", m, err)
	}

	// 1. 获取单个字段
	name := gjson.Get(s, "name")
	t.Log("name:", name.String())

	age := gjson.Get(s, "age")
	t.Log("age:", age)

	// 2. 获取嵌套字段, .分隔
	city := gjson.Get(s, "info.city")
	t.Log("city:", city.String())

	// 3. 获取数组元素，索引从0开始
	hobby0 := gjson.Get(s, "hobby.0")
	t.Log("hobby[0]:", hobby0.String())

	// 4. 判断key是否存在
	if gjson.Get(s, "not exist").Exists() {
		t.Log("exist!")
	} else {
		t.Log("not exist!")
	}

	// 5. 遍历数组
	hobbyArr := gjson.Get(s, "hobby")
	hobbyArr.ForEach(func(key, value gjson.Result) bool {
		fmt.Println("hobby item:", value.String())
		return true
	})
}
