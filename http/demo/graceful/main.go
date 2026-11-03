// Command graceful 演示一条完整的端到端链路：
// 起服务端 → 客户端依次调用 → 触发优雅关闭（暂停服务）→
// 在途慢请求处理完，新连接被拒，进程退出。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"golearn/http/server"
)

func main() {
	// Ctrl+C 触发退出；真实服务只保留这一层。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// 演示用：1s 后自动开始优雅关闭，好让脚本自己跑完。真实服务删掉这行。
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

// run 启动服务端，用客户端把接口调一遍，然后在 ctx 取消时优雅关闭。
func run(ctx context.Context) error {
	// 随机端口，避免和本机其他服务抢 :8080。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	// 复用 ../server 包的路由，再挂一条慢接口用于演示「在途请求」。
	mux := server.NewMux(server.NewStore())
	mux.HandleFunc("GET /slow", slowHandler)

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	base := "http://" + ln.Addr().String()
	fmt.Printf("服务端已启动: %s\n", base)

	// ---------------- 客户端调用 ----------------
	client := &http.Client{Timeout: 10 * time.Second}

	health, err := getJSON(client, base+"/healthz")
	if err != nil {
		return fmt.Errorf("GET /healthz: %w", err)
	}
	fmt.Printf("GET  /healthz -> %v\n", health)

	created, err := postJSON(client, base+"/users", map[string]string{"name": "carol"})
	if err != nil {
		return fmt.Errorf("POST /users: %w", err)
	}
	// encoding/json 把 JSON 数字解到 any 时统一是 float64，不是 int。
	id, ok := created["id"].(float64)
	if !ok {
		return fmt.Errorf("POST /users 返回里没有数字 id: %v", created)
	}
	fmt.Printf("POST /users  -> id=%v name=%v\n", created["id"], created["name"])

	user, err := getJSON(client, fmt.Sprintf("%s/users/%d", base, int(id)))
	if err != nil {
		return fmt.Errorf("GET /users/%d: %w", int(id), err)
	}
	fmt.Printf("GET  /users/%d -> %v\n", int(id), user)

	// ---------------- 一个在途慢请求 ----------------
	const slow = 3 * time.Second
	slowDone := make(chan error, 1)
	go func() {
		resp, err := client.Get(base + "/slow?d=" + slow.String())
		if err != nil {
			slowDone <- err
			return
		}
		defer resp.Body.Close()
		var body map[string]any
		err = json.NewDecoder(resp.Body).Decode(&body)
		fmt.Printf("慢请求完成: status=%d body=%v\n", resp.StatusCode, body)
		slowDone <- err
	}()

	// 等慢请求进到 handler 里，再开始关闭，才能看到 Shutdown 等它跑完。
	time.Sleep(200 * time.Millisecond)

	// ---------------- 优雅关闭 ----------------
	<-ctx.Done()
	fmt.Println("收到停止信号，开始优雅关闭 ...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	fmt.Printf("Shutdown 返回，耗时 %s（在途慢请求已跑完）\n", time.Since(start).Round(100*time.Millisecond))

	if err := <-serveErr; err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	if err := <-slowDone; err != nil {
		return fmt.Errorf("在途慢请求失败: %w", err)
	}

	// ---------------- 关闭后新连接应被拒绝 ----------------
	fresh := &http.Client{
		Timeout:   time.Second,
		Transport: &http.Transport{DisableKeepAlives: true},
	}
	if _, err := fresh.Get(base + "/healthz"); err == nil {
		return errors.New("关闭后新连接仍然成功，不符合预期")
	} else {
		fmt.Printf("关闭后新连接被拒绝: %v\n", err)
	}

	fmt.Println("完成：在途请求已处理完，监听已停止，进程可以退出了")
	return nil
}

// slowHandler 睡 d 后返回，模拟耗时业务。d 限制在 (0, 5s]。
func slowHandler(w http.ResponseWriter, r *http.Request) {
	d, err := time.ParseDuration(r.URL.Query().Get("d"))
	if err != nil || d <= 0 || d > 5*time.Second {
		http.Error(w, "d must be a duration in (0, 5s]", http.StatusBadRequest)
		return
	}
	time.Sleep(d)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"slept": d.String()})
}

func getJSON(client *http.Client, url string) (map[string]any, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var v map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func postJSON(client *http.Client, url string, body any) (map[string]any, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var v map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}
