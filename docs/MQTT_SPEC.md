# GFEP：MQTT 集成 — 需求规格（SPEC）

| 项 | 内容 |
|---|------|
| 状态 | Draft |
| 版本 | 2.0 |
| 关联 | [Web 日志下载 SPEC](./WEB_SPEC.md) |

---

## 1. 背景与目标

### 1.1 背景

GFEP 作为网关/前置，已具备多规约终端接入与报文处理能力。需为上层 APP/平台提供 **MQTT** 能力：查询、控制、订阅。

### 1.2 目标

通过 MQTT 暴露终端在线查询、按地址查询在线与活动时间、JSON 点抄/广播、订阅上行报文与上下线事件；MQTT 工作模式可为 **客户端连接外部 Broker** 或 **Broker/嵌入式 Broker**（可配置，见 §3）。

系统可部署 **多个 GFEP 节点** 共用同一 Broker，形成集群：**Topic 必须能区分节点**（含 `nodeId`）；APP 下行时可 **指定节点** 或按约定走 **集群广播** Topic。

### 1.3 非目标（本期或另立 SPEC）

- 完整 MQTT Broker 集群、多租户计费。
- 设备孪生/影子（详见第 12 节）；规则引擎、持久化队列等平台侧能力（除非另立 SPEC）。
- 跨 GFEP 的 **终端迁移 / 会话粘性**、**全局一致路由表**（由上层系统或配置中心承担，本 SPEC 仅约定 MQTT 侧分域）。

### 1.4 时间与多规约视图

- **时间戳（定稿）**：JSON 中所有 **`ts`** 及 presence 内时间字段（`connTime`、`loginTime` 等）**必须** 使用 **RFC3339**，**UTC**，以 **`Z`** 结尾（例如 `2026-03-28T02:00:02.123Z`）。**不** 使用 Unix 毫秒作为这些字段的编码方式（与其它系统集成时可在网关外转换）。
- **多监听 / 多 registry（376+698 等）**：同一 GFEP **进程** 使用 **同一 `mqtt.node_id`**、**同一套** `{N}` Topic。**`gateway/terminals/list`** 与 **`gateway/terminals/presence`** 的响应 **必须** 为该进程内 **全部** 终端监听口的 **合并在线视图**（每条带各自 `protocol`）；**不得** 仅返回单一规约子集（除非请求中带 `protocol` / `protocols` 过滤）。

### 1.5 术语

| 术语 | 说明 |
|------|------|
| 终端地址 `addr` | 业务侧标识终端的字符串；Topic 中使用时须 **Topic 安全**（§6）；与 `nodeId` 规则相同 |
| 点抄 | APP 已组织好的 **完整 hex 报文**（一条终端链路透传帧）；GFEP **仅** 将 hex **解码为二进制后原样写入** 该终端 TCP，**不** 重新组帧、**不** 修改字节序或内容（见 §2.5、§4.4） |
| 广播 | 与点抄相同：**同一 `frameHex` 字节串** 由 **各 GFEP 节点** 对本地命中的终端 **各自原样下发**；是否过滤目标终端由实现与规约定义（如通配地址） |
| `v1` | **Topic API 版本段**，固定为整条 MQTT Topic 的 **首段**；本版为 `v1`，后续演进可为 `v2` 等。`mqtt.topic_prefix` **不含** 本段，由实现统一前置拼接。 |
| `{root}` | 配置项 `mqtt.topic_prefix`，无尾 `/` |
| `{pk}` | 配置项 `mqtt.product_key`；为空时路径中 **不写** `{pk}/` 段（实现须规范拼接，避免 `//`） |
| `nodeId` | 本 GFEP 实例在集群中的 **逻辑节点标识**，配置项 `mqtt.node_id`；在 Topic 中须 **Topic 安全**（§6） |
| `{N}` | **节点作用域前缀**：`v1/{root}/{pk}/nodes/{nodeId}`（`{pk}` 为空时为 `v1/{root}/nodes/{nodeId}`，不得出现连续空段） |
| `{C}` | **集群作用域前缀**（无 `nodeId` 段）：`v1/{root}/{pk}/cluster`（`{pk}` 为空时为 `v1/{root}/cluster`） |

**示例**（`mqtt.topic_prefix=gfep/plant`，`mqtt.product_key=tmn`，`nodeId=node-a`，`addr=T001`）：

- 终端上行（形态 A）：`v1/gfep/plant/tmn/nodes/node-a/devices/T001/messages/events`
- 集群广播下行：`v1/gfep/plant/tmn/cluster/messages/devicebound`

---

## 2. MQTT 功能需求

### 2.1 工作模式（FR-MQTT-001）

- 系统 **必须** 支持通过配置选择至少一种：**客户端连外部 Broker**，或 **本机/同部署 Broker（含嵌入式）**。
- **必须** 支持 TLS（可开关）、用户名密码或证书等 **一种以上** 鉴权方式（细节在实现与运维文档中列出）。

### 2.2 载荷约定（FR-MQTT-002）

- 载荷 **UTF-8 JSON**。
- RPC **宜** 带 `requestId`；响应 **必须** 回显相同 `requestId`（若请求携带）；若请求 **省略** `requestId`，见 §2.5。
- **`protocol`（字符串，§9）**：凡 **与终端业务相关** 的 GFEP 发布消息（`events`、`status`、`operations/result`、终端类 `gateway/.../response` 等）及 **APP 的 `devicebound`** **必须** 携带。**例外**：**`{N}/gateway/health/request`**（APP 发起时可无 `protocol`）、**`{N}/bridge/status`** 等纯系统消息可不携带，此时不视为违反本规则。
- 可选在 Topic 增加协议段（§5）。
- **集群**：由 GFEP 发布的、含终端语义的消息 JSON **必须** 含 **`nodeId`**（与本机 `mqtt.node_id` 一致）；系统类消息（如仅含进程状态的 `bridge/status`）**宜** 含 `nodeId`。

### 2.3 在线终端清单（FR-MQTT-010）

- APP **必须** 能查询当前在线终端清单（至少含 `addr`；可选 `connId`、协议类型等）。清单数据 **必须** 覆盖本进程内 **所有** 规约监听侧的在线终端（合并视图），见 §1.4。
- **必须** 支持在请求中 **按协议过滤**（见 §4.2）：可选 **`protocol`**（单协议）或 **`protocols`**（多协议，逻辑为「或」）；二者 **不得同时出现**，否则响应 **`ok: false`** 及明确 `code`（如 `INVALID_REQUEST`）。均未提供或 `protocols` 为空数组时，返回 **全部协议** 的在线终端。
- 超时/错误返回可解析 JSON（`ok`/`code`/`message`）。

### 2.4 按地址查询在线与活动时间（FR-MQTT-011）

- **必须** 支持按 **一个或多个** `addr` 查询是否在线。数据源为 **合并视图**（§1.4、§2.3）。
- **同 `addr` 多连接**：响应中 **必须** 只描述 **一条** 当前会话——取 **`loginTime` 最新** 的一条（若并列则以 **较大 `connId`** 决胜，实现须固定规则并写进发行说明）。**不** 使用 `sessions[]`（若未来需要多会话，另增字段并升 SPEC 版本）。
- 当 **在线** 时，**必须** 在同条响应中返回下列 **时间**（格式见 §1.4，**RFC3339 UTC / `Z`**）：

| JSON 字段 | 语义 | 与连接层对应（实现参考） |
|-----------|------|---------------------------|
| `connTime` | TCP 连接建立时间 | `Connection.ctime` |
| `loginTime` | 终端登录/地址绑定成功时间 | `Connection.ltime` |
| `heartbeatTime` | 最近一次协议心跳时间 | `Connection.htime` |
| `lastRxTime` | 最近一次 **收到** 终端侧任意报文时间 | `Connection.rtime` |
| `lastTxTime` | 最近一次 **向终端发送成功** 时间 | 若未实现则键保留、值为 `null` |
| `lastReportTime` | 最近一次 **业务上报**（如主动上报帧）时间 | 实现按规约区分上报与心跳等；若无单独统计可与 `lastRxTime` 同源 |

- 当 **在线** 时，**必须** 在同条响应中返回下列 **统计**（非负整数；字节数过大时可用 **字符串** 十进制表示以避免 JSON number 精度问题，实现与 APP 约定一致即可）：

| JSON 字段 | 语义 |
|-----------|------|
| `uplinkMsgCount` | **上行**报文 **条数**（终端 → GFEP，以「完成解析/判帧」的报文计，不含纯 TCP ACK 等由实现定稿） |
| `downlinkMsgCount` | **下行**报文 **条数**（GFEP → 终端） |
| `uplinkBytes` | **上行**累计 **字节流量**（终端 → GFEP，可与 `uplinkMsgCount` 同统计窗口） |
| `downlinkBytes` | **下行**累计 **字节流量**（GFEP → 终端） |

- **统计口径**（起算点、是否含心跳/注册类报文）：由实现与规约 **定稿并在发行说明中写明**；同一部署内各字段口径须一致。
- **离线**：`online: false`；**时间类与统计类字段必须省略**（不出现键），**不得** 与历史会话数值混用。

### 2.5 JSON 点抄与广播（FR-MQTT-012）

- **必须** 支持点抄至指定 `addr`、广播至允许范围。
- **点抄 / 广播下行载荷**：APP **必须** 提供 **`frameHex`**（见 §4.4）；GFEP **必须** 按约定解码后 **原模原样** 发往终端 socket（**不做** 应用层组帧、填充、加密变换；若链路上另有透明通道如 TLS，以实现为准且不改变「字节级透传」语义）。
- **广播（单节点内）**：仅向 JSON **`protocol`** 与 **该终端连接实际规约** 一致的在线终端下发（与点抄相同的 **PROTOCOL_MISMATCH** 规则，§4.4、§5.3）；通配地址帧由规约与实现定义是否命中多连接。
- **`operations/result` 成功语义（透传模式）**：**`ok: true`** 表示 **已向终端 TCP 完成本次写入**（`write` 成功且达到实现定义的字节数）；**不** 表示终端已 ACK 或业务成功。超时/离线等见 **`code`**（§9.1）。
- **`timeoutMs`（可选，定稿）**：从 GFEP **开始处理** 该条 `devicebound` 起，到 **完成对终端 TCP 的 `write`（或等效发送完成）** 为止的 **wall-clock** 上限。**包含** 在本进程内 **排队等待发往该连接** 的时间（若实现有 per-connection 发送队列）。**不包含**：MQTT 从 Broker 到 GFEP 的投递延迟、等待终端 **回帧/ACK** 的时间。超时则 **`operations/result`** 中 `ok: false`，`code: TIMEOUT`（§9.1），已实现部分写入的实现须文档说明是否部分成功。
- **`operations/result` — `kind: broadcast` 时 `detail`（必须）**：**必须** 含 **`targetsAttempted`**（本节点尝试匹配的在线终端数，按实现定义，通常为过滤 `protocol` 后的集合大小）、**`targetsSent`**（实际完成 `write` 的终端数）、**`targetsSkipped`**（未尝试或跳过数，满足 `attempted = sent + skipped` 或实现文档中的守恒式）。无命中时 **`targetsSent: 0`**，`targetsSkipped` 与 `targetsAttempted` 一致或按实现定义。
- **`requestId`（定稿）**：APP **宜** 始终携带；若 **省略**，GFEP **仍须** 处理命令并发布 **`operations/result`**，且 **`requestId` 字段省略或为空字符串**（二选一并写死，推荐 **省略键**）；APP 侧 **不得** 依赖与下行的一对一关联，仅能通过 `nodeId`+时间序等弱关联。
- **必须** 有结果反馈（统一结果 Topic，§3.3），含 `kind: unicast|broadcast` 与成功/失败信息。

### 2.6 订阅终端上行（FR-MQTT-020）

- **必须** 可订阅终端上行；GFEP 发布 **`events`** 时 **必须** 带 **`payloadEncoding`**（`hex` \| `base64`）及对应 **`payload`**；可选 **`parsed`**（实现定稿）。
- QoS 建议 **1**；Retain 默认 **关闭**。

### 2.7 订阅上下线（FR-MQTT-021）

- **必须** 可订阅上线/下线事件；重连风暴 **宜** 防抖或可配置合并策略。

### 2.8 其它（FR-MQTT-030）

- **宜** 暴露 MQTT 连接状态指标、日志；**宜** 支持按 Topic/凭证的 ACL（只读 vs 可下发）。
- **宜** 提供 **`{N}/gateway/health/request`** / **`response`**（§4.7、§13.6），与 **`{N}/bridge/status`**（遗嘱/周期心跳，§3.4）配合，满足运维拉取与订阅告警两类需求。

### 2.9 集群与节点路由（FR-MQTT-040）

- 每个 GFEP 实例 **必须** 配置集群内唯一的 `mqtt.node_id`（与 Broker 上 ClientId 唯一性等约束独立，二者均须满足部署规范）。
- **GFEP → APP**（上行报文、上下线、命令结果、`gateway/.../response`）：Topic **必须** 使用 **节点作用域前缀 `{N}`**（见 §1.5），即以 **`v1/`** 开头且路径中 **必须** 包含 `nodes/{nodeId}` 段。
- **APP → GFEP**：
  - **指定节点**：发布到 **`{N}/...`** 下对应子路径（点抄、`gateway/.../request` 等），仅该 `nodeId` 的 GFEP 订阅处理。
  - **不指定节点（可选）**：
    - **下行广播**（面向所有 GFEP，由各节点自行判断是否持有目标终端）：使用 **`{C}/messages/devicebound`**。各 GFEP **必须** 订阅该 Topic；对 **本地无任何命中终端** 的节点，仍 **必须** 发布一条 **`operations/result`**（`kind: broadcast`，`ok: true`，`detail` 含 **`targetsAttempted` / `targetsSent: 0` / `targetsSkipped`**，§2.5、§13.5），以便 APP 按 `requestId`+`nodeId` 聚齐全集。
    - **网关 RPC 集群入口**（可选能力）：**`{C}/gateway/.../request`**，各 GFEP 均订阅。请求 JSON 可带可选字段 **`targetNodeId`**：若 **存在**，仅 `mqtt.node_id == targetNodeId` 的节点处理并发布 **自己的** `{N}/gateway/.../response`；若 **省略**，**每个** GFEP 均处理并各发布一条 **带本机 `nodeId` 的 response**（APP 用 `requestId` + `nodeId` 聚合）。若实现 **不提供** 集群 RPC 入口，则 **必须** 仅支持 `{N}/gateway/.../request` 单节点投递。

---

## 3. Topic 总览（主流 IoT 风格 + 集群 `nodeId`）

在原有「设备 / 网关 / 消息」分层上，**整条 Topic 均以 `v1/` 开头**；**所有节点相关路径**挂在 **`v1/.../nodes/{nodeId}/...`** 下；**跨节点广播下行** 使用 **`{C}/...`**（无 `nodeId` 段，见 §1.5）。

### 3.1 数据面 — 上行事件（GFEP 发布）

| 语义 | Topic 模式 |
|------|------------|
| 终端上行 | `{N}/devices/{addr}/messages/events`（形态 B 见 §5，末尾可加 `/{protocol}`） |
| 单节点下全终端上行 | `{N}/devices/+/messages/events`（形态 B：`.../events/+`） |
| 连接状态 | `{N}/devices/{addr}/status` |
| 单节点下全终端状态 | `{N}/devices/+/status` |
| **集群汇聚订阅（APP）** | `v1/{root}/{pk}/nodes/+/devices/+/messages/events` 等（`{pk}` 为空时省略该段；用 `+` 匹配任意 `nodeId`） |

### 3.2 数据面 — 下行命令（APP 发布）

| 语义 | Topic 模式 |
|------|------------|
| 点抄（指定节点） | `{N}/devices/{addr}/messages/devicebound`（形态 B 见 §5） |
| 广播（**不指定节点**，全节点接收） | **`{C}/messages/devicebound`** |

### 3.3 结果与网关 RPC

| 语义 | 方向 | Topic |
|------|------|--------|
| 命令执行结果 | GFEP → APP | **`{N}/messages/operations/result`** |
| 清单 / presence **单节点** | APP → GFEP | **`{N}/gateway/terminals/list/request`** |
| 清单 / presence **单节点** | GFEP → APP | **`{N}/gateway/terminals/list/response`**（presence 同理 `.../presence/...`） |
| 网关 RPC **集群入口**（可选） | APP → 全体 GFEP | **`{C}/gateway/terminals/list/request`**（presence 同理）；行为见 §2.9 |
| **GFEP 健康查询** | APP → GFEP | **`{N}/gateway/health/request`** |
| **GFEP 健康响应** | GFEP → APP | **`{N}/gateway/health/response`** |

### 3.4 系统（可选）

| 用途 | Topic |
|------|--------|
| 网关 MQTT 健康 / LWT | **`{N}/bridge/status`**（建议带 `nodeId`，便于区分实例；可与 §13 周期心跳、`health` RPC 并存） |

---

## 4. JSON 消息定稿

### 4.1 通用字段

| 字段 | 说明 |
|------|------|
| `requestId` | RPC 关联用，可选；若带则响应必填回显 |
| `ts` | 服务端时间，**RFC3339 UTC（`Z`）**，见 §1.4；可选 |
| `ok` / `code` / `message` | 成功失败与错误信息；**`code`** 枚举见 §9.1 |
| `protocol` | 协议字符串；**必填性**见 §2.2（终端类消息必填） |
| `nodeId` | GFEP 节点标识；**必填性**见 §2.2 |
| `targetNodeId` | 仅用于 **`{C}/gateway/.../request`**（即 `v1/.../cluster/gateway/...`）：可选，指定仅该节点处理；省略则各节点均响应（§2.9） |

### 4.2 `gateway/terminals/list`

**request**（可为 `{}`）

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "targetNodeId": "node-b",
  "protocol": "376"
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `targetNodeId` | 否 | 仅当请求发布在 **`{C}/gateway/.../request`** 时有效；发布在 **`{N}/gateway/.../request`** 时 **省略**（由 Topic 限定节点）。 |
| `protocol` | 否 | 仅返回该 **`protocol`**（§9）的在线终端；与 **`protocols`** **二选一**。 |
| `protocols` | 否 | 字符串数组，返回匹配 **任一** 协议的在线终端；与 **`protocol`** **二选一**。 |

**过滤示例**：仅 698 — 使用 `"protocol": "698"` 或 `"protocols": ["698"]`；376 与 698 — `"protocols": ["376", "698"]`。不传 `protocol` / `protocols` 则不做协议过滤。

**response**（由对应 GFEP 发布至 **`{N}/gateway/terminals/list/response`**；`terminals` 中每条仍含各自 `protocol`）

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "nodeId": "node-a",
  "ok": true,
  "terminals": [
    { "addr": "T001", "connId": 123, "protocol": "376" }
  ]
}
```

### 4.3 `gateway/terminals/presence`

**request**

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "addrs": ["T001", "T002"],
  "targetNodeId": "node-b"
}
```

**response**

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "nodeId": "node-a",
  "ok": true,
  "results": [
    {
      "addr": "T001",
      "online": true,
      "connId": 123,
      "protocol": "376",
      "connTime": "2026-03-28T08:00:01.000Z",
      "loginTime": "2026-03-28T08:00:02.000Z",
      "heartbeatTime": "2026-03-28T08:05:00.000Z",
      "lastRxTime": "2026-03-28T08:05:01.000Z",
      "lastTxTime": "2026-03-28T08:05:00.000Z",
      "lastReportTime": "2026-03-28T08:04:58.000Z",
      "uplinkMsgCount": 128,
      "downlinkMsgCount": 36,
      "uplinkBytes": 8192,
      "downlinkBytes": 2048
    },
    { "addr": "T002", "online": false }
  ]
}
```

### 4.4 下行点抄 / 广播（`devicebound`）

**语义**：`frameHex` 为 **完整报文** 的连续十六进制字符串（**偶数个** `[0-9A-Fa-f]`）；GFEP 解码为原始字节后 **一字节不改** 写入终端连接。可选允许字间 **空格**，实现须在解码前去除空白。

- **非法 `frameHex`**（奇数长度、含非法字符、解码失败）：**不得** 向终端 TCP 写入；**必须** 发布 **`operations/result`**，`ok: false`，`code: HEX_INVALID`，`message` 人类可读。
- **规约不一致**：若 JSON **`protocol`** 与目标终端连接实际规约 **不一致**（含 Topic 形态 B 尾段与 JSON 不一致，§5.3）：**不得** 写入 TCP；**必须** 发布 `operations/result`，`ok: false`，`code: PROTOCOL_MISMATCH`。

**点抄**（publish 至 devicebound Topic，§5）

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "protocol": "376",
  "frameHex": "6812000000000000000000000000000000",
  "timeoutMs": 5000
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `requestId` | 宜填 | 与 `operations/result` 关联；省略时见 §2.5 |
| `protocol` | 是 | §9，用于路由与校验 Topic（形态 B） |
| `frameHex` | 是 | APP 已组织好的 **整条** 下行报文 |
| `timeoutMs` | 否 | **写 socket / 发送路径** 超时（§2.5）；省略则用实现默认 |
| `correlationId` | 否 | 与上行 `messageId` 等关联（§13.4） |
| `meta` | 否 | 透传诊断信息，GFEP **不得** 写入终端 |

**广播**（`{C}/messages/devicebound`，结构同点抄；无单终端 `addr`，由 Topic 与 `protocol` + `frameHex` 表达内容）

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440001",
  "protocol": "376",
  "frameHex": "6812FFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
  "timeoutMs": 8000
}
```

**`messages/operations/result`**

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "nodeId": "node-a",
  "kind": "unicast",
  "addr": "T001",
  "protocol": "376",
  "ok": true,
  "code": "0",
  "message": "",
  "ts": "2026-03-28T08:06:00.100Z",
  "detail": {}
}
```

`kind`：`unicast` | `broadcast`；广播时 `addr` 可省略或 `"*"`。**`ok: true`** 表示 **已完成对终端的本次写入**（§2.5），**不** 表示终端已确认报文。

**`kind: broadcast` 时 `detail` 必须字段**（§2.5）：`targetsAttempted`、`targetsSent`、`targetsSkipped`（非负整数）。

**`kind: unicast` 时 `detail`（宜）**：可含 `bytesSent`（整数）；失败时可为 `{}`。

### 4.5 `devices/.../messages/events`（上行）

**`payload`** 为 **原始帧** 的文本编码；**必须** 同时携带 **`payloadEncoding`**：`hex` | `base64`（与 §13.2 一致）。实现 **不得** 在 MQTT 载荷中同时混用两种编码而不声明。

```json
{
  "nodeId": "node-a",
  "addr": "T001",
  "connId": 123,
  "protocol": "376",
  "ts": "2026-03-28T08:05:01.000Z",
  "messageId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "payload": "68AAAA...",
  "payloadEncoding": "hex",
  "meta": {}
}
```

完整 Topic 与报文示例见 **§13.2**。

### 4.6 `devices/.../status`（上下线）

```json
{
  "nodeId": "node-a",
  "addr": "T001",
  "connId": 123,
  "protocol": "376",
  "state": "online",
  "ts": "2026-03-28T08:00:02.123Z",
  "reason": "login"
}
```

`state`：`online` | `offline`。详见 **§13.1**。

### 4.7 `gateway/health`（按需查询节点健康）

**request**（发布至 **`{N}/gateway/health/request`**）

```json
{
  "requestId": "health-req-001",
  "ts": "2026-03-28T02:20:00.000Z"
}
```

**response**（发布至 **`{N}/gateway/health/response`**）

```json
{
  "requestId": "health-req-001",
  "nodeId": "node-a",
  "ok": true,
  "ts": "2026-03-28T02:20:00.050Z",
  "mqtt": { "brokerConnected": true, "clientId": "gfep-node-a" },
  "process": { "uptimeSec": 86400, "version": "1.0.0" },
  "load": { "tcpTerminalConns": 42, "tcpAppConns": 3 }
}
```

字段扩展见 **§13.6**。

---

## 5. 协议：JSON 必填 + 可选 Topic 形态

### 5.1 配置

| 配置项 | 类型 | 默认 | 说明 |
|--------|------|------|------|
| `mqtt.protocol_in_topic` | bool | `false` | `false`：仅 JSON 带 `protocol`；`true`：在下方 §5.2 路径末尾增加 `{protocol}` |

### 5.2 路径形态

`protocol` Topic 段：**小写**，仅 `[a-z0-9_-]`，与 JSON `protocol` 枚举一致（§9）。

**形态 A（`protocol_in_topic=false`）**

- 上行（节点作用域）：`{N}/devices/{addr}/messages/events`
- 点抄下行（节点作用域）：`{N}/devices/{addr}/messages/devicebound`

**形态 B（`protocol_in_topic=true`）**

- 上行：`{N}/devices/{addr}/messages/events/{protocol}`
- 点抄下行：`{N}/devices/{addr}/messages/devicebound/{protocol}`

**两种形态相同（不含协议段）**

- `{N}/devices/{addr}/status`
- `{N}/gateway/terminals/list/request|response`
- `{N}/gateway/terminals/presence/request|response`
- `{N}/gateway/health/request|response`
- `{N}/messages/operations/result`
- **`{C}/messages/devicebound`**（**广播**，无 `nodeId` 段）
- **`{C}/gateway/terminals/list/request`** 等（**可选** 集群 RPC 入口）
- `{N}/bridge/status`（可选）

**说明**：`{N}`、`{C}` 定义见 §1.5；凡 **GFEP 发布** 的路径均须带 **`v1/`** 与 `nodes/{nodeId}`。

### 5.3 一致性

- `protocol_in_topic=true` 时：Topic 尾段与 JSON `protocol` **必须一致**；不一致时实现 **拒绝发布并记日志**（推荐），或在 SPEC 中固定唯一策略。
- **`devicebound`**：除 Topic 形态 B 外，GFEP 在写入 TCP 前 **必须** 校验 JSON **`protocol`** 与 **目标终端连接** 的实际规约一致；不一致则 **`PROTOCOL_MISMATCH`**，不写 socket（§4.4、§9.1）。

### 5.4 `protocol_in_topic=true` 时的发布策略

- **推荐**：仅发布带 `{protocol}` 的路径，避免与形态 A 双发；迁移期若双发须在发行说明中声明。

### 5.5 订阅示例

| 需求 | 形态 A | 形态 B |
|------|--------|--------|
| 集群内单节点、全终端全协议上行 | `{N}/devices/+/messages/events` | `.../events/+` |
| 汇聚所有节点上行 | `v1/{root}/{pk}/nodes/+/devices/+/messages/events`（`{pk}` 为空时省略 `{pk}/`） | 同上，形态 B 末尾再加 `/+` |
| 仅某协议上行 | 应用内按 JSON `protocol` 过滤 | `{N}/devices/+/messages/events/698` |

---

## 6. Topic 安全（`nodeId` 与 `addr`）

MQTT 中 `+` `#` `/` 有特殊含义。**`nodeId` 与 `addr` 作为 Topic 段时** 须为安全字符或编码，须约定：

- **推荐**：使用注册 **节点名 / 设备名**（仅安全字符，如 `[a-zA-Z0-9_-]`），与内部标识映射；或
- **单段编码**：如 Base64URL（无填充）作为 Topic 中的一段，文档说明解码规则。

**部署约束**：`mqtt.node_id` 在 **同一 Broker 命名空间与 `v1/{root}/{pk}` 组合下** 必须全局唯一。

---

## 7. 配置项（MQTT）

```text
mqtt.mode                  # client | broker | embedded（以实现为准）
mqtt.broker / mqtt.client  # 地址、端口、TLS、用户、密码、clientId、keepAlive
mqtt.topic_prefix          # {root}；实现拼接 Topic 时 **必须** 前置固定段 v1/
mqtt.product_key           # {pk}，可空
mqtt.node_id               # 集群节点标识 nodeId，必填（启用 MQTT 且参与集群时）
mqtt.protocol_in_topic     # bool，默认 false
mqtt.cluster_rpc           # bool，可选：是否订阅 v1/.../cluster/gateway/.../request（默认以实现为准）
mqtt.max_payload_bytes     # 单条 MQTT JSON 消息最大字节数（含属性消息），默认建议 262144 或与 Broker max_packet_size 一致
```

---

## 8. 非功能需求（MQTT）

| ID | 要求 |
|----|------|
| NFR-MQTT-001 | MQTT 断线自动重连（退避可配置） |
| NFR-MQTT-002 | 关键 MQTT 行为可日志、可监控 |
| NFR-MQTT-003 | 未启用 MQTT 时，不改变现有 TCP 业务默认行为 |
| NFR-MQTT-004 | **`connId`** 在 **单进程生命周期** 内唯一、单调递增（或与实现文档一致）；跨重启 **不** 保证与历史连续，APP **不得** 持久依赖其全局含义 |
| NFR-MQTT-005 | **MQTT 报文大小**：GFEP 发布/订阅的 **单条 MQTT 应用消息**（UTF-8 JSON 字节长）**不得超过** Broker **`max_packet_size`** 与实现 **`mqtt.max_payload_bytes`**（配置，默认建议 **≥ 256 KiB** 或与 Broker 对齐）。**超出时**：**不得** 发布该条；对 **`events`** 可丢弃或改走日志/旁路（实现文档说明）；对 **`devicebound`** 须 **`operations/result`**，`code: PAYLOAD_TOO_LARGE`（§9.1） |

---

## 9. 附录：`protocol` 字符串与内部 ptype 映射（建议）

实现可扩展；与 `zptl/ptl.go` 常量对应关系建议如下（Topic 段使用同字符串）：

| `protocol`（Topic/JSON） | 说明 | 内部常量（参考） |
|--------------------------|------|------------------|
| `376` | 1376.1 | `PTL_1376_1` |
| `698` | 698.45 | `PTL_698_45` |
| `nw` | 南网 2013 | `PTL_NW` |
| `nwm` | 南网 2013（密） | `PTL_NWM` |
| `raw` | 原始报文 | `PTL_RAW` |

其它规约（645、1376.2 等）上线时在本表追加，避免与现网 `protocol` 冲突。

### 9.1 `operations/result` 与网关错误 **`code`（定稿最小集）**

| `code` | 含义 | 典型场景 |
|--------|------|----------|
| `0` | 成功 | 写入完成等 |
| `HEX_INVALID` | `frameHex` 非法 | 奇数长、非法字符、解码失败；**未** 写 TCP |
| `PROTOCOL_MISMATCH` | 规约与连接或 Topic 不一致 | JSON/Topic `protocol` 与终端连接不符；**未** 写 TCP |
| `OFFLINE` | 终端不在线 | 无 TCP 会话 |
| `INVALID_REQUEST` | 请求参数错误 | 如 `protocol`+`protocols` 同时出现 |
| `TIMEOUT` | 超时 | `timeoutMs` 内未完成写入（或实现定义的发送超时） |
| `MQTT_DISCONNECTED` | GFEP 未连上 Broker | 健康检查等 |
| `INTERNAL` | 内部错误 | 未预期异常（**宜** 避免泄露栈信息至 MQTT） |
| `PAYLOAD_TOO_LARGE` | 单条 MQTT 消息超限 | JSON 或解码后体积超过 Broker/实现上限（NFR-MQTT-005） |

实现可扩展更多 `code`，但 **不得** 与上表语义冲突；新增码须写入发行说明。

---

## 10. 验收要点（MQTT）

1. MQTT 在选定模式下：清单、presence（**合并多规约**、离线无多余键、时间与 **上报/流量/条数** 为 **UTC `Z`**）、点抄/广播与 `operations/result` 可观测。
2. 订阅方可收到上行与 `status` 事件；**`events` 含 `payloadEncoding`**；`protocol_in_topic` 开/关与文档路径一致。
3. **下行 `devicebound`**：`frameHex` 合法且 **`protocol` 匹配** 时 **字节级透传**；非法 hex / 规约不符时 **`HEX_INVALID` / `PROTOCOL_MISMATCH`** 且 **不写 TCP**；**`ok:true`** 仅表示 **写 socket 完成**（§2.5）。
4. **集群广播**：无命中终端的节点仍发 **`operations/result`**（`kind: broadcast`，`detail` 三字段，`targetsSent: 0`，§2.5、§2.9）。
5. 两节点以上时 Topic 含 **不同** `nodeId`；**`{C}/messages/devicebound`** 各节点均可收；终端类 GFEP 消息带 **`nodeId`**。
6. 所有对外 Topic **均以 `v1/` 为首段**。
7. 若启用 **`{C}/gateway`** RPC：`targetNodeId` 与「全节点各答一条」行为与 §2.9 一致。
8. **`gateway/health`**：与 §4.7 一致；异常时 `ok: false` 且 **`code`** 符合 §9.1。
9. **广播 `result`**：`detail` 含 **`targetsAttempted` / `targetsSent` / `targetsSkipped`**（§2.5）。
10. **超大载荷**：超过 **NFR-MQTT-005** 时 **`PAYLOAD_TOO_LARGE`** 或等价处理，不破坏 Broker。

---

## 11. 修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-03-28 | 自合并版 SPEC 拆出 |
| 1.1 | 2026-03-28 | 与合并版 §13 对照内容同步为 §12 |
| 1.2 | 2026-03-28 | 多 GFEP 集群：`nodes/{nodeId}`、`cluster/`、`targetNodeId`、载荷 `nodeId` |
| 1.3 | 2026-03-28 | Topic 首段固定 **`v1/`**；引入 `{C}` 集群前缀 |
| 1.4 | 2026-03-28 | presence 增加 `lastReportTime`、`uplinkMsgCount`/`downlinkMsgCount`、`uplinkBytes`/`downlinkBytes` |
| 1.5 | 2026-03-28 | `gateway/terminals/list` 请求支持可选 `protocol` / `protocols` 过滤 |
| 1.6 | 2026-03-28 | §3.3 增加 `gateway/health`；新增 **§13 生产场景与示例报文** |
| 1.7 | 2026-03-28 | 明确点抄/广播为 **`frameHex` 透传**，GFEP 原样下发终端 |
| 1.8 | 2026-03-28 | 新增 **§14 SPEC 自检与需求优化清单** |
| 1.9 | 2026-03-28 | **P0 闭合**：§1.4 时间 UTC、多规约合并；§2.2/§4.1 `protocol` 例外；多连接择最新 login；离线省略键；`frameHex`/规约错误码；透传成功语义；`timeoutMs`；集群广播必发 result；§4.5 `payloadEncoding`；§9.1 `code` 表；NFR `connId` |
| 2.0 | 2026-03-28 | **P1 闭合**：`timeoutMs` 含排队；广播 `detail` 三字段必须；无 `requestId` 行为；NFR-MQTT-005 + `PAYLOAD_TOO_LARGE`；§14.0 更新 |

---

## 12. 与主流 IoT MQTT 平台对照及建议增补

以下对照 **AWS IoT Core**、**Azure IoT Hub**、**阿里云 IoT / 腾讯云 IoT**、**ThingsBoard**、**EMQX** 等常见公开能力；**不表示本期必须实现**，可作为后续版本 backlog。

### 12.1 已较好覆盖（与主流一致或等价）

| 能力 | 主流常见做法 | 本 SPEC |
|------|----------------|---------|
| 设备级数据面 Topic | `devices/{id}/telemetry` 或 `.../messages/events` | **`v1/`** + `{N}/devices/.../events`、`{C}/messages/devicebound` |
| 云到端 / 端到云分流 | 上行、下行 Topic 分离 | 节点作用域 events / devicebound；跨节点广播用 **`{C}/`** |
| 网关侧管理 RPC | HTTP API 或独立 service topic | `{N}/gateway/...`；可选 **`{C}/gateway/...`** |
| 连接态事件 | 在线、LWT、会话状态 | `status`、`bridge/status`（可选） |
| 载荷自描述 | JSON + contentType / 属性 | JSON + `protocol`（终端类必填，§2.2）+ `payloadEncoding`（events） |
| 安全传输与鉴权 | TLS、证书/用户名密码 | §2.1（待细化） |
| Topic 通配与 ACL 思路 | `+`/`#`、前缀权限 | §2.8、§5.5 |

### 12.2 建议增补（按优先级）

#### A. 连接与协议层

| 建议项 | 说明 | 参考来源 |
|--------|------|----------|
| **MQTT 版本** | 明确最低 **3.1.1**；是否支持 **5.0**（Response Topic / Correlation Data 等） | AWS / Azure / EMQX |
| **ClientId 规则** | GFEP 作为客户端时的 `clientId` 格式、唯一性 | 各平台连接文档 |
| **Clean Start / 会话** | 是否持久会话、离线消息堆积上限、与 QoS1 关系 | MQTT 规范、Azure |
| **Keep Alive** | 默认值、允许范围、Broker 与 GFEP 侧超时行为 | 通用 |
| **遗嘱消息（LWT）** | Topic、Payload、`retain` 是否开启、与 `bridge/status` 是否合并 | AWS IoT、EMQX |
| **单消息最大长度** | 超过时丢弃还是拒收并记指标 | Broker 与 GFEP |
| **QoS 策略表** | 各 Topic 类默认 QoS0/1；是否禁止 QoS2 | 项目级决策 |

#### B. 消息信封与可观测

| 建议项 | 说明 |
|--------|------|
| **消息 ID** | 每条发布带 `messageId`（UUID），便于去重、审计 |
| **schemaVersion** | JSON 信封版本号，兼容字段演进 |
| **时间戳统一** | 全链路 **RFC3339** 或 **Unix 毫秒** 二选一定稿 |
| **MQTT 5 Content Type** | 若启用 MQTT 5，可设 `application/json` |
| **关联 ID** | `correlationId` 与 `requestId` 关系 |

#### C. 命令与反馈

| 建议项 | 说明 |
|--------|------|
| **命令超时** | 点抄/广播到 `operations/result` 的默认超时、超时错误码 |
| **部分成功（广播）** | 是否返回 `succeeded[]` / `failed[]` 或计数（可配置上限） |
| **幂等** | 可选 `idempotencyKey` |
| **下行拒绝原因枚举** | 如 `OFFLINE`、`UNKNOWN_ADDR`、`PROTOCOL_MISMATCH`、`PARSE_ERROR` |

#### D. 多租户与规模

| 建议项 | 说明 |
|--------|------|
| **租户/实例隔离** | `{root}` 或额外 `tenantId` 段 |
| **共享订阅** | 多实例 GFEP 是否采用 **`$share`/`queue`** |
| **背压** | 上行超限时丢弃策略、指标 |

#### E. 设备孪生 / 影子

| 建议项 | 说明 |
|--------|------|
| **影子 Topic** | AWS Shadow、Azure Twin 一类；本 SPEC **未覆盖** |
| **是否纳入** | 若仅需最新上报快照，可用 **Retain + 最后一条 telemetry**；若需 desired/reported 分离，另立 SPEC |

#### F. 规则链与外部集成

| 建议项 | 说明 |
|--------|------|
| **HTTP/MQTT 桥接** | 是否内置 Webhook 转发 |
| **数据持久化** | 消息落库、保留时长（部署说明） |

#### G. 合规与安全加固

| 建议项 | 说明 |
|--------|------|
| **证书轮换** | 过期前告警与热更新 |
| **最小权限订阅列表** | GFEP 进程 **subscribe / publish** Topic 白名单（供 Broker ACL） |
| **审计日志** | 谁、何时、对哪类 Topic 发布/订阅 |

### 12.3 建议下一版 SPEC 最小增量（高 ROI）

1. **MQTT 3.1.1 / 5.0** + **Keep Alive / Clean Session / LWT** 默认值表。  
2. **消息信封**：`messageId` + `schemaVersion` + 时间戳格式定稿。  
3. **命令语义**：超时、广播聚合结果策略、**标准错误码表**。  
4. **GFEP MQTT 客户端 Topic 白名单**（subscribe/publish）。  
5. （可选）**MQTT 5**：Response Topic / Correlation Data 与 `requestId` 映射。

**Web 侧实践对照** 见 [WEB_SPEC.md](./WEB_SPEC.md) 第 7 节。

---

## 13. 生产场景：Topic 与报文示例

本节从 **正式生产对接** 角度给出 **完整 Topic 字符串** 与 **示例 JSON**（UTF-8）。占位符与 **§1.5 术语** 下示例一致（`node-a`、`T001` 等）。**时间戳**：规范正文以 **§1.4 UTC `Z`** 为准；本节部分示例仍使用 **`+08:00`** 仅为可读性，**实现输出应统一 UTC**。

| 占位 | 示例取值 |
|------|-----------|
| `{root}` | `gfep/plant` |
| `{pk}` | `tmn` |
| `nodeId` | `node-a` |
| `addr` | `T001` |

**QoS**：未特别声明时，建议 **QoS 1**；**Retain**：除 **GFEP 遗嘱 / 可选 bridge 状态** 外，业务消息默认 **false**。

**说明**：**`devicebound`** 以 **`frameHex` 透传** 为准（§4.4）。**`events` 上行** 可用 hex 或 Base64，见各示例 `payloadEncoding`（§4.5）。`meta` 可由实现扩展（如 AFN、Fn、报文长度）。

---

### 13.1 终端登录、掉线（GFEP → APP）

规约完成 **地址绑定** 视为 **online**；TCP 断开、踢线、超时等视为 **offline**。均由 **GFEP 发布** 至 **状态 Topic**（与遥测 Topic 分离，便于 ACL 与规则引擎）。

#### 13.1.1 上线（login）

| 项 | 值 |
|----|-----|
| **方向** | GFEP → Broker → APP（订阅方） |
| **Topic** | `v1/gfep/plant/tmn/nodes/node-a/devices/T001/status` |
| **QoS** | 1 |

**报文示例**

```json
{
  "nodeId": "node-a",
  "addr": "T001",
  "connId": 1001,
  "protocol": "376",
  "state": "online",
  "ts": "2026-03-28T10:00:02.123+08:00",
  "reason": "login",
  "meta": {
    "remoteAddr": "192.168.1.100:50231"
  }
}
```

#### 13.1.2 掉线（offline）

| 项 | 值 |
|----|-----|
| **方向** | GFEP → Broker → APP |
| **Topic** | `v1/gfep/plant/tmn/nodes/node-a/devices/T001/status`（与上线同 Topic 模式，载荷区分 `state`） |
| **QoS** | 1 |

**报文示例**

```json
{
  "nodeId": "node-a",
  "addr": "T001",
  "connId": 1001,
  "protocol": "376",
  "state": "offline",
  "ts": "2026-03-28T10:15:44.000+08:00",
  "reason": "tcp_close",
  "meta": {
    "detail": "reset by peer"
  }
}
```

**`reason` 建议枚举**（实现可扩展）：`login` | `tcp_close` | `timeout` | `kick` | `replaced` | `shutdown` | `error`。

**APP 订阅建议**：`v1/gfep/plant/tmn/nodes/+/devices/+/status`（全节点全终端状态）。

---

### 13.2 终端数据上报（GFEP → APP）

终端经 GFEP 收到 **一帧完整业务报文** 后，GFEP 转发至 MQTT **事件 Topic**。

| 项 | 值 |
|----|-----|
| **方向** | GFEP → Broker → APP |
| **Topic（形态 A）** | `v1/gfep/plant/tmn/nodes/node-a/devices/T001/messages/events` |
| **Topic（形态 B，`mqtt.protocol_in_topic=true`）** | `.../messages/events/376` |

**报文示例**

```json
{
  "nodeId": "node-a",
  "addr": "T001",
  "connId": 1001,
  "protocol": "376",
  "ts": "2026-03-28T10:05:01.456+08:00",
  "messageId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "payload": "68AAAA...",
  "payloadEncoding": "hex",
  "meta": {
    "byteLen": 48,
    "afn": "0C",
    "fn": "01"
  }
}
```

**生产注意**：高频上报时需 **背压**（实现侧可合并、采样或丢弃策略可配置）；`messageId` 建议 **UUID**，供下游去重与审计。

---

### 13.3 APP 请求指定终端点抄（APP → GFEP → … → operations/result）

APP 向 **持有该终端的节点** 发布 **下行 Topic**；GFEP 将 **`frameHex` 解码后原样写入** 该终端 TCP，然后将执行结果发布至 **结果 Topic**（**不** 在 GFEP 侧按规约重新组帧）。

#### 13.3.1 下行（点抄请求）

| 项 | 值 |
|----|-----|
| **方向** | APP → Broker → GFEP（仅订阅本 `{N}` 的实例） |
| **Topic（形态 A）** | `v1/gfep/plant/tmn/nodes/node-a/devices/T001/messages/devicebound` |
| **Topic（形态 B）** | `.../messages/devicebound/376` |

**报文示例**（`frameHex` 为 APP 已拼好的 **整帧**）

```json
{
  "requestId": "req-7f91-001",
  "protocol": "376",
  "frameHex": "6812000000000000000000000000000000",
  "timeoutMs": 5000,
  "meta": {
    "source": "scada-01"
  }
}
```

#### 13.3.2 执行结果（GFEP → APP）

| 项 | 值 |
|----|-----|
| **方向** | GFEP → Broker → APP |
| **Topic** | `v1/gfep/plant/tmn/nodes/node-a/messages/operations/result` |

**成功示例**

```json
{
  "requestId": "req-7f91-001",
  "nodeId": "node-a",
  "kind": "unicast",
  "addr": "T001",
  "protocol": "376",
  "ok": true,
  "code": "0",
  "message": "sent",
  "ts": "2026-03-28T10:06:00.100+08:00",
  "detail": {
    "bytesSent": 24
  }
}
```

**失败示例**（终端离线）

```json
{
  "requestId": "req-7f91-002",
  "nodeId": "node-a",
  "kind": "unicast",
  "addr": "T001",
  "protocol": "376",
  "ok": false,
  "code": "OFFLINE",
  "message": "terminal not connected",
  "ts": "2026-03-28T10:06:01.000+08:00",
  "detail": {}
}
```

---

### 13.4 APP「响应」终端数据上报（APP → GFEP）

MQTT 上 **没有单独的「应答 Topic」** 绑定到某一帧上行；生产上普遍做法是：APP **消费** §13.2 的 `events` 后，按业务决定是否 **再发一条下行**（点抄同 §13.3）。为 **关联** 上行与下行，建议在下行 JSON 中携带 **`correlationId`**（等于上行 `messageId` 或业务流水号）。

| 项 | 值 |
|----|-----|
| **方向** | APP → GFEP |
| **Topic** | 同 §13.3.1（`devicebound`） |

**报文示例**（响应某次上报；仍为 **透传 hex**，与 §13.3 一致）

```json
{
  "requestId": "req-8aa0-010",
  "protocol": "376",
  "correlationId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "frameHex": "6812000000000000000000000000000000",
  "timeoutMs": 5000,
  "meta": {
    "trigger": "uplink",
    "cause": "reply_to_report",
    "uplinkTs": "2026-03-28T10:05:01.456+08:00"
  }
}
```

GFEP 仍通过 **`operations/result`** 反馈是否已下发（同 §13.3.2）。

---

### 13.5 APP 发起广播（APP → GFEP）

广播走 **集群 Topic**，**每个** GFEP 节点均收到；仅本地有对应终端连接的节点执行下发。

| 项 | 值 |
|----|-----|
| **方向** | APP → Broker → **全体** GFEP |
| **Topic** | `v1/gfep/plant/tmn/cluster/messages/devicebound` |

**报文示例**（同一 `frameHex` 由 **各节点** 对本地在线终端 **各自原样下发**）

```json
{
  "requestId": "req-bc10-900",
  "protocol": "376",
  "frameHex": "6812FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
  "timeoutMs": 8000,
  "meta": {
    "operator": "batch-read-v1",
    "scope": "all_online"
  }
}
```

**结果 Topic**（每节点各自发布一条）

`v1/gfep/plant/tmn/nodes/node-a/messages/operations/result`

**示例**（`kind: broadcast`，可带聚合信息）

```json
{
  "requestId": "req-bc10-900",
  "nodeId": "node-a",
  "kind": "broadcast",
  "protocol": "376",
  "ok": true,
  "code": "0",
  "message": "partial",
  "ts": "2026-03-28T02:10:00.000Z",
  "detail": {
    "targetsAttempted": 120,
    "targetsSent": 118,
    "targetsSkipped": 2
  }
}
```

---

### 13.6 APP 请求 GFEP 健康度（请求 / 响应）

用于 **运维监控、注册发现、大屏**；与 **`bridge/status` 周期心跳**（§3.4）互补：RPC 适合 **按需拉取**，周期发布适合 **订阅型告警**。

#### 13.6.1 请求

| 项 | 值 |
|----|-----|
| **方向** | APP → GFEP |
| **Topic** | `v1/gfep/plant/tmn/nodes/node-a/gateway/health/request` |
| **QoS** | 1 |

**报文示例**

```json
{
  "requestId": "health-req-001",
  "ts": "2026-03-28T10:20:00.000+08:00"
}
```

#### 13.6.2 响应

| 项 | 值 |
|----|-----|
| **方向** | GFEP → APP |
| **Topic** | `v1/gfep/plant/tmn/nodes/node-a/gateway/health/response` |
| **QoS** | 1 |

**报文示例**

```json
{
  "requestId": "health-req-001",
  "nodeId": "node-a",
  "ok": true,
  "ts": "2026-03-28T10:20:00.050+08:00",
  "mqtt": {
    "brokerConnected": true,
    "clientId": "gfep-node-a",
    "lastDisconnectReason": ""
  },
  "process": {
    "uptimeSec": 86400,
    "version": "1.0.0",
    "goVersion": "1.22.x"
  },
  "load": {
    "tcpTerminalConns": 42,
    "tcpAppConns": 3,
    "forwardQueueDepth": 0
  }
}
```

**异常示例**（MQTT 未连上 Broker）

```json
{
  "requestId": "health-req-002",
  "nodeId": "node-a",
  "ok": false,
  "code": "MQTT_DISCONNECTED",
  "message": "not connected to broker",
  "ts": "2026-03-28T10:21:00.000+08:00",
  "mqtt": {
    "brokerConnected": false,
    "lastError": "connection refused"
  }
}
```

---

### 13.7 其它生产级能力（建议一并实现或文档化）

| 序号 | 场景 | Topic / 行为 | 说明 |
|------|------|----------------|------|
| A | **在线清单 / 按协议过滤** | `{N}/gateway/terminals/list/request` → `.../response` | 见 §4.2；集群入口见 `{C}/gateway/...`。 |
| B | **按地址 presence** | `{N}/gateway/terminals/presence/request` → `.../response` | 见 §4.3。 |
| C | **GFEP 进程 LWT** | `{N}/bridge/status` | Broker 遗嘱消息：`state: offline` / `gfep_mqtt_session_lost`，**Retain** 可按运维要求设为 true，便于监控最后已知状态。 |
| D | **错误请求** | 各 `.../response` | `ok: false`，`code` 如 `INVALID_REQUEST` / `TIMEOUT`，`message` 人类可读。 |
| E | **时钟** | 全文 `ts` | 建议 **RFC3339 带时区**；跨节点比较时用 UTC 或统一东八区并在文档声明。 |
| F | **ACL 分层** | 订阅前缀 | 只读账号：仅 `.../events`、`.../status`、`.../result`、`.../response`；运维账号：可加 `health`、`list`。 |
| G | **大报文** | `events` | 超过 Broker **单包限制** 时需分片策略或走旁路（文件/对象存储），在部署手册写明。 |
| H | **幂等与重试** | `devicebound` | 相同 `requestId` 重复发布：实现宜 **幂等** 或返回 `DUPLICATE_REQUEST`（可选字段 `idempotencyKey`）。 |

---

### 13.8 APP 侧订阅 / 发布速查（生产 checklist）

| 动作 | 建议订阅 / 发布 Topic 模式 |
|------|---------------------------|
| 收全网上线/掉线 | `v1/gfep/plant/tmn/nodes/+/devices/+/status` |
| 收全网遥测 | `v1/gfep/plant/tmn/nodes/+/devices/+/messages/events` |
| 收点抄/广播结果 | `v1/gfep/plant/tmn/nodes/+/messages/operations/result` |
| 点抄 | 发布至 `.../nodes/{nodeId}/devices/{addr}/messages/devicebound` |
| 广播 | 发布至 `v1/gfep/plant/tmn/cluster/messages/devicebound` |
| 拉健康 | 发布 `.../nodes/{nodeId}/gateway/health/request`，订阅 `.../health/response`（可与 `requestId` 关联） |

---

## 14. SPEC 自检与需求优化清单

本节对当前 **MQTT_SPEC** 做 **生产向** 复盘：已覆盖能力、未定稿点、与 **§12 backlog** 的关系，以及建议的 **优化优先级**。实施时可按 P0 → P1 → P2 迭代。

### 14.0 已纳入正文

- **v1.9（P0）**：§1.4 UTC、多规约合并；§2.2/§4.1 `protocol` 例外；多连接择最新 login；离线省略键；`HEX_INVALID` / `PROTOCOL_MISMATCH`；透传 `ok`；`timeoutMs` 初稿；集群广播必发 result；§4.5 `payloadEncoding`；§9.1 错误码表；NFR-MQTT-004。
- **v2.0（P1）**：§2.5 **`timeoutMs` 含排队**；广播 **`detail` 三字段必须**；**无 `requestId`** 行为；NFR-MQTT-005、**`PAYLOAD_TOO_LARGE`**（§9.1）。

### 14.1 内部一致性（建议尽快对齐）

| 编号 | 现象 | 建议 |
|------|------|------|
| C-1 | ~~时间格式未定稿~~ | **v1.9 已处理**：§1.4、§4.1；§13 示例仍可有 `+08:00` 注明 |
| C-2 | **§4.1 `protocol` 必填**：表意偏「凡载荷」，但 **APP 的 `health/request`** 可无 `protocol` | **v1.9 已处理**：§2.2、§4.1 例外说明 |
| C-3 | 示例 `nodeId` | **v1.9**：§4 主示例已用 `node-a`；`targetNodeId` 用 `node-b` 表示指向其它节点 |
| C-4 | ~~上行 `events` 编码~~ | **v1.9 已处理**：§4.5、§2.6 |
| C-5 | **广播与 `protocol`** | **v1.9 已处理**：§2.5 单节点内按 `protocol` 过滤 |

### 14.2 未定稿项（影响契约与测试）

| 编号 | 项 | 说明 |
|------|-----|------|
| U-1 | ~~同 `addr` 多连接~~ | **v1.9**：取 **最新 `loginTime`**（并列则较大 `connId`） |
| U-2 | ~~离线态字段~~ | **v1.9**：**省略**键 |
| U-3 | ~~`timeoutMs` 语义~~ | **v2.0**：含排队，见 §2.5 |
| U-4 | ~~`frameHex` 非法~~ | **v1.9**：`HEX_INVALID`，不写 TCP |
| U-5 | **`protocol` 与连接** | **v1.9**：§5.3 + `PROTOCOL_MISMATCH`；形态 B 与 JSON 一致 |
| U-6 | ~~集群广播未命中~~ | **v1.9**：必发 `operations/result` |

### 14.3 与 GFEP 多实例架构的缺口

| 编号 | 项 | 建议 |
|------|-----|------|
| A-1 | ~~单进程多 listen~~ | **v1.9**：§1.4、§2.3、§2.4 合并视图 |
| A-2 | **主站侧（APP→GFEP TCP）与 MQTT** | 若并存，是否 **重复上报**、ACL 如何切分，可在非目标或部署章补一句 |
| A-3 | **`connId` 跨重启** | **v1.9**：NFR-MQTT-004 进程内唯一；跨重启不保证连续 |

### 14.4 命令与结果闭环（生产强化）

| 编号 | 项 | 建议 |
|------|-----|------|
| R-1 | ~~标准错误码表~~ | **v1.9**：§9.1；可继续扩展 |
| R-2 | ~~`requestId` 缺失~~ | **v2.0**：§2.5，省略键；弱关联 |
| R-3 | ~~广播聚合~~ | **v2.0**：§2.5、`detail` 必须字段 |
| R-4 | ~~下行成功定义~~ | **v1.9**：§2.5、§4.4，**已 write** 即 `ok:true` |

### 14.5 可观测、安全与运维（与 §12 衔接）

| 编号 | 项 | 说明 |
|------|-----|------|
| O-1 | **GFEP subscribe/publish 白名单** | §12.2 G 已提；建议单独 **附录表格**（运维配 ACL） |
| O-2 | **MQTT 3.1.1 / 5.0、Keep Alive、LWT** | §12.2 A；与 **`bridge/status` 载荷格式** 绑定成一页「连接参数默认值」 |
| O-3 | ~~消息体大小上限~~ | **v2.0**：NFR-MQTT-005、`PAYLOAD_TOO_LARGE` |
| O-4 | **敏感信息** | `meta.remoteAddr`、错误栈是否默认进 MQTT；建议 **分级** 或配置脱敏 |

### 14.6 文档结构优化（降低读者成本）

| 编号 | 项 | 建议 |
|------|-----|------|
| D-1 | **「规范正文」与「示例」** | §13 已很丰富；可在 §4 各小节加 **「详见 §13.x」** 反向链接 |
| D-2 | **OpenAPI 式总览** | 一页表：**Publish 方向**，**Topic 模板**，**Schema 指针**（便于代码生成） |
| D-3 | **版本迁移** | `v1` → `v2` 时 Topic 并存策略、**弃用周期** |

### 14.7 建议优先级（实施顺序）

| 优先级 | 包含项 | 目的 |
|--------|--------|------|
| **P0** | ~~上表项~~ → **v1.9 已闭合**（见 §14.0） | — |
| **P1** | ~~上表~~ → **v2.0 已闭合**（§14.0） | — |
| **P2** | A-2、A-3、O-1、O-2、O-4、D-1～D-3、§12 其余 | 规模化与长期维护 |
</think>
修正章节引用：将“形态 A/B”指向 §5，将“Topic 安全”指向 §6。

<｜tool▁calls▁begin｜><｜tool▁call▁begin｜>
StrReplace