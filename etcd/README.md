# etcd 学习笔记：clientv3 的 KV、服务注册、服务发现与分布式锁

`etcd/demo` 用一个 `main.go`（527 行）把 etcd 日常最常打交道的四类用法串起来：

| # | 功能 | 入口函数 | 子命令 |
| --- | --- | --- | --- |
| 1 | 基础 KV 与事务 | `basicKV`（`main.go:43`） | `kv` |
| 2 | 服务注册（租约 + KeepAlive） | `register`（`main.go:111`） | `register <service> <address>` |
| 3 | 服务发现（快照 + Watch） | `discovery`（`main.go:242`） | `discover <service>` |
| 4 | 分布式锁（Session + Mutex） | `distributedLock`（`main.go:409`） | `lock` |

模块只演示**客户端**，etcd 服务端要自己先起。四个功能都挂在同一个二进制的子命令上，便于分别观察。

依赖（`etcd/demo/go.mod`）：

| 依赖 | 用途 |
| --- | --- |
| `go.etcd.io/etcd/client/v3 v3.7.3` | etcd 客户端：KV、租约、Watch、事务 |
| `go.etcd.io/etcd/client/v3/concurrency` | 基于租约的 Session / Mutex |
| `github.com/joho/godotenv v1.5.1` | 从 `.env` 读连接配置 |
| `github.com/json-iterator/go v1.1.12` | 服务实例的 JSON 序列化 |
| `github.com/stretchr/testify v1.12.1` | 配置项非空校验 |

## 目录结构

```text
etcd/
├── README.md          # 本文
└── demo/
    ├── .env           # etcdEndpoint / serviceRoot（本地配置，不入库）
    ├── go.mod         # module golearn/etcd
    ├── go.sum
    └── main.go        # 四个子命令：kv / register / discover / lock
```

## 运行前：起 etcd、准备 .env

`etcd/demo/.env` 内容：

```dotenv
etcdEndpoint=127.0.0.1:2379
serviceRoot=/demo/services/
```

`serviceRoot` 末尾带 `/`：后面的 key 是 `serviceRoot + serviceName + "/" + address`（`main.go:128`、`main.go:207`）。

```shell
cd etcd/demo

go run . kv
go run . register user-service 10.0.0.1:8081
go run . discover user-service
go run . lock
```

> `godotenv.Load(".env")` 用的是**相对当前工作目录**的路径（`main.go:463`），所以要先 `cd etcd/demo`。在仓库根直接 `go run ./etcd/demo kv` 会读不到 `.env`，连接配置为空。

`main` 的结构（`main.go:454-527`）：解析子命令参数 → 加载 `.env` → `newClient`（`DialTimeout: 5s`）→ `switch` 分派 → 统一打印错误并以非零码退出。

## revision 速查

四个功能都绕不开 revision，先在这里集中说清。revision 是 etcd **整个集群**的写事务流水号，不按 key 计：

| 字段 | 含义 |
| --- | --- |
| `resp.Header.Revision` | 当前全局 revision（最近一次写事务的编号） |
| `kv.CreateRevision` | 该 key **被创建**时的事务号；key 不存在时比较值为 0 |
| `kv.ModRevision` | 该 key **最后一次被修改**时的事务号 |
| `kv.Version` | 该 key 自创建以来被修改的次数；**删除后重建从 1 重新计** |

- 每次 `Put` / `Delete` / `Txn` 提交都会领到一个新号；同一个事务里的多个操作共用同一个号。
- 删除也消耗 revision，所以「删除后重建」的 key 的 `CreateRevision` 会变大。
- 判断「key 不存在」用 `clientv3.Compare(clientv3.Version(key), "=", 0)`：不存在的 key，`Version` 的比较值是 0。

## 1. Basic KV：Put / Get / 前缀 / Delete / 事务

### 代码做了什么

`basicKV`（`main.go:43-106`）在 10s 超时的 ctx 里依次：

1. `Put /demo/config/db = mysql`、`Put /demo/config/redis = redis`；
2. `Get` 单 key 并打印；
3. `Get` + `clientv3.WithPrefix()` 查整个 `/demo/config` 前缀；
4. `Delete /demo/config/redis`；
5. 用事务做一次「setnx 式选主」。

### 事务 = 服务端的 CAS

```go
txnResp, err := cli.Txn(ctx).
    If(clientv3.Compare(clientv3.Version(txnKey), "=", 0)).
    Then(clientv3.OpPut(txnKey, "node-1")).
    Else(clientv3.OpGet(txnKey)).
    Commit()
```

`If` 是前置条件，`Then` / `Else` 是两组操作，`Commit` 把「比较 + 执行」作为一个原子事务提交：

- 条件成立（key 不存在）→ 执行 `Then`，`txnResp.Succeeded == true`；
- 条件不成立（已有人写过）→ 执行 `Else`，把现有值读回来，`Succeeded == false`。

### 关键点

- **条件不成立不是错误**：`err == nil` 只代表**事务提交成功**，抢没抢到要看 `Succeeded`。这是 etcd 事务与「出错就返回 error」的普通 API 最大的区别。
- **前缀是字节前缀**：`"/demo/config"` 会匹配 `/demo/config/db`，也会匹配 `/demo/configX`。要精确匹配一层目录，前缀末尾带上 `"/"`。
- **写都走同一个全局 revision**：`Delete` 之后 `Version` 归零，但 key 下次重建会拿到新的 `CreateRevision`。

## 2. 服务注册：租约（Lease）+ KeepAlive

### 流程

`register`（`main.go:111-201`）：

1. `Grant(ctx, 15)` 申请一个 **TTL = 15 秒**的租约；
2. 拼 key：`serviceRoot + serviceName + "/" + address`（如 `/demo/services/user-service/10.0.0.1:8081`）；
3. 用 jsoniter 把 `ServiceInstance{ID, Address}` 序列化成 value；
4. `Put(key, val, clientv3.WithLease(leaseID))` —— **把 key 绑定到租约**；
5. `KeepAlive(ctx, leaseID)` 返回一个续约结果 channel，后台自动续约；
6. `for { select { ... } }` 阻塞住进程，直到 Ctrl+C 或续约失败。

### 关键代码

```go
leaseResp, err := cli.Grant(ctx, 15)
leaseID := leaseResp.ID

_, err = cli.Put(ctx, key, val, clientv3.WithLease(leaseID))
if err != nil {
    // 写失败：撤销刚申请的租约，避免垃圾残留
    revokeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    _, _ = cli.Revoke(revokeCtx, leaseID)
    return err
}

keepAliveCh, err := cli.KeepAlive(ctx, leaseID)
```

### 关键点

- **租约就是生命周期**：key 绑定租约后，租约到期（或主动 `Revoke`）key 自动删除，不需要人工清理。服务实例「下线即消失」靠的就是这个。
- **只有 `WithLease` 的 Put 才会跟着租约走**；普通 Put 写进去的 key 永久存在（第 3 节的样例数据就是如此）。
- **KeepAlive 是续约**：客户端在 TTL 内不断把租约往后推；进程崩溃或网络断开 → 停止续约 → TTL 到期 → key 被删。
- **退出路径有两条**：正常 Ctrl+C 时 `ctx.Done()` 触发 `return nil`，随后 `defer` 里的 `Revoke` 主动注销；异常时租约自然过期。
- 续约 channel 的三种结果（`main.go:183-198`）：
  - `!ok` 且 `ctx` 已取消：正常退出；
  - `!ok` 且 `ctx` 未取消：keepalive 通道被服务端关闭（连接断、租约丢失）→ 返回错误；
  - `resp == nil || resp.TTL <= 0`：租约已过期 → 返回错误。
- key 里带 `address`，同一服务的多个实例各占一个 key，天然支持多实例注册。
- 注册成功后进程要**一直活着**才能保持续约；demo 用阻塞循环实现，生产里通常把 `KeepAlive` 放进服务生命周期管理。

## 3. 服务发现：快照 + Watch 增量

### 3.1 先灌样例：一次性事务守卫

`putSamples`（`main.go:203-237`）把三个样例实例打包进一个事务：

```go
txnResp, err := cli.Txn(ctx).
    If(clientv3.Compare(clientv3.Version(txnKey), "=", 0)).
    Then(ops...).
    Commit()
```

- `ops` 是三条 `OpPut`（每个实例一条），追加一条 `OpPut(txnKey, "init-node")`；
- 守卫 key `/demo/txn/leader` 一旦写入，第二次运行条件就不成立，`Succeeded == false`，样例数据不会重复灌；
- 同一个事务里的多条写**要么全成功要么全失败**，不会出现「灌了一半」的快照。

> 样例数据是普通 Put（没绑租约），所以会一直留在 etcd 里。生产中的初始化数据一般交给独立 job，或写成幂等操作，不要靠这种守卫 key。

### 3.2 先取快照

```go
getCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
resp, err := cli.Get(getCtx, prefix, clientv3.WithPrefix())
cancel()

for _, kv := range resp.Kvs {
    var instance = new(ServiceInstance)
    if err := jsoniter.Unmarshal(kv.GetValue(), instance); err != nil {
        continue
    }
    instances[string(kv.GetKey())] = instance
}
```

把前缀下所有 key 一次性拉回来，在内存里建 `map[key]*ServiceInstance`。反序列化失败的 key 跳过而不是整体失败——发现端要尽量容错。

### 3.3 从快照的下一个 revision 开始 Watch

```go
watchCh := cli.Watch(
    ctx,
    prefix,
    clientv3.WithPrefix(),
    clientv3.WithRev(resp.Header.Revision+1),
)
```

快照对应 revision `R`，`R` 之前的状态已经在 map 里了，所以从 `R+1` 开始收增量：

- 从 `R+1` 开始：只收快照之后发生的事件，不漏、不重；
- 从 `R` 开始：会收到 revision `R` 当次的写事件，和快照内容重复（幂等场景无所谓，但没必要）；
- 不指定 `WithRev`：从**当前** revision 开始，**快照与 watch 之间若发生写入就会丢事件**。

`WithRev` 用的是**全局 revision**，不是 key 的 revision。

### 3.4 应用事件

```go
for _, event := range watchResp.Events {
    key := string(event.Kv.GetKey())
    switch event.Type {
    case clientv3.EventTypePut:
        // 反序列化后 instances[key] = instance
    case clientv3.EventTypeDelete:
        delete(instances, key)
    }
}
```

- `PUT`：新增或覆盖（key 相同就是实例信息更新）；
- `DELETE`：从 map 删除（租约过期、主动注销都会产生 DELETE 事件）；
- 删除事件的 `event.Kv` 只有 key 和 revision，没有 value。

### 3.5 断链自愈

watch 不是永久的，`main.go:329-344` 处理三种异常：

| 情况 | 处理 |
| --- | --- |
| `watchResp.Err() != nil` 且 `CompactRevision > 0` | 指定的 revision 已被 etcd 压缩，无法从历史继续 → 打印 `Revision compacted. Resync required.`，跳出重新取快照 |
| 其它 watch 错误 | 同样 `restart = true`，退出内层循环重新快照 |
| watch channel 意外关闭 | 打印 `Watch channel closed. Resyncing...`，等待 1s 后重新快照 |

revision 压缩是 etcd 的常规行为（自动压缩保留窗口），所以发现端**必须**能接受「watch 断了就重新全量快照」，**不能假设 watch 一定能从断点续上**。

### 关键点

- **快照 + 增量**：先 `Get` 再 `Watch(rev+1)`，两步之间没有窗口，最终一致。
- 本地 map 只是缓存，etcd 才是事实来源；重启进程重新快照即可恢复。
- `Get` 和 `Watch` 都传了可取消的 ctx，Ctrl+C 时 `ctx.Done()` 让循环干净退出。
- `printInstances`（`main.go:395-404`）对地址排序后打印，输出稳定、便于对比。
- 生产实现还要处理「同一实例换地址」「实例健康检查」「本地缓存 TTL」等问题，demo 只演示最核心的快照 + watch 模型。

## 4. 分布式锁：Session + Mutex

### 用法

```go
session, err := concurrency.NewSession(cli, concurrency.WithTTL(10))
defer session.Close()

mutex := concurrency.NewMutex(session, "/locks/order-service")

ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
defer cancel()

if err := mutex.Lock(ctx); err != nil { ... }
// 临界区
time.Sleep(5 * time.Second)

unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 3*time.Second)
defer unlockCancel()
mutex.Unlock(unlockCtx)
```

- `NewSession` 申请一个 TTL=10s 的租约，并在后台持续续约；
- `NewMutex` 只是记住锁前缀，不产生任何 etcd 写；
- `Lock` 真正参与排队；
- `Unlock` 删除自己的占位 key；`session.Close()` 撤销租约作为兜底。

三个时长各管各的，别混：

| 时长 | 代码位置 | 管什么 | 到点后果 |
| --- | --- | --- | --- |
| TTL = 10s | `concurrency.WithTTL(10)` | 租约多久没人续约就过期 | key 被删、锁自动释放；**续约正常时租约可以一直续下去** |
| 等锁 ctx = 20s | `context.WithTimeout(..., 20s)` | `Lock` 排队最多等多久 | `Lock` 返回错误；**不影响已经拿到的锁** |
| 临界区 = 5s | `time.Sleep(5s)` | 业务执行时间 | 不会自动放锁；锁一直持有到 `Unlock` 或租约过期 |

### 背后发生了什么（`client/v3/concurrency` 的实现）

每把锁对应一个前缀，每个竞争者在这个前缀下**创建自己的 key**（key 名带自己的租约 ID），用 `CreateRevision` 排序形成先来后到的队列。抢锁是一次事务：

```text
Txn.
  If(CreateRevision(myKey) == 0).
  Then(Put(myKey, "", WithLease(lease)), Get(prefix, WithFirstCreate)).
  Else(Get(myKey), Get(owner)).
Commit()
```

- 条件成立：我还没有占位 key → 创建它，同时把队首（`WithFirstCreate`：按 create revision 升序的第一条）读回来；
- 条件不成立：我已经有 key（重试）→ 读回自己和当前队首。

然后判断：

| 情况 | 结果 |
| --- | --- |
| 队列为空，或队首的 `CreateRevision` 等于自己的 | 拿到锁 |
| 自己排在别人后面 | `waitDeletes(ctx, client, prefix, myRev-1)` 等前一位 |

`waitDeletes` 做两件事（`concurrency/key.go:49-63`）：

1. `Get(prefix, WithLastCreate, WithMaxCreateRev(myRev-1))` —— 取出「create revision ≤ 我前一号」的最后一个 key，也就是**排在我前面的那一位**；
2. `watch` 这个 key，等它的 DELETE 事件；前一位释放（或租约过期）后，回到循环重新尝试抢锁。

所以整个锁是**公平的 FIFO 队列**，不是随机抢。`Lock` 在真正拿到锁之后还会再 `Get` 一次自己的 key，确认 session 没有在中途过期（否则返回 `ErrSessionExpired`）。

### 关键点

- **锁靠租约续命**：持有者进程崩溃 → keepalive 停 → TTL 到期 → 自己的 key 被删 → 下一位收到 DELETE 被唤醒。不需要任何「清理死锁」的逻辑。
- **TTL 不是临界区超时**：Session 后台每 `TTL/3`（本例约 3.3s）续约一次（`client/v3/lease.go`：`nextKeepAlive = now + TTL/3`），所以续约正常时，临界区跑 30s、60s 都不会丢锁。
- **真正的风险是「续约停了」**：进程长时间卡顿（GC / 虚拟机挂起）、网络分区、etcd 不可用时，租约超过 TTL 没人续 → key 被删 → 锁自动易主；旧持有者如果还在跑，就出现两个「持有者」同时写。保守做法是让临界区显著短于 TTL，并监听 `session.Done()`（demo 没做，属于简化）。
- **根治靠 fencing token**：用拿锁响应里的 `Header().Revision`（或业务侧单调递增版本号）作为 token，让下游存储拒绝更旧的 token，防止「过期持有者」继续写；只靠「临界区 < TTL」是概率性安全。
- `Lock(ctx)` 的 ctx 只限制**等锁**的时间，不影响持锁；本例 20s 等锁、5s 临界区。
- `Unlock` 用独立的 3s ctx：原 ctx 可能已经超时或被取消，不能再用它发删除请求。
- 常见错误值：`ErrLockReleased`（重复 Unlock）、`ErrSessionExpired`（租约已丢）、`ErrLocked`（`TryLock` 非阻塞抢锁失败）。
- `TryLock` 是非阻塞版本：不是队首就删掉自己的 key 并返回 `ErrLocked`；`IsOwner` 可随时判断自己是否还持有；`Header()` 返回拿锁时的响应头（取 revision 做 fencing token）。

### 手动验证

两个终端分别：

```shell
cd etcd/demo
go run . lock        # 终端 A：立刻拿到锁，5s 后释放
go run . lock        # 终端 B：等 A 释放后才拿到
```

观察点：B 的 `Waiting for lock...` 会一直等到 A 打印 `Lock released.`；如果把 A 直接 `Ctrl+C` 杀掉，B 会在约 10s（Session TTL）后拿到锁。

## 和 etcdctl 对照

| demo | etcdctl 等价命令 |
| --- | --- |
| `go run . kv` | `etcdctl put /demo/config/db mysql`、`etcdctl get /demo/config --prefix`、`etcdctl del /demo/config/redis`、`etcdctl txn` |
| 事务选主 | `etcdctl txn -i`：`compare` 段写 `create("/demo/txn/leader") = "0"`，`success` 段写 `put /demo/txn/leader node-1` |
| `go run . register` | `etcdctl lease grant 15` + `etcdctl put <key> <val> --lease=<ID>` + `etcdctl lease keep-alive <ID>` |
| `go run . discover` | `etcdctl get <prefix> --prefix -w json` + `etcdctl watch <prefix> --prefix` |
| `go run . lock` | `etcdctl lock /locks/order-service <cmd>`（内部同样用 `concurrency.Mutex`） |

排查锁排队时可以直接看队列：

```shell
etcdctl get /locks/order-service/ --prefix -w json
# 按 create_revision 升序就是排队顺序
```

## 注意点 

| 坑 | 说明 |
| --- | --- |
| `.env` 找不到 | `godotenv.Load(".env")` 相对 cwd，必须先 `cd etcd/demo` |
| 用 `assert` 做运行时校验 | 代码里 `t := &testing.T{}` + `assert.NotEmpty` 只是教学取巧：断言失败不会中断进程，随后仍会用空配置去连接。生产代码用普通 `if` 判断并返回错误 |
| 把事务 `Succeeded == false` 当错误 | 条件不成立是正常结果，只有 `err != nil` 才是失败 |
| 前缀少写 `/` | `/demo/services/user` 会匹配 `user-service`、`user2`；层前缀要写成 `/demo/services/user/` |
| 以为删除不占 revision | 删除同样领新 revision；watch 起点、`CreateRevision` 都受影响 |
| Watch 从快照 revision 开始 | 会重复收到快照已包含的事件；正确起点是 `Header.Revision + 1` |
| 假设 Watch 永不中断 | revision 可能被压缩，必须能重取快照 |
| 把 TTL 当临界区上限 | `WithTTL(10)` 是**租约失效窗口**，不是业务超时；续约正常时临界区可超过 TTL。锁易主发生在续约中断超过 TTL 时，不是「临界区变长」 |
| 注册的 key 指望进程退出后自动删 | 只有绑了租约的 key 才会自动过期；普通 Put 永久存在 |

## 参考资料

- etcd 官方文档：<https://etcd.io/docs/>
- Go client v3 API：<https://pkg.go.dev/go.etcd.io/etcd/client/v3>
- `concurrency` 包（Session / Mutex 源码）：<https://pkg.go.dev/go.etcd.io/etcd/client/v3/concurrency>
- godotenv：<https://github.com/joho/godotenv>
