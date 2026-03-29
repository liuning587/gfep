# GFEP 配置文件说明（`gfep.json`）

本文档说明主配置文件 **JSON** 的字段含义、默认值、加载方式，以及与 **TCP 空闲踢线**、**Web 控制台** 的关系。实现以 `utils/globalobj.go` 中的 `GlobalObj` 及 `Reload()` 为准。

---

## 1. 文件位置与格式

| 项目 | 说明 |
|------|------|
| **默认路径** | 进程工作目录下的 `conf/gfep.json`（`GlobalObject.ConfFilePath`，在 `init()` 里设为 `{cwd}/conf/gfep.json`）。 |
| **格式** | 标准 JSON（UTF-8）。字段名与 Go 结构体 **导出字段名** 一致，区分大小写（如 `LogWebEnabled`）。 |
| **注释** | JSON 不支持注释；勿在文件内写 `//` 或 `/* */`。 |
| **不存在时** | 若路径上无文件，`Reload()` 直接返回，保留内存中 **内置默认值**（见第 4 节）。 |

---

## 2. 何时加载

1. **进程启动**：`init()` 中构造 `GlobalObject` 后立刻调用一次 `Reload()`。
2. **Web 热更新**：管理员在控制台 **保存部分配置**（`PUT /api/config`）或 **调整日志级别**（`PUT /api/log-level`）写回磁盘后，会再次执行 `Reload()`。

`Reload()` 会重新读取整个 JSON 并 `json.Unmarshal` 到 `GlobalObj`；随后根据 `LogFile` / `LogDebugClose` 刷新日志后端。

---

## 3. `Reload()` 对部分键的「保留」逻辑

若 JSON 里 **没有** 下列键，则从 **本次 Reload 前的内存值** 拷贝回来（避免缺省键把字段冲成零值）：

| 键名 | 说明 |
|------|------|
| `FirstFrameTimeoutMin` | 见 §5.2 |
| `PostLoginRxIdleMinutes` | 见 §5.2 |
| `LogLinkLayer` | 缺省时保留旧值 |
| `LogWebSessionIdleMin` | Web 会话空闲分钟数；缺省保留旧值 |
| `LogWebSessionCookie` | 自定义会话 Cookie 名；缺省保留旧值 |

其余字段以本次解析结果为准（未出现在 JSON 中的字段，在 Go 的 `Unmarshal` 规则下多为类型零值，除非上面列表单独处理）。

---

## 4. 内置默认值（配置文件缺失或未含某键时）

以下为 `utils/globalobj.go` 中 `init()` 写入内存的初始值；若 `gfep.json` 不存在，运行时使用这些值（仅经过 §3 所列键不会「意外变零」的情况以代码为准）。

| 字段 | 默认值 |
|------|--------|
| `Name` | `gfep` |
| `Version` | `V0.3` |
| `Host` | `0.0.0.0` |
| `TCPPort` | `20083` |
| `TCPNetwork` | `tcp` |
| `BridgeHost698` | `""`（空表示不按桥接地址启用） |
| `MaxConn` | `50000` |
| `MaxPacketSize` | `2200` |
| `WorkerPoolSize` | `256` |
| `MaxWorkerTaskLen` | `1024` |
| `MaxMsgChanLen` | `8` |
| `LogDir` | `{cwd}/log` |
| `LogFile` | `""`（空则主要日志走 stderr 等默认行为） |
| `LogWebEnabled` | `false` |
| `LogWebHost` | `0.0.0.0` |
| `LogWebPort` | `20084` |
| `LogWebSessionIdleMin` | `0`（Web 侧另有「≤0 时用 480 分钟」等逻辑，见 `web` 包） |
| `LogWebSessionCookie` | `""` |
| `LogDebugClose` | `false` |
| `LogConnTrace` | `false` |
| `LogNetVerbose` | `false` |
| `LogPacketHex` | `false` |
| `LogLinkLayer` | `true` |
| `LogForwardEgressHex` | `false` |
| `ForwardWorkers` | `32` |
| `ForwardQueueLen` | `16384` |
| `FirstFrameTimeoutMin` | `1` |
| `PostLoginRxIdleMinutes` | `10` |
| `Timeout` | `30` |
| `SupportCompress` | `false` |
| `SupportCas` | `false` |
| `SupportCasLink` | `false` |
| `SupportCommTermianl` | `true` |
| `SupportReplyHeart` | `true` |
| `SupportReplyReport` | `false` |

---

## 5. 字段详解（按类别）

### 5.1 监听与连接

| 键 | 类型 | 说明 |
|----|------|------|
| `Name` | string | 服务名称（展示/标识用）。 |
| `Host` | string | TCP 监听 IP。IPv6 勿随意加方括号（与 `net.JoinHostPort` 等用法一致）。 |
| `TCPPort` | int | TCP 监听端口。 |
| `TCPNetwork` | string | `tcp`（推荐）、`tcp4`、`tcp6`；空一般按 `tcp` 处理。 |
| `MaxConn` | int | 最大连接数（框架侧限制，具体以 `znet` 为准）。 |
| `MaxPacketSize` | uint32 | 单包最大长度相关上限（与帧长、缓冲配置一致）。 |
| `BridgeHost698` | string | 698 桥接对端；空或首字符为 `0` 时，业务侧可按「未启用」处理（Web 总览等会据此显示）。实际桥接逻辑见 `fep/gfep_ptl.go` 等。 |

### 5.2 Worker 与超时（含空闲踢线）

| 键 | 类型 | 说明 |
|----|------|------|
| `WorkerPoolSize` | uint32 | 业务 Worker 池大小。 |
| `MaxWorkerTaskLen` | uint32 | 每 Worker 任务队列长度上限。 |
| `MaxMsgChanLen` | uint32 | `SendBuffMsg` 等发送缓冲相关长度。 |
| `ForwardWorkers` | int | 异步转发 worker 数量；`≤0` 时实现里会用默认（如 32）。 |
| `ForwardQueueLen` | int | 转发队列长度；`≤0` 时实现里会用默认（如 16384）。 |
| `FirstFrameTimeoutMin` | int | **分钟**。TCP 建立后若 **始终收不到完整规约帧**（`RxFrames == 0`），超过该时间则断开。`0` 表示关闭此项检测。实现：`znet/server.go` `runRxIdleSweep`。 |
| `PostLoginRxIdleMinutes` | int | **分钟**。**登录成功后**，自 **最近一次收到完整规约帧** 起算（若从未收过帧则用登录时间），超过该时间仍无收包则断开。`0` 关闭。实现：同上。 |
| `Timeout` | int | **分钟**。注释为「TCP 连接超时」。当前实现中 **`znet` 空闲扫包不使用该字段**；仅写入 `gfep.json`、在 **Web 控制台「有效配置」** 中展示，并属于 **管理员可在线 PATCH 的键**之一。若需与「登录后无流量踢线」统一，应在代码中显式把 `idleTO` 与 `Timeout` 绑定（见仓库演进或自行修改）。 |

**扫包周期**：`runRxIdleSweep` 使用 **约 30 秒** 的定时器轮询连接，因此实际断开时刻会在「阈值 + 最多约一个周期」附近。

### 5.3 日志

| 键 | 类型 | 说明 |
|----|------|------|
| `LogDir` | string | 日志目录。 |
| `LogFile` | string | 非空时，`Reload()` 后会配置 `zlog` 写文件（见 `zlog.SetLogFile`）。 |
| `LogDebugClose` | bool | `true` 时关闭 Debug 级别（与 `zlog` 一致）；Web 改配置后会 `Reload` 并切换。 |
| `LogConnTrace` | bool | 是否打印连接 Accept/Add/Remove 等跟踪（高并发建议关）。 |
| `LogNetVerbose` | bool | Worker 启动、路由注册等框架详细日志。 |
| `LogPacketHex` | bool | 是否对报文打十六进制（高 QPS 建议关）。 |
| `LogLinkLayer` | bool | `false` 时不打链路层 LINK（登录/心跳/登出/Online 等）及对应 hex；**不影响** FORWARD/REPORT 等其它类日志。 |
| `LogForwardEgressHex` | bool | `true` 时转发出口方向额外 hex 日志（流量大时慎用）。 |

### 5.4 Web 管理控制台

| 键 | 类型 | 说明 |
|----|------|------|
| `LogWebEnabled` | bool | 是否启用内嵌 HTTP 控制台（API + 静态 UI + 日志列举/下载等）。**勿对公网裸奔**。 |
| `LogWebHost` | string | Web 监听地址；空时常按 `0.0.0.0`。 |
| `LogWebPort` | int | Web 端口；`≤0` 时常用默认 `20084`。 |
| `LogWebSessionIdleMin` | int | 会话空闲超时（分钟）；`≤0` 时 Web 包内另有默认（如 480 分钟），见 `web/auth.go`。 |
| `LogWebSessionCookie` | string | 非空则会话 Cookie 使用该名称；空则自动生成 `gfep_session_<端口>`。 |

Web 用户、黑名单等 **不在** `gfep.json` 内：用户见 `conf/web_users.json`，终端黑名单见 `conf/terminal_blacklist.json`（路径与 `ConfFilePath` 同目录策略一致，以代码为准）。

### 5.5 规约 / 南网等业务开关

| 键 | 类型 | 说明 |
|----|------|------|
| `SupportCompress` | bool | 是否支持压缩（南网相关）。 |
| `SupportCas` | bool | 是否级联。 |
| `SupportCasLink` | bool | 级联终端登录、心跳等。 |
| `SupportCommTermianl` | bool | 是否允许终端重复登录等（字段名拼写历史保留）。 |
| `SupportReplyHeart` | bool | 前置机是否维护/回复心跳（如 `fep/gfep_ptl.go` 中判断）。 |
| `SupportReplyReport` | bool | 前置机是否确认上报等。 |

### 5.6 运行时注入（勿写入 JSON）

| 字段 | 说明 |
|------|------|
| `TCPServer` | 运行时由 `znet` 设置，**不要**出现在 `gfep.json`。 |
| `ConfFilePath` | 默认由程序设置；若在 JSON 里出现可能被反序列化覆盖，一般 **不建议** 手写。 |

---

## 6. Web 控制台可在线修改的键

管理员 **`PUT /api/config`** 时，仅下列键会写回 `gfep.json`（其余键即使出现在请求体也会被忽略）：

`LogPacketHex`、`LogLinkLayer`、`LogForwardEgressHex`、`LogDebugClose`、`LogConnTrace`、`LogNetVerbose`、`Timeout`、`FirstFrameTimeoutMin`、`PostLoginRxIdleMinutes`、`ForwardWorkers`、`ForwardQueueLen`

写入成功后会 `GlobalObject.Reload()` 并按 `LogDebugClose` 切换 `zlog`。

---

## 7. 示例片段（与仓库 `conf/gfep.json` 一致思路）

最小思路：**监听**、**日志目录**、**空闲踢线**、**Web** 按需组合。示例（字段可增减）：

```json
{
  "Host": "0.0.0.0",
  "TCPPort": 20083,
  "TCPNetwork": "tcp",
  "LogDir": "./log",
  "LogFile": "gfep.log",
  "LogWebEnabled": true,
  "LogWebHost": "0.0.0.0",
  "LogWebPort": 20084,
  "FirstFrameTimeoutMin": 1,
  "PostLoginRxIdleMinutes": 10,
  "Timeout": 30
}
```

---

## 8. 相关文档与代码索引

| 内容 | 位置 |
|------|------|
| 结构体定义与默认值 | `utils/globalobj.go` |
| 空闲踢线 | `znet/server.go` → `runRxIdleSweep` |
| Web 展示脱敏 | `web/config_redact.go` |
| Web 可写键 | `web/http.go` → `configWritableKeys` |
| HTTP 控制台行为 | `docs/WEB_SPEC.md` |

修改配置后若行为不符合预期，请确认已 **保存 JSON** 且 **已 Reload**（重启进程或 Web 保存触发）。
