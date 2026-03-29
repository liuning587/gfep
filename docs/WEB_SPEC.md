# GFEP：Web 管理控制台 — 需求规格（SPEC）

| 项 | 内容 |
|---|------|
| 状态 | Draft |
| 版本 | 2.4 |
| 关联 | [MQTT 集成 SPEC](./MQTT_SPEC.md) |

---

## 1. 背景与目标

### 1.1 背景

GFEP 在运行过程中产生日志、维护终端连接与业务参数。运维与值班人员需要在 **受控网络环境** 内，通过浏览器完成 **状态查看、日志观测、配置与策略管理**，而不仅限于裸 HTTP 目录列举。

### 1.2 目标

在 **单一 HTTP(S) 监听** 上提供 **现代化 Web 控制台**（实现可选用 **SPA + API** 或服务端模板 + 少量 API），**沿用现有配置语义**：由 **`LogWebEnabled` / `LogWebHost` / `LogWebPort`**（见 `conf/gfep.json`）控制是否监听及地址；默认 **关闭** 以降低暴露面。关闭时 **不得** 监听该端口。

### 1.3 非目标（本期或另立 SPEC）

- 面向公网开放的 SaaS 级多租户、计费与复杂组织树权限。
- 任意路径文件上传、在线编辑磁盘上任意文件（除规格明确允许的 **配置/黑名单等受控落盘** 外）。
- 替代专业监控（Prometheus/Grafana）；本控制台提供 **内置轻量指标** 即可。

### 1.4 与历史能力的关系

- **v1.x**：仅对 `LogDir` 做静态列举与下载（见 §11 附录）。
- **v2.x**：在同一端口上升级为 **带鉴权与角色** 的管理界面；历史下载能力 **必须** 仍以安全方式保留（见 §3.6），可由 UI 调用或受控 API 实现。

### 1.5 控制台时间格式与 MQTT 的关系

- **Web API / 控制台 JSON** 中的时间字段（如 `loginTime`、`connTime`）当前实现为 **`Asia/Shanghai` 本地可读字符串**，格式 `2006-01-02 15:04:05`（秒精度），见 `web.FormatDisplayWeb`。
- **[MQTT_SPEC.md](./MQTT_SPEC.md)** 要求对 MQTT 载荷使用 **RFC3339 UTC（`Z`）**；二者 **场景不同**，**不** 要求 Web JSON 与 MQTT 字节级一致。若未来统一为 UTC `Z`，须升 SPEC 版本并改前端。

---

## 2. 用户与鉴权

### 2.1 账号类型（FR-WEB-010）

| 角色 | 能力概要 |
|------|-----------|
| **管理员（admin）** | 用户与角色管理、黑名单、日志级别、敏感配置查看/修改（以 §3 为准）、**终端踢下线**（§10.2），全量菜单 |
| **普通用户（user）** | 查看系统状态、**在线终端明细**、**APP/主站连接明细**、**698 桥接明细**（**仅** `BridgeHost698` 已配置时显示菜单）、实时/历史日志、**只读**配置与参数；**无** 黑名单/日志级别/用户管理（除非另行授权，默认关闭） |

- **必须** 支持至少 **上述两类** 账号；密码或凭据 **不得** 明文落盘（哈希 + 盐或等价方案，实现定稿）。
- **必须** 支持 **多用户同时登录**（独立会话）：会话与连接数上限、空闲超时策略由实现定稿并写入运维说明。

### 2.2 登录与会话（FR-WEB-011）

- **必须** 提供登录页；未登录访问受保护资源时 **401/302 至登录**（实现选一并文档化）。
- **宜** 支持 **HTTPS**（`web.tls.*` 或与 `LogWeb*` 并列的配置项，实现定稿）、**会话固定防护**、**登出**。
- **宜** 记录登录成功/失败审计（见 §8）。

### 2.3 初始账号（FR-WEB-012）

- 首次部署 **必须** 有明确方式创建或引导设置管理员（环境变量、首次启动向导、或随发行说明的配置文件），**禁止** 长期依赖硬编码默认密码。
- **当前实现**：若 **`conf/web_users.json`** 不存在且设置环境变量 **`GFEP_WEB_BOOTSTRAP_PASSWORD`**，启动时自动创建 **`GFEP_WEB_BOOTSTRAP_USER`**（默认 `admin`）管理员账号；**须在首次登录后删除该环境变量**（见启动日志提示）。

---

## 3. 功能需求（按模块）

### 3.1 系统状态总览（FR-WEB-020）

**必须** 在控制台中展示（刷新周期可配置，建议 1–5s 或 SSE/轮询）：

| 类别 | 内容 |
|------|------|
| **主机** | CPU 使用率、内存使用率、磁盘使用率（至少包含 `LogDir` 所在分区或实现选定分区） |
| **业务** | 在线终端数量 **按协议（规约）分组** 统计；可选展示总连接数、Worker 队列长度等（与 `utils.GlobalObject` / 连接管理可观测数据对齐） |

- 数据获取 **不得** 阻塞核心收发包路径（**宜** 在独立 goroutine 或带超时的查询中完成）。
- **当前实现（`/api/status`）额外字段**：`tcpConnTotal`、`terminalsByProtocol`、`appsByProtocol`、`workerPoolSize`、`forwardWorkers`、`forwardQueueLen` 等；**Go runtime** 堆/GC 摘要（`host.goRuntime`）；可选 **`traffic`**（终端/主站累计字节与近 15 分钟粒度，由 `fep` 采样）；**698 Bridge** 是否启用及 `bridge698Host`（配置非空且首字符非 `0` 时视为启用）。
- 总览页 **宜** 提供快捷跳转至 **§3.2**、**§3.3**；**§3.4 698 桥接** 仅在 **`BridgeHost698` 已启用**（与 `/api/status` 的 `bridge698Enabled` 一致）时显示入口。

### 3.2 在线终端信息（FR-WEB-021）

**必须** 提供 **表格化** 展示当前 **终端侧** TCP 会话（本进程内 **合并多监听/多规约**，与 [MQTT_SPEC.md](./MQTT_SPEC.md) §1.4、§2.3、§2.4 数据源一致）；**管理员与普通用户均须可查看**（只读）。

| 列（逻辑名） | 说明 |
|--------------|------|
| **序号** | 界面展示用连续序号或稳定行键（刷新可重排时 **宜** 附加 `connId` 等稳定标识列，便于对照） |
| **IP:port** | 终端侧 **远端** `IP:port`（或对等网络标识，实现定稿） |
| **协议** | 规约/协议字符串（与 MQTT `protocol`、内部 ptype 映射一致或可转换） |
| **通信地址 `addr`** | 业务终端地址（登录/绑定后） |
| **连接时间** | TCP 建立时间，对应连接层 `connTime`（RFC3339 UTC / `Z`，与 MQTT presence 一致） |
| **登录时间** | 地址绑定/登录成功时间 `loginTime` |
| **最近心跳时间** | `heartbeatTime`（若无统计可为 `null` 或省略键，实现定稿） |
| **最近接收时间** | `lastRxTime` |
| **最近发送时间** | `lastTxTime` |
| **最近上报时间** | `lastReportTime` |
| **上行报文数 / 下行报文数** | `uplinkMsgCount`、`downlinkMsgCount` |
| **上行流量 / 下行流量** | `uplinkBytes`、`downlinkBytes`（大数展示规则与 MQTT 侧一致，可为字符串十进制） |

**必须** 支持 **按协议筛选**、**按 `addr` / IP 搜索**（至少一种全文过滤）；**宜** 支持排序（如按登录时间、流量）、分页或虚拟滚动（大量连接时）。

**同 `addr` 多连接**：展示策略须与 MQTT **`gateway/terminals/presence`** 规则 **一致**（默认 **仅展示当前选中会话一条**：`loginTime` 最新，并列则 **较大 `connId`**），或界面提供 **「展开全部会话」**（若实现展示多条，须在发行说明中写明，且与 MQTT 默认单条语义不冲突）。**宜** 展示可选 **`connId`** 列供排障。

**当前实现约定**：

- **协议列字符串**（终端）：`376.1`、`698.45`、`NW`（与内部 profile 对应；与 MQTT 字符串 `376`/`698` 等 **可能字面不同**，集成时做映射表）。
- **JSON 字段**：`connId`、`remoteTcp`、`protocol`、`addr`、`connTime`、`onlineDuration`、`loginTime`、各 `last*`、上下行条数与流量字符串等，见 `web.TerminalRow`。
- **查询参数**：`GET /api/terminals` 支持 `protocol`、`q`（匹配 `addr` 或 `remoteTcp` 子串）、`expand=1`（同 addr+协议多会话全展）、`sort=addr|login`、`order=asc|desc`、分页 `page` / `pageSize`（允许 10～1000 中预设档）或 `all=1` 全量（慎用）。
- **踢下线**：`POST /api/terminals/kick`，body `{"connId":<uint>}`，**仅管理员**；成功后断开该终端 TCP（须当前在任一端终端 registry 中）。

### 3.3 APP / 主站后台连接信息（FR-WEB-022）

**必须** 单独列表展示 **上位机 / 主站 / APP 后台** 与 GFEP 之间的 **非终端监听类 TCP（或等价）连接**（例如主站召测链路、扩展服务口等，**以实际代码中的连接类型为准**，实现须在发行说明中列出纳入本表的连接来源）。

| 列（逻辑名） | 说明 |
|--------------|------|
| **序号** | 同 §3.2 |
| **IP:port** | 主站/APP 侧 **远端** `IP:port` |
| **协议** | 该链路的协议或业务类型标识（如「698 主站」「扩展口」等，实现定稿枚举） |
| **主站地址** | 对端身份摘要：**对端 IP**、和/或 **配置中的主站名/链路名**、和/或 **监听本地 `IP:port`**，至少一种 **人类可读**（实现定稿） |
| **连接时间** | 会话建立时间（RFC3339 UTC / `Z`） |
| **最近接收时间** | 最近一次从该连接 **收到** 数据时间 |
| **最近发送时间** | 最近一次 **向该连接发送成功** 时间 |
| **最近上报时间** | 若该链路上区分「业务上报」与纯心跳，则填业务上报时间；否则 **宜** 与最近接收同源或显示 `—`（实现定稿） |
| **上行报文数 / 下行报文数** | 相对 **GFEP 视角**：**上行** = 主站→GFEP 条数，**下行** = GFEP→主站 条数（须在表头或帮助文案中写清，避免与终端表混淆） |
| **上行流量 / 下行流量** | 同上，字节累计 |

若无此类连接（未启用相关功能），界面 **必须** 展示 **空状态** 说明，而非报错。

**当前实现纳入范围**：**376 / 698 / NW 主站侧** registry 上的 TCP 连接（与终端监听 **非同一列表**）。**协议列**示例：`376-主站`、`698-主站`、`Nw-主站`。**`masterSummary`** 含主站登记地址、远端 `IP:port`、本机监听 `Host:TCPPort` 拼接说明。`GET /api/apps?q=` 支持对摘要/IP 子串过滤。

### 3.4 698 桥接连接（FR-WEB-023）

**必须** 在 **`BridgeHost698` 已配置且视为启用**（非空且首字符非 `0`，与 `tryStartBridge698` 一致）时，在主导航提供 **独立菜单/页面**（与「在线终端」「主站/APP」并列）；**未启用时主导航不展示该项**（**`GET /api/bridges` 仍可用**，便于脚本/书签，返回空表即可）。展示 **桥接链路**：在 **终端侧 TCP 会话** 上挂起的、GFEP 作为客户端 **出站拨号** 至配置 **`BridgeHost698`**（等）的 **第二条 TCP**，用于 698（及实现支持时的其它规约）透传。

| 列（逻辑名） | 说明 |
|--------------|------|
| **序号** | 展示用 |
| **终端 `connId` / `addr` / 终端 IP:port** | 关联的终端连接（与 §3.2 同一 `connId` 可对查） |
| **桥终端地址（hex）** | 桥接登录使用的地址域（实现为 hex 串，与规约一致） |
| **桥对端（主站）** | `BridgeHost698` 配置值（`IP:port`） |
| **规约** | 当前实现以 **698.45** 为主；扩展时标注 `376.1` / `NW` 等 |
| **状态** | 未连接 / TCP 已连 / 已登录 / 退出中（实现枚举 + 中文文案） |
| **本段 TCP 起始时间** | 当前桥接 socket 建立时刻；重连后重置 |
| **桥登录时间** | 主站确认登录成功时刻 |
| **最近心跳时间** | 收到主站对心跳类确认的最近时刻 |
| **本段 TCP 在线时长** | 由 TCP 起始时刻推算（展示文案） |
| **最近收 / 最近发** | 桥接 socket 上最近自主站收包、至主站发包时刻 |
| **自主站收（rx）** | 帧数（判帧回调次数）与 **原始读字节**（可含粘包） |
| **至主站发（tx）** | 写次数（每次 `Write` 计 1 帧）与写字节累计 |
| **heartUnAck** | （可选列）未应答心跳累计，供排障 |

- **管理员与普通用户** 均 **只读** 查看；**不得** 与 §3.3「主站 inbound」混淆：§3.3 为 **APP 连入 GFEP 监听口**，§3.4 为 **GFEP 连出至主站**。
- **无桥接对象**（未启用 `BridgeHost698` 或无终端挂桥）时 **空状态** 说明。

**当前实现**：`GET /api/bridges?q=`；`web.BridgeRow` JSON；`bridge.Conn` 内统计 **TCP 重连时清零**；`ziface.IConnManager` 提供 **`Range`** 遍历以收集带 `bridge` 属性的连接。登录后拉取 **`/api/status`**，仅当 **`bridge698Enabled===true`** 时在主导航与总览快捷链插入 **「698 桥接」**。

### 3.5 实时通信日志（FR-WEB-030）

- **必须** 支持 **实时** 查看通信类日志（内容与现有 `LogPacketHex`、`LogLinkLayer` 等开关语义一致或子集，实现定稿）。
- **必须** 支持按 **终端地址 `addr` 过滤**（包含/精确匹配规则实现定稿）；可选按协议、方向（上行/下行）过滤。
- **宜** 使用 **WebSocket** 或 **SSE** 推送；若轮询，间隔与条数上限须限制，避免压垮服务。
- 展示 **UTF-8** 编码，异常二进制 **宜** 以 hex 转义展示，避免乱码与前端报错。

**当前实现**：**SSE** `GET /api/logs/stream`，Query：`addr`（匹配 JSON 行内 `addr`/`remoteTcp` 或整行子串）、可重复 `protocol`。心跳注释包 `: ping` 每 25s。订阅数有上限（过多返回 503）。

### 3.6 历史日志下载（FR-WEB-040）

- **必须** 保留 v1 能力：在授权下 **列举** `LogDir`（或 `web.log_root` 等价路径）下可下载文件，**禁止** `../` 路径穿越；非法路径 **404/403**。
- **必须** 支持 **HTTP 下载**，`Content-Disposition: attachment`；大文件 **宜** 流式输出。
- UI 上 **必须** 提供与权限匹配的下载入口（管理员与普通用户至少均可下载其权限范围内的日志，若需区分则写入实现说明）。

**当前实现**：`GET /api/logs/files` 返回树形列举（深度限制防遍历爆炸）；`GET /api/logs/download?name=<相对 log 根的路径>`，`..` 拒绝。下载写 **审计日志**（含远端地址，匿名用户名）。

### 3.7 软件参数与配置（FR-WEB-050）

- **必须** 支持在界面中 **查看** 当前有效配置（可与 `gfep.json` / 运行时合并结果对应，敏感项如密码字段 **脱敏** 显示）。
- **宜（管理员）** 支持 **在线修改部分参数** 并持久化或热更新（范围实现定稿：如日志开关、超时、转发参数等）；**必须** 对写操作记 **审计日志**。
- 普通用户 **默认仅只读**；若未来扩展「授权编辑子集」，需升 SPEC 版本。

**当前实现**：`GET /api/config` 返回 **`effective`** 脱敏 Map（`web.RedactEffectiveConfig`）。`PUT /api/config` **仅管理员**，仅允许写入下列键（其余忽略）：`LogPacketHex`、`LogLinkLayer`、`LogForwardEgressHex`、`LogDebugClose`、`LogConnTrace`、`LogNetVerbose`、`Timeout`、`FirstFrameTimeoutMin`、`PostLoginRxIdleMinutes`、`ForwardWorkers`、`ForwardQueueLen`。写盘后 **`GlobalObject.Reload()`**，并按 `LogDebugClose` 切换 `zlog` Debug。审计日志含用户名与键名列表。

### 3.8 黑名单管理（FR-WEB-060）

- **必须**（管理员）支持维护 **终端地址黑名单**（增删查；持久化位置与格式实现定稿）。
- **必须** 与连接层策略一致：列入黑名单的地址 **拒绝新连接** 或按现有业务规则处理（与 `znet`/registry 集成点实现定稿）。

**当前实现**：持久化 **`conf/terminal_blacklist.json`**（与 `web_users.json` 同目录策略）。`GET /api/blacklist` 任意已登录用户；`PUT /api/blacklist` **仅管理员**，body `{"addrs":["..."]}` 全量替换。终端登录前 **`web.TerminalAddrBlacklisted`** 检查（见 `gfep_ptl`）。审计日志含管理员与条数。

### 3.9 日志级别管理（FR-WEB-070）

- **必须**（管理员）可在界面调整 **全局或模块日志级别**（如 Error/Warn/Info/Debug），生效策略：**立即** 或 **重启后**（实现选一并文档化）。
- **宜** 与现有 `zlog`/应用日志实现统一，避免多套级别互不生效。

**当前实现**：`GET /api/log-level` 任意已登录用户；`PUT /api/log-level` **仅管理员**，体 `{"logDebugClose": true|false}`，写 `gfep.json` 中 **`LogDebugClose`** 并 `Reload` + `zlog.CloseDebug`/`OpenDebug`。**尚未** 暴露细粒度 per-logger 级别；扩展时升 SPEC。

---

## 4. 建议增补功能（Backlog）

以下 **不阻塞首版上线**，可按迭代纳入：

| 项 | 说明 |
|----|------|
| **转发/MQTT 状态** | 若启用转发或 MQTT，展示 Broker 连接态、最近错误 |
| **Bridge 深度探测** | 如 RTT、主站协议级就绪；当前已有 **§3.4** 连接表 + **`/api/status`** 启用标志 |
| **操作审计页** | 管理员查询登录、配置变更、黑名单修改记录 |
| **只读模式** | 全局开关：灾难演练或合规场景下仅允许查看 |
| **速率限制** | 对登录、下载、API **限流**，防暴力与拖库 |
| **双因素认证** | 管理账号可选 2FA（另立安全 SPEC 亦可） |
| **国际化** | 中/英 UI 切换（可选） |

---

## 5. 界面与体验（非功能）

| ID | 要求 |
|----|------|
| NFR-WEB-UI-001 | 整体风格 **现代、简约、时尚**；信息层级清晰，**常用操作 ≤3 步** 可达 |
| NFR-WEB-UI-002 | **响应式布局**，主流桌面浏览器优先；移动端 **宜** 可用 |
| NFR-WEB-UI-003 | 全站 **UTF-8**；HTTP 头、`meta`、JSON API **编码一致**，避免乱码与前端脚本错误 |
| NFR-WEB-UI-004 | 前端构建 **无控制台报错**（无未捕获异常、无资源 404）；实现可选用成熟组件库以保持视觉统一 |
| NFR-WEB-UI-005 | 关键操作 **确认**（删除黑名单、修改级别等）；错误提示 **人类可读** |

---

## 6. 安全与运维

| ID | 要求 |
|----|------|
| NFR-WEB-SEC-001 | 默认 **关闭** `LogWebEnabled`；生产 **宜** 仅内网或回环 + 反向代理 |
| NFR-WEB-SEC-002 | 所有 API **鉴权**；管理员接口 **必须** 校验角色 |
| NFR-WEB-SEC-003 | **CORS**：若跨域，**显式白名单**；内网单域可默认收紧 |
| NFR-WEB-SEC-004 | **CSRF**：若使用 Cookie 会话，**必须** 采取 SameSite/Token 等防护 |
| NFR-WEB-SEC-005 | 安全响应头 **宜** 配置（如 `X-Content-Type-Options: nosniff`） |
| NFR-WEB-OPS-001 | Web 访问与鉴权失败 **宜** 写入应用日志，便于审计 |

**当前实现**：`web.securityHeaders` 已设 `X-Content-Type-Options: nosniff`、`X-Frame-Options: SAMEORIGIN`、`Referrer-Policy: strict-origin-when-cross-origin`；**未** 内置 HTTPS（`ListenAndServe`）。

---

## 7. 配置项汇总（与实现对齐）

**当前仓库字段**（`conf/gfep.json` / `utils.GlobalObject`）：

```text
LogWebEnabled          # 是否启用 Web 控制台（默认 false）
LogWebHost             # 监听 IP，默认 0.0.0.0
LogWebPort             # 监听端口，默认 20084
LogDir                 # 日志根目录（列举/下载与实时/历史日志路径基准）
LogWebSessionIdleMin   # 会话空闲超时（分钟），<=0 时默认 480（8h）；每次请求滑动续期
LogWebSessionCookie    # 会话 Cookie 名；空则 gfep_session_<LogWebPort>
```

**落盘文件**（与 `ConfFilePath` 同目录，通常为 `conf/`）：`web_users.json`（账号 bcrypt）、`terminal_blacklist.json`。

**建议随实现扩展**（名称可与代码统一为 `web.*` 或保持 `LogWeb*` 前缀，二选一并文档化）：

```text
web.tls.*              # 可选 HTTPS（当前未接 ListenAndServeTLS）
web.log_root           # 若与 LogDir 分离时覆盖下载根目录；否则同 LogDir
web.auth.*             # 若保留 Basic/Token 作为补充鉴权方式
web.rate_limit         # 可选限流参数
```

---

## 8. 验收要点（Web）

1. `LogWebEnabled=false` 时 **无** Web 监听。
2. 管理员与普通用户登录后菜单与接口权限 **符合 §2、§3**。
3. **多用户** 可同时登录且互不挤掉（除非实现声明「单点登录」策略，本 SPEC 默认 **并发会话**）。
4. 状态页展示 **CPU/内存/磁盘** 与 **按协议在线终端**；刷新不拖垮主业务。
5. **在线终端表**（§3.2）含 **序号、IP:port、协议、`addr`、连接/登录/心跳/收发/上报时间及上下行条数与流量**；与 MQTT presence 语义一致或可解释的差异已写入发行说明。
6. **APP/主站连接表**（§3.3）字段完整；无连接时空状态；**上下行** 相对 GFEP 视角与帮助文案一致。
7. **698 桥接**：**已启用 BridgeHost698** 时主导航有菜单且 **§3.4** 与 **`/api/bridges`** 一致；**未启用** 时菜单隐藏；无桥接对象时空状态。
8. 实时日志：**可选 addr** 过滤，长时间运行 **无** 明显内存泄漏（背压或丢弃策略可文档说明）。
9. 历史日志：**路径穿越** 无效；授权用户可下载。
10. 黑名单与日志级别：**仅管理员** 可写；生效行为与运行时一致。
11. **终端踢下线**（`POST /api/terminals/kick`）：**仅管理员**，合法 `connId` 可断开对应终端连接。
12. 页面与 API **UTF-8** 正常，主流浏览器无 **编码相关** 乱码与前端报错（按 NFR-WEB-UI-003、004）。

---

## 9. 修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-03-28 | 自合并版 SPEC 拆出 |
| 1.1 | 2026-03-28 | 增加 §7 与通用 Web 安全实践对照 |
| 2.0 | 2026-03-28 | 升级为 **Web 管理控制台**：角色、状态、实时/历史日志、配置、黑名单、日志级别；**沿用 LogWebPort**；UI/NFR 与验收更新；原「仅静态目录」降为附录子能力 |
| 2.1 | 2026-03-28 | 新增 **§3.2 在线终端明细**、**§3.3 APP/主站连接明细**（字段表、筛选、与 MQTT 对齐说明）；原 §3.2–§3.6 顺延为 §3.4–§3.8；验收 §8 增补 |
| 2.2 | 2026-03-28 | **与仓库实现对齐**：§1.5 控制台时间 vs MQTT；各节「当前实现」摘要；§7 会话配置与落盘文件；**§10 API 索引**；终端行补 **`connTime`**；踢线改 **仅管理员** 并记审计 |
| 2.3 | 2026-03-28 | **§3.4 698 桥接** 独立菜单与 **`GET /api/bridges`**；`bridge.Conn` 流量/快照；`IConnManager.Range`；前端「698 桥接」页；原 §3.4–§3.8 顺延为 §3.5–§3.9 |
| 2.4 | 2026-03-28 | **未配置 BridgeHost698** 时隐藏主导航「698 桥接」与总览快捷入口（仍可调 **`/api/bridges`**） |

---

## 10. 实现架构与 API 索引（仓库同步）

本节描述 **当前** `gfep` 代码布局与 HTTP 路由，供评审与联调；细节以源码为准。

### 10.1 代码位置

| 区域 | 路径 | 说明 |
|------|------|------|
| 启动入口 | `fep/logweb.go` | `startLogWebIfEnabled` → `web.StartConsole` |
| 数据注入 | `fep/snapshot_web.go`、`fep/traffic_history.go` | `web.Provider`：主机指标、终端/主站/**桥接**行、流量采样、`KickTerminal` |
| 桥接统计 | `bridge/bridge.go`、`bridge/snapshot.go` | 桥接 socket 读写字节/帧计数、`Snapshot()` |
| 控制台核心 | `web/*.go` | 路由、鉴权、用户文件、黑名单、SSE、嵌入静态页 |
| 实时日志源 | `fep/live_publish.go`、`fep/connutil.go` | `web.PublishLiveJSON` / `PublishLivef`（受 `LogWebEnabled` 控制） |

### 10.2 HTTP 路由（前缀均在同端口）

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/api/auth/login` | 无 | JSON `username`/`password`，Set-Cookie 会话 |
| POST | `/api/auth/logout` | 可选 | 清会话 |
| GET | `/api/auth/me` | 登录 | 当前用户与 `role` |
| GET/POST/PUT/DELETE | `/api/users` | **admin** | Web 账号 CRUD（bcrypt） |
| GET | `/api/status` | 登录 | 主机 + 业务摘要（见 §3.1） |
| GET | `/api/terminals` | 登录 | 见 §3.2 |
| POST | `/api/terminals/kick` | **admin** | 见 §3.2 |
| GET | `/api/apps` | 登录 | 见 §3.3 |
| GET | `/api/bridges` | 登录 | 见 §3.4，`?q=` 过滤 |
| GET | `/api/logs/files` | 登录 | 列举 `LogDir` |
| GET | `/api/logs/download` | 登录 | `?name=` 相对路径下载 |
| GET | `/api/logs/stream` | 登录 | SSE 实时日志 |
| GET/PUT | `/api/config` | GET 登录 / PUT **admin** | 有效配置脱敏 / 白名单键补丁 |
| GET/PUT | `/api/log-level` | GET 登录 / PUT **admin** | `LogDebugClose` |
| GET/PUT | `/api/blacklist` | GET 登录 / PUT **admin** | 终端地址黑名单 |
| GET | `/`、`/index.html` | 无 | 嵌入 `index.html`（`charset=utf-8`） |
| GET | `/static/*` | 无 | `app.js`、`style.css` |

### 10.3 审计日志（`logx`，前缀 `web audit:`）

含：登录成功/失败、配置补丁、黑名单更新、日志级别、`LogDebugClose`、**日志下载**、**终端踢下线**。

### 10.4 与 SPEC 仍有差距 / 后续可做

- **HTTPS**、**限流**、专用 **审计查询页**：见 §4 Backlog。
- **MQTT Broker 状态**：未在 `/api/status` 暴露（若后续接入 MQTT 可增补）。

---

## 11. 附录：v1「仅日志下载」能力对照

以下为 v1 核心要求，**已并入** §3.6、§6、§8；实现时 **不得** 弱化路径安全与授权。

- 将访问限制在日志根目录内，**禁止** `../` 穿越。
- 大文件下载 **宜** 流式。
- 仅 **GET**（及必要时 **HEAD**）用于静态下载路由；管理 API 方法集实现定稿。

**MQTT 侧平台对照** 见 [MQTT_SPEC.md](./MQTT_SPEC.md) 第 12 节。
