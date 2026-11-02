package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"golearn/http/server"
)

func main() {
	// 创建logger日志管理器
	// log.LstdFlags 全局默认logger自带的格式标记
	logger := log.New(os.Stdout, "http-server ", log.LstdFlags)

	// 中间件从外到内：日志 → panic 恢复 → trace id → 路由。
	handler := server.Chain(
		server.NewMux(server.NewStore()),
		server.Logging(logger),
		server.Recoverer(logger),
		server.TraceID(),
	)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Ctrl+C 后开始优雅关闭：停止接收新连接，等待在途请求处理完。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		logger.Printf("listening on http://127.0.0.1%s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("serve failed: %v", err)
		}
	}()

	<-ctx.Done()
	stop()
	logger.Println("shutting down ...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
	}
	logger.Println("bye")
}
