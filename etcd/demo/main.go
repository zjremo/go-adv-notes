package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/joho/godotenv"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
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
		return err
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
		return err
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

func putSamples(ctx context.Context, cli *clientv3.Client, samples []*ServiceInstance, serviceName string) error {
	if len(samples) == 0 {
		return nil
	}
	prefix := serviceRoot + serviceName + "/"

	txnKey := "/demo/txn/leader" // 直接造一个不存在的事务key，来一次交互完成批处理插入
	ops := make([]clientv3.Op, 0, len(samples))
	for _, sample := range samples {
		key := fmt.Sprint(prefix, sample.Address)
		value, err := jsoniter.MarshalToString(sample)
		if err != nil {
			return err
		}
		ops = append(ops, clientv3.OpPut(key, value))
	}
	// append txnLock
	ops = append(ops, clientv3.OpPut(txnKey, "init-node"))

	txnResp, err := cli.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(txnKey), "=", 0)).
		Then(ops...).
		Commit()
	if err != nil {
		return err
	}

	// succeeded is set to true if the compare evaluated to true or false otherwise.
	if txnResp.Succeeded {
		log.Println("Put samples successfully")
	} else {
		log.Println("Transaction key already exists!")
	}
	return nil
}

/*
3. Service Discovery
*/
func discovery(
	ctx context.Context,
	cli *clientv3.Client,
	serviceName string,
) error {
	// 先put一些数据作为样例
	samples := []*ServiceInstance{
		{
			ID:      "1",
			Address: "1",
		},
		{
			ID:      "2",
			Address: "2",
		},
		{
			ID:      "3",
			Address: "3",
		},
	}
	if err := putSamples(ctx, cli, samples, serviceName); err != nil {
		return err
	}

	log.Println("\n========== Service Discovery==========")

	prefix := serviceRoot + serviceName + "/"

	// Local service instance cache
	var instances map[string]*ServiceInstance

	for {
		if ctx.Err() != nil {
			return nil
		}

		// Step 1: Get current service instances
		getCtx, cancel := context.WithTimeout(
			ctx, 5*time.Second,
		)

		resp, err := cli.Get(
			getCtx,
			prefix,
			clientv3.WithPrefix(),
		)
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("Get failed: %v\n", err)
			time.Sleep(time.Second)
			continue
		}

		// use resp.KVs to build instances
		instances = make(map[string]*ServiceInstance)

		for _, kv := range resp.Kvs {
			var instance = new(ServiceInstance)
			if err := jsoniter.Unmarshal(kv.GetValue(), instance); err != nil {
				log.Printf("unmarshal kv failed, key=%s err=%v", string(kv.Key), err)
				continue
			}

			instances[string(kv.GetKey())] = instance
		}

		log.Printf(
			"Snapshot Revision: %d\n",
			resp.Header.Revision,
		)
		printInstances(instances)

		// Step 2: watch from snapshot revision + 1
		watchCh := cli.Watch(
			ctx,
			prefix,
			clientv3.WithPrefix(),
			clientv3.WithRev(resp.Header.Revision+1),
		)

		restart := false
		for watchResp := range watchCh {

			if watchResp.Err() != nil {
				log.Printf("Watch error: %v\n", watchResp.Err())
				if watchResp.CompactRevision > 0 {
					// 指定的revision已经被etcd压缩过，watch无法从这个历史版本再次开始
					log.Println("Revision compacted. Resync required.")
					restart = true
					break
				}

				if ctx.Err() != nil {
					return nil
				}

				restart = true
				break
			}

			// 接收watch事件，server端变更
			for _, event := range watchResp.Events {
				key := string(event.Kv.GetKey())
				switch event.Type {
				case clientv3.EventTypePut: // Put 事件
					// 解析出来value
					instance := new(ServiceInstance)
					if err := jsoniter.Unmarshal(event.Kv.GetValue(), instance); err != nil {
						log.Printf("unmarshal kv failed, key=%s err=%v", string(event.Kv.Key), err)
						continue
					}

					instances[key] = instance

					log.Printf(
						"[PUT] %s -> %s (Revision=%d)\n",
						key,
						instance.Address,
						event.Kv.GetModRevision(),
					)
				case clientv3.EventTypeDelete:
					delete(instances, key)

					log.Printf(
						"[DELETE] %s (Revision=%d)\n",
						key,
						event.Kv.GetModRevision(),
					)
				}
				printInstances(instances)
			}
		}

		if ctx.Err() != nil {
			return nil
		}

		if restart {
			log.Println("Restarting discovery watch...")
			time.Sleep(time.Second)
			continue
		}

		// Watch channel closed unexpectedly
		log.Println("Watch channel closed. Resyncing...")
		time.Sleep(time.Second)
	}
}

func printInstances(instances map[string]*ServiceInstance) {
	addresses := make([]string, 0, len(instances))

	for _, instance := range instances {
		addresses = append(addresses, instance.Address)
	}

	sort.Strings(addresses)
	log.Printf("Current instances: %v\n", addresses)
}

/*
4. Distributed Lock
*/
func distributedLock(cli *clientv3.Client) error {
	log.Println("\n========== Distributed Lock ==========")

	// Session creates a Lease and keeps it alive
	session, err := concurrency.NewSession(
		cli,
		concurrency.WithTTL(10),
	)
	if err != nil {
		return err
	}
	defer session.Close()

	mutex := concurrency.NewMutex(
		session,
		"/locks/order-service",
	)

	ctx, cancel := context.WithTimeout(
		context.Background(), 20*time.Second,
	)
	defer cancel()

	log.Println("Waiting for lock...")

	if err := mutex.Lock(ctx); err != nil {
		return err
	}

	log.Println("Lock acquired!")
	time.Sleep(5 * time.Second)

	log.Println("Critical section completed.")
	unlockCtx, unlockCancel := context.WithTimeout(
		context.Background(), 3*time.Second,
	)
	defer unlockCancel()

	if err := mutex.Unlock(unlockCtx); err != nil {
		return err
	}
	log.Println("Lock released.")
	return nil
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
	case "discover":
		if len(os.Args) != 3 {
			log.Println("Usage: go run . discover <service>")
			os.Exit(1)
		}

		ctx, stop := signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)
		defer stop()

		runErr = discovery(ctx, cli, os.Args[2])
	case "lock":
		runErr = distributedLock(cli)
	default:
		log.Println("Unknown command:", strings.TrimSpace(os.Args[1]))
	}
	if runErr != nil {
		log.Printf("Error: %v\n", runErr)
		os.Exit(1)
	}
}
