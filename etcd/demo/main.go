package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/joho/godotenv"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	etcdEndpoint, serviceRoot string
)

// 服务注册实例结构体
type ServiceInstance struct {
	ID      string `json:"id"`
	Address string `json:"address"`
}

func newClient() (*clientv3.Client, error) {
	return clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdEndpoint},
		DialTimeout: 5 * time.Second,
	})
}

/*
1. Basic KV Operations
*/
func basicKV(cli *clientv3.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("\n===== Basic KV=====")

	// Put
	_, err := cli.Put(ctx, "/demo/config/db", "mysql")
	if err != nil {
		return err
	}

	_, err = cli.Put(ctx, "/demo/config/redis", "redis")
	if err != nil {
		return err
	}

	// Get
	resp, err := cli.Get(ctx, "/demo/config/db")
	if err != nil {
		return nil
	}

	for _, kv := range resp.Kvs {
		log.Printf("%s -> %s", kv.GetKey(), kv.GetValue())
	}

	// Get with prefix
	resp, err = cli.Get(ctx, "/demo/config", clientv3.WithPrefix())
	if err != nil {
		return err
	}
	log.Println("\nPrefix Query:")

	for _, kv := range resp.Kvs {
		log.Printf("%s -> %s\n", kv.GetKey(), kv.GetValue())
	}

	// delete
	_, err = cli.Delete(ctx, "/demo/config/redis")
	if err != nil {
		return err
	}
	log.Println("\nRedis config deleted!")

	// Transaction: 类似Redis 的setnx
	txnKey := "/demo/txn/leader"
	txnResp, err := cli.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(txnKey), "=", 0)).
		Then(clientv3.OpPut(txnKey, "node-1")).
		Else(clientv3.OpGet(txnKey)).
		Commit()
	if err != nil {
		return nil
	}

	if txnResp.Succeeded {
		log.Println("Transaction: node-1 acquired leadership")
	} else {
		log.Println("Transaction: key already exists")
	}

	return nil
}

/*
2. Service Registration
*/
func register(
	ctx context.Context,
	cli *clientv3.Client,
	serviceName string,
	address string,
) error {
	log.Println("\n========== Service Register ==========")

	// Create lease: TTL = 15 seconds
	// unit: second
	leaseResp, err := cli.Grant(ctx, 15)
	if err != nil {
		return err
	}

	leaseID := leaseResp.ID // 租约唯一标识
	// 将租约绑定到某一个kv
	key := fmt.Sprint(serviceRoot, serviceName, "/", address)
	ins := &ServiceInstance{
		ID:      address,
		Address: address,
	}

	val, err := jsoniter.MarshalToString(ins)
	if err != nil {
		return err
	}
	_, err = cli.Put(
		ctx,
		key,
		val,
		clientv3.WithLease(leaseID),
	)
	if err != nil { // 撤销租约
		revokeCtx, cancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancel()
		_, _ = cli.Revoke(revokeCtx, leaseID)
		return err
	}

	log.Printf("Registered: %s\n", key)
	log.Printf("Lease ID: %x\n", leaseID)

	// KeepAlive renews the Lease continuously.
	keepAliveCh, err := cli.KeepAlive(ctx, leaseID)
	if err != nil {
		return err
	}
	log.Println("KeepAlive started. Press Ctrl+C to stop.")

	defer func() {
		revokeCtx, cancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancel()

		_, err := cli.Revoke(revokeCtx, leaseID)
		if err != nil {
			log.Printf("Revoke failed: %v\n", err)
		} else {
			log.Println("Lease revoked. Service deregistered.")
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case resp, ok := <-keepAliveCh:
			if !ok {
				if ctx.Err() != nil {
					return nil // context主动取消导致退出
				}
				return errors.New("keepalive channel closed")
			}

			if resp == nil || resp.TTL <= 0 {
				return errors.New("lease expired")
			}
			log.Printf(
				"KeepAlive: lease=%x, TTL=%ds\n",
				resp.ID,
				resp.TTL,
			)
		}
	}
}

func main() {
	t := &testing.T{}
	if len(os.Args) < 2 {
		log.Println("Usage:")
		log.Println("  go run . kv")
		log.Println("  go run . register <service> <address>")
		return
	}

	err := godotenv.Load(".env")
	if err != nil {
		log.Printf("load env variables failed: %v\n", err)
	}

	etcdEndpoint = os.Getenv("etcdEndpoint")
	serviceRoot = os.Getenv("serviceRoot")
	assert.NotEmpty(t, etcdEndpoint)
	assert.NotEmpty(t, serviceRoot)
	log.Printf("etcdEndpoint: %s, serviceRoot: %s", etcdEndpoint, serviceRoot)

	cli, err := newClient()
	if err != nil {
		log.Printf("Connect etcd failed: %v\n", err)
		os.Exit(1)
	}
	defer cli.Close()

	var runErr error

	switch os.Args[1] {
	case "kv":
		runErr = basicKV(cli)
	case "register":
		if len(os.Args) != 4 {
			log.Println("Usage: go run . register <service> <address>")
			os.Exit(1)
		}

		ctx, stop := signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)
		defer stop()
		runErr = register(
			ctx,
			cli,
			os.Args[2],
			os.Args[3],
		)
	default:
		log.Println("Unknown command:", strings.TrimSpace(os.Args[1]))
	}
	if runErr != nil {
		log.Printf("Error: %v\n", runErr)
		os.Exit(1)
	}
}
