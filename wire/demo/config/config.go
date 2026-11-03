// Package config 提供进阶 demo 用的配置类型。
//
// 注意 DSN 是独立类型而不是裸 string：
// wire 按类型匹配 provider，两个 string 参数无法区分谁是谁。
package config

import "time"

// DSN 数据源名称。
type DSN string

// Options 由多个零散的 provider 拼成，用来演示 wire.Struct。
type Options struct {
	Region  string
	Timeout time.Duration
	Debug   bool
}
