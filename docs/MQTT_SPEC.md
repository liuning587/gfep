# GFEP：MQTT 集成 — 需求规格（SPEC）

| 项 | 内容 |
|---|------|
| 状态 | Draft |
| 版本 | 1.1 |
| 关联 | [Web 日志下载 SPEC](./WEB_SPEC.md) |

---

## 1. 背景与目标

### 1.1 背景

GFEP 作为网关/前置，已具备多规约终端接入与报文处理能力。需为上层 APP/平台提供 **MQTT** 能力：查询、控制、订阅。

### 1.2 目标

通过 MQTT 暴露终端在线查询、按地址查询在线与活动时间、JSON 点抄/广播、订阅上行报文与上下线事件；MQTT 工作模式可为 **客户端连接外部 Broker** 或 **Broker/嵌入式 Broker**（可配置，见 §3）。

### 1.3 非目标（本期或另立 SPEC）

- 完整 MQTT Broker 集群、多租户计费。
- 设备孪生/影子（详见 §13）；规则引擎、持久化队列等平台侧能力（除非另立 SPEC）。

### 1.4 术语

| 术语 | 说明 |
|------|------|
| 终端地址 `addr` | 业务侧标识终端的字符串；Topic 中使用时须 **Topic 安全**（§6） |
| 点抄 | 针对单个终端地址的下行 |
| 广播 | 面向多个或全部在线终端的下行（语义与现网路由一致） |
| `{root}` | 配置项 `mqtt.topic_prefix`，无尾 `/` |
| `{pk}` | 配置项 `mqtt.product_key`；为空时路径中 **不写** `{pk}/` 段（实现须规范拼接，避免 `//`） |

---

## 2. MQTT 功能需求

### 2.1 工作模式（FR-MQTT-001）

- 系统 **必须** 支持通过配置选择至少一种：**客户端连外部 Broker**，或 **本机/同部署 Broker（含嵌入式）**。
- **必须** 支持 TLS（可开关）、用户名密码或证书等 **一种以上** 鉴权方式（细节在实现与运维文档中列出）。

### 2.2 载荷约定（FR-MQTT-002）

- 载荷 **UTF-8 JSON**。
- RPC 建议带 `requestId`；响应 **必须** 回显相同 `requestId`（若请求携带）。
- **协议类型**：每条业务消息 JSON **必须** 含 `protocol`（字符串，§9）；可选在 Topic 增加协议段（§5）。

### 2.3 在线终端清单（FR-MQTT-010）

- APP **必须** 能查询当前在线终端清单（至少含 `addr`；可选 `connId`、协议类型等）。
- 超时/错误返回可解析 JSON（`ok`/`code`/`message`）。

### 2.4 按地址查询在线与活动时间（FR-MQTT-011）

- **必须** 支持按 **一个或多个** `addr` 查询是否在线。
- 当 **在线** 时，**必须** 在同条响应中返回下列时间（全系统统一 **RFC3339** 或 **Unix 毫秒**，二选一定稿）：

| JSON 字段 | 语义 | 与连接层对应（实现参考） |
|-----------|------|---------------------------|
| `connTime` | TCP 连接建立时间 | `Connection.ctime` |
| `loginTime` | 终端登录/地址绑定成功时间 | `Connection.ltime` |
| `heartbeatTime` | 最近一次协议心跳时间 | `Connection.htime` |
| `lastRxTime` | 最近一次 **收到** 终端报文时间 | `Connection.rtime` |
| `lastTxTime` | 最近一次 **向终端发送成功** 时间 | 若未实现则键保留、值为 `null` |

- **离线**：`online: false`；时间字段 **省略或全部为 `null`**（二选一并写死，不得与历史会话混用）。
- 同一 `addr` 多连接时：**必须** 约定返回单条主连接或 `sessions[]`（含 `connId` 与上述字段）。

### 2.5 JSON 点抄与广播（FR-MQTT-012）

- **必须** 支持点抄至指定 `addr`、广播至允许范围。
- **必须** 有结果反馈（统一结果 Topic，§3.3），含 `kind: unicast|broadcast` 与成功/失败信息。

### 2.6 订阅终端上行（FR-MQTT-020）

- **必须** 可订阅终端上行（原始或解析形态由实现定稿：`payload` Base64 与/或 `parsed`）。
- QoS 建议 **1**；Retain 默认 **关闭**。

### 2.7 订阅上下线（FR-MQTT-021）

- **必须** 可订阅上线/下线事件；重连风暴 **宜** 防抖或可配置合并策略。

### 2.8 其它（FR-MQTT-030）

- **宜** 暴露 MQTT 连接状态指标、日志；**宜** 支持按 Topic/凭证的 ACL（只读 vs 可下发）。

---

## 3. Topic 总览（主流 IoT 风格）

对齐常见「设备为中心」数据面（类 Azure IoT Hub `devices/.../messages/...`）+ **网关管理** 分支。

### 3.1 数据面 — 上行事件

| 语义 | GFEP 发布（APP 订阅） |
|------|------------------------|
| 终端上行 | 见 §5：形态 A / B |
| 全终端上行 | `.../devices/+/messages/events` 或形态 B 下 `.../devices/+/messages/events/+` |
| 连接状态 | `{root}/{pk}/devices/{addr}/status` |
| 全终端状态 | `{root}/{pk}/devices/+/status` |

### 3.2 数据面 — 下行命令

| 语义 | APP 发布（GFEP 订阅） |
|------|------------------------|
| 点抄 | 见 §5：形态 A / B |
| 广播（固定，不按设备分） | `{root}/{pk}/messages/devicebound` |

### 3.3 结果与网关 RPC

| 语义 | 方向 | Topic |
|------|------|--------|
| 命令执行结果 | GFEP → APP | `{root}/{pk}/messages/operations/result` |
| 在线清单请求/响应 | APP ↔ GFEP | `{root}/{pk}/gateway/terminals/list/request` · `.../response` |
| 在线与时间查询 | APP ↔ GFEP | `{root}/{pk}/gateway/terminals/presence/request` · `.../response` |

### 3.4 系统（可选）

| 用途 | Topic |
|------|--------|
| 网关 MQTT 健康 / LWT | `{root}/{pk}/bridge/status` |

---

## 4. JSON 消息定稿

### 4.1 通用字段

| 字段 | 说明 |
|------|------|
| `requestId` | RPC 关联用，可选；若带则响应必填回显 |
| `ts` | 服务端时间，RFC3339，可选 |
| `ok` / `code` / `message` | 成功失败与错误信息 |
| `protocol` | 协议字符串，**必填**（§9） |

### 4.2 `gateway/terminals/list`

**request**（可为 `{}`）

```json
{ "requestId": "550e8400-e29b-41d4-a716-446655440000" }
```

**response**

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
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
  "addrs": ["T001", "T002"]
}
```

**response**

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "ok": true,
  "results": [
    {
      "addr": "T001",
      "online": true,
      "connId": 123,
      "protocol": "376",
      "connTime": "2026-03-28T08:00:01Z",
      "loginTime": "2026-03-28T08:00:02Z",
      "heartbeatTime": "2026-03-28T08:05:00Z",
      "lastRxTime": "2026-03-28T08:05:01Z",
      "lastTxTime": "2026-03-28T08:05:00Z"
    },
    { "addr": "T002", "online": false }
  ]
}
```

### 4.4 下行点抄 / 广播

**点抄（publish 至 devicebound Topic，§5）**

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "protocol": "376",
  "payload": {}
}
```

**广播**

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "protocol": "376",
  "payload": {}
}
```

**`messages/operations/result`**

```json
{
  "requestId": "550e8400-e29b-41d4-a716-446655440000",
  "kind": "unicast",
  "addr": "T001",
  "protocol": "376",
  "ok": true,
  "code": "0",
  "message": "",
  "detail": {}
}
```

`kind`：`unicast` | `broadcast`；广播时 `addr` 可省略或 `"*"`。

### 4.5 `devices/.../messages/events`（上行）

```json
{
  "addr": "T001",
  "connId": 123,
  "protocol": "376",
  "ts": "2026-03-28T08:05:01Z",
  "payload": "base64…",
  "meta": {}
}
```

### 4.6 `devices/.../status`（上下线）

```json
{
  "addr": "T001",
  "connId": 123,
  "protocol": "376",
  "state": "online",
  "ts": "2026-03-28T08:00:02Z",
  "reason": ""
}
```

`state`：`online` | `offline`。

---

## 5. 协议：JSON 必填 + 可选 Topic 形态

### 5.1 配置

| 配置项 | 类型 | 默认 | 说明 |
|--------|------|------|------|
| `mqtt.protocol_in_topic` | bool | `false` | `false`：仅 JSON 带 `protocol`；`true`：在下方 §5.2 路径末尾增加 `{protocol}` |

### 5.2 路径形态

`protocol` Topic 段：**小写**，仅 `[a-z0-9_-]`，与 JSON `protocol` 枚举一致（§9）。

**形态 A（`protocol_in_topic=false`）**

- 上行：`{root}/{pk}/devices/{addr}/messages/events`
- 点抄下行：`{root}/{pk}/devices/{addr}/messages/devicebound`

**形态 B（`protocol_in_topic=true`）**

- 上行：`{root}/{pk}/devices/{addr}/messages/events/{protocol}`
- 点抄下行：`{root}/{pk}/devices/{addr}/messages/devicebound/{protocol}`

**两种形态相同（不含协议段）**

- `{root}/{pk}/devices/{addr}/status`
- `{root}/{pk}/gateway/terminals/list/request|response`
- `{root}/{pk}/gateway/terminals/presence/request|response`
- `{root}/{pk}/messages/operations/result`
- `{root}/{pk}/messages/devicebound`（广播）
- `{root}/{pk}/bridge/status`（可选）

### 5.3 一致性

- `protocol_in_topic=true` 时：Topic 尾段与 JSON `protocol` **必须一致**；不一致时实现 **拒绝发布并记日志**（推荐），或在 SPEC 中固定唯一策略。

### 5.4 `protocol_in_topic=true` 时的发布策略

- **推荐**：仅发布带 `{protocol}` 的路径，避免与形态 A 双发；迁移期若双发须在发行说明中声明。

### 5.5 订阅示例

| 需求 | 形态 A | 形态 B |
|------|--------|--------|
| 全终端全协议上行 | `.../devices/+/messages/events` | `.../devices/+/messages/events/+` |
| 仅某协议上行 | 应用内按 JSON `protocol` 过滤 | `.../devices/+/messages/events/698` |

---

## 6. Topic 安全（`addr`）

MQTT 中 `+` `#` `/` 有特殊含义。`addr` 若含非法字符，须约定：

- **推荐**：对外使用注册 **设备名**（仅安全字符），与内部 `addr` 映射；或
- **单段编码**：如 Base64URL（无填充）作为 Topic 中的一段，文档说明解码规则。

---

## 7. 配置项（MQTT）

```text
mqtt.mode                  # client | broker | embedded（以实现为准）
mqtt.broker / mqtt.client  # 地址、端口、TLS、用户、密码、clientId、keepAlive
mqtt.topic_prefix          # {root}
mqtt.product_key           # {pk}，可空
mqtt.protocol_in_topic     # bool，默认 false
```

---

## 8. 非功能需求（MQTT）

| ID | 要求 |
|----|------|
| NFR-MQTT-001 | MQTT 断线自动重连（退避可配置） |
| NFR-MQTT-002 | 关键 MQTT 行为可日志、可监控 |
| NFR-MQTT-003 | 未启用 MQTT 时，不改变现有 TCP 业务默认行为 |

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

---

## 10. 验收要点（MQTT）

1. MQTT 在选定模式下：清单、presence（含时间字段语义）、点抄/广播与 `operations/result` 可观测。
2. 订阅方可收到上行与 `status` 事件；`protocol_in_topic` 开/关与文档路径一致。
3. 任意上行/下行 JSON 均带 `protocol`，且与 Topic 协议段（若启用）一致。

---

## 11. 修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-03-28 | 自合并版 SPEC 拆出 |
| 1.1 | 2026-03-28 | 与合并版 §13 对照内容同步为 §12 |

---

## 12. 与主流 IoT MQTT 平台对照及建议增补

以下对照 **AWS IoT Core**、**Azure IoT Hub**、**阿里云 IoT / 腾讯云 IoT**、**ThingsBoard**、**EMQX** 等常见公开能力；**不表示本期必须实现**，可作为后续版本 backlog。

### 12.1 已较好覆盖（与主流一致或等价）

| 能力 | 主流常见做法 | 本 SPEC |
|------|----------------|---------|
| 设备级数据面 Topic | `devices/{id}/telemetry` 或 `.../messages/events` | `devices/{addr}/messages/events`、`devicebound` |
| 云到端 / 端到云分流 | 上行、下行 Topic 分离 | events / devicebound / operations/result |
| 网关侧管理 RPC | HTTP API 或独立 service topic | `gateway/.../request|response` |
| 连接态事件 | 在线、LWT、会话状态 | `status`、`bridge/status`（可选） |
| 载荷自描述 | JSON + contentType / 属性 | JSON + 必填 `protocol` |
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
</think>
修正章节引用：将“形态 A/B”指向 §5，将“Topic 安全”指向 §6。

<｜tool▁calls▁begin｜><｜tool▁call▁begin｜>
StrReplace