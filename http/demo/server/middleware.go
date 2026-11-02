package server

import (
	"context"
	"log"
	"net/http"
	"time"
)

// Middleware 是标准中间件形状：接收 Handler，返回包了一层的 Handler。
// 这是一种逻辑增强，类似Java AOP
type Middleware func(http.Handler) http.Handler

// Chain: 执行顺序包装。比如mws: [A, B]
// A(B(h)) 执行顺序为A -> B -> h
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// statusRecorder 记录实际写出的状态码；Handler 没显式调用 WriteHeader 时默认 200。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// Logging 打印方法、路径、状态码与耗时。
func Logging(logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			logger.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond))
		})
	}
}

// Recoverer 把 panic 变成 500 响应，避免单个请求崩掉整个进程。
func Recoverer(logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					logger.Printf("panic: %v", v)
					writeError(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type ctxKey string

const traceIDKey ctxKey = "trace-id"

// TraceID 从请求头 X-Trace-ID 取值，缺省时生成一个，写回响应头并放进 context 供下游读取。
func TraceID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Trace-ID")
			if id == "" {
				id = time.Now().Format("20060102150405.000000")
			}
			// 响应头写入X-Trace-ID
			w.Header().Set("X-Trace-ID", id)
			ctx := context.WithValue(r.Context(), traceIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TraceIDFromContext 读取 TraceID 中间件放进 context 的值。
func TraceIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(traceIDKey).(string)
	return id
}
