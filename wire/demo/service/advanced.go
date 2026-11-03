package service

import (
	"fmt"
	"time"

	"golearn/wire/dal"
)

// TimedService 是具体类型，靠 wire.Bind 被当成 Service 使用。
type TimedService struct {
	dal.Dal
	Timeout time.Duration
}

// NewAdvancedService 的 timeout 同样由 wire.Value 提供，
// 与 wire.Struct 消费的是同一个值。
func NewAdvancedService(d dal.Dal, timeout time.Duration) *TimedService {
	return &TimedService{Dal: d, Timeout: timeout}
}

func (s *TimedService) Process() {
	fmt.Printf("advanced service: timeout=%v\n", s.Timeout)
	s.Dal.Process()
}
