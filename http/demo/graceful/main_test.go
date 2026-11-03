package main

import (
	"context"
	"testing"
	"time"
)

// 关闭信号在 1s 时到达，而慢请求要 3s，Shutdown 必须等它跑完再返回。
// 断言总耗时 > 2s，就是在验证「优雅暂停」确实在等在途请求。
func TestRunGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	if err := run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 2*time.Second {
		t.Fatalf("耗时 %s，没有等待在途慢请求", elapsed)
	}
}
