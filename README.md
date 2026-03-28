# gfep

**gfep**（Golang Front-End Processor）是基于 Go 实现的电力采集**前置机 / 网关**服务，在单进程内通过 TCP 接入多种终端规约，完成报文解析、路由与转发，适用于 **DL/T 698.45**、**Q/GDW 1376.1** 及 **南网（NW）** 等相关场景。

---

## 目录

- [功能特性](#功能特性)
- [环境要求](#环境要求)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [构建与交叉编译](#构建与交叉编译)
- [运行与运维](#运行与运维)
- [仓库结构](#仓库结构)
- [文档](#文档)
- [贡献](#贡献)
- [许可证](#许可证)

---

## 功能特性

- **多规约 TCP 接入**：同一监听端口上按报文类型分发至 698.45、376.1、NW 等处理链路。
- **连接与注册表管理**：终端 / 应用侧连接状态维护，断线时清理桥接与注册信息。
- **698 桥接**：可选 `BridgeHost698` 与终端侧 698 通道配合（见配置项说明）。
- **异步转发池**：可配置 `ForwardWorkers`、`ForwardQueueLen`，用于高并发下的下行转发。
- **分模块滚动日志**：376 / 698 / NW 分文件输出，支持按目录归档（见 `timewriter`）。
- **可选交互菜单**：非 Linux 环境下可在控制台查看在线列表、版本等（见 `fep/app.go` 中 `usrInput`）。

---

## 环境要求

| 项目 | 说明 |
|------|------|
| Go | **1.21+**（见 `go.mod`） |
| 操作系统 | Windows / Linux 等 Go 支持的平台；Linux 下默认不启动交互菜单 |

---

## 快速开始

```bash
git clone https://github.com/liuning587/gfep.git
cd gfep
```

1. 编辑 **`conf/gfep.json`**（至少确认 `Host`、`TCPPort`、`LogDir` 等）。
2. 在工作目录下启动（需能读到 `conf/gfep.json`，默认路径为当前工作目录下的 `conf/gfep.json`）：

```bash
go run .
# 或
go build -o gfep .
./gfep
```

服务默认监听 **`0.0.0.0:20083`**（以配置文件为准）。

---

## 配置说明

配置文件路径由程序在启动时解析为 **`<工作目录>/conf/gfep.json`**（见 `utils/globalobj.go`）。

| 配置项 | 说明 |
|--------|------|
| `Name` | 服务名称 |
| `Host` / `TCPPort` | TCP 监听地址与端口 |
| `BridgeHost698` | 698 桥接对端地址（空则按业务默认处理） |
| `MaxConn` | 最大连接数 |
| `WorkerPoolSize` | 业务 Worker 池大小 |
| `ForwardWorkers` / `ForwardQueueLen` | 转发 worker 数与队列长度 |
| `LogDir` / `LogFile` | 日志目录与可选单文件日志 |
| `LogDebugClose` | 是否关闭 Debug 级别 |
| `LogConnTrace` | 是否打印连接 Accept/Add/Remove 等跟踪（高并发建议关闭） |
| `LogNetVerbose` | 是否打印 Worker 启动、路由注册、`msgBuffChan` 关闭等框架详细日志（默认关闭） |
| `LogPacketHex` | 是否对报文打十六进制日志（高 QPS 建议关闭） |
| `LogLinkLayer` | 是否打印链路层（LINK：登录/心跳/登出/Online）相关文本与对应 hex；`false` 时仍打印 FORWARD/REPORT 等（默认 `true`） |
| `Timeout` | TCP 超时（分钟） |
| `SupportCompress` / `SupportCas` / `SupportCasLink` | 南网相关：压缩、级联、级联终端登录与心跳 |
| `SupportCommTermianl` | 是否允许终端重复登录等行为 |
| `SupportReplyHeart` / `SupportReplyReport` | 前置机维护心跳、确认上报 |

未提供配置文件时，程序使用 `utils/globalobj.go` 中的内置默认值，并在配置文件存在时 **`Reload` 覆盖**。

---

## 构建与交叉编译

在已安装 Go 的机器上，于项目根目录执行：

```bash
go build -ldflags "-s -w" -o gfep .
```

仓库根目录提供了 Windows 批处理示例，用于交叉编译 Linux 二进制：

| 脚本 | 说明 |
|------|------|
| `build_linux.bat` | `GOOS=linux` `GOARCH=amd64` |
| `build_i386.bat` | Windows 386 |
| `build_i386_linux.bat` | Linux 386 |

可在脚本中取消注释 `-ldflags "-s -w"` 以减小体积。

---

## 运行与运维

- **日志**：默认写入 `LogDir` 下按规约区分的滚动日志（如 `376`、`698`、`nw` 模块名）。业务路径统一经 `internal/logx` 输出到 stderr（前缀 `gfep:`），与 `zlog` 文件日志并存时请按运维策略分流与留存。
- **性能**：生产环境建议关闭 `LogConnTrace`、`LogNetVerbose`、`LogPacketHex`，或保留 `LogPacketHex` 仅排障转发时配合关闭 `LogLinkLayer`，并按并发调整 `WorkerPoolSize` 与转发池参数。
- **敏感与合规**：`LogPacketHex` 与桥接侧详细日志会输出报文十六进制，仅应在排障时由授权人员短时开启，并注意日志留存与脱敏策略。
- **转发队列**：队列满时会丢弃任务并打 `[WARN]`；累计丢弃次数在 expvar `gfep_forward_queue_drops`（若进程暴露 `/debug/vars` 可查看）。
- **版本号**：当前内置版本字符串见 `utils.GlobalObject.Version`（如 `V0.2`）。

---

## 仓库结构

```
gfep/
├── conf/           # 默认配置文件 gfep.json
├── internal/logx/  # 统一 stderr 日志前缀封装
├── znet/           # TCP 服务、连接与消息调度
├── ziface/         # 接口定义
├── zptl/           # 698 / 376 / NW 等规约解析与校验
├── bridge/         # 698 桥接相关
├── timewriter/     # 按时间的日志写入
├── utils/          # 全局配置与工具
├── test/           # 测试与辅助程序（如 gterminal 698 模拟终端、stress 连接压测）
├── docs/           # 设计 / 规格文档
├── fep/            # 前置机业务包：入口逻辑、规约 profile、注册表、转发池等
├── gfep.go         # 程序入口（调用 `fep.Main()`）
└── go.mod
```

---

## 文档

- 需求规格（草案）：[MQTT 集成](docs/MQTT_SPEC.md) · [Web 日志下载](docs/WEB_SPEC.md) · [索引](docs/MQTT_WEB_SPEC.md)
- 698 模拟终端（联调 / 轻量压测）：[test/gterminal/README.md](test/gterminal/README.md)

---

## 贡献

欢迎通过 Issue / Pull Request 反馈问题或提交改进。提交前请确保：

- `go test ./...` 可通过（如有测试覆盖的模块）；
- 变更与现有代码风格、配置字段命名保持一致。

---

## 许可证

本仓库根目录若未包含 `LICENSE` 文件，使用前请与维护方确认授权方式；若你计划对外开源，建议补充明确的许可证文件并在本段替换为对应说明。
