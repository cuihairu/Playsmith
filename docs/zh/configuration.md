# 配置

所有配置都是进程环境变量加上少量 PEM 文件路径。没有配置文件要改，应用也不持久化
任何本地状态。

## API 网关（`cmd/api`）

| 变量 | 作用 | 读取位置 |
|---|---|---|
| `FABRIC_GATEWAY_ENDPOINT` | 设置时构建真实 Fabric Gateway 客户端；未设置时回退 `NoopClient` 开发模式 | `cmd/api/main.go:230` |
| `FABRIC_MSP_ID` | 网关自身 Fabric 身份的 MSP | `cmd/api/main.go:232` |
| `FABRIC_CERT_PATH` | 该身份的 X.509 证书 | `cmd/api/main.go:233` |
| `FABRIC_KEY_PATH` | 该身份的私钥 | `cmd/api/main.go:234` |
| `FABRIC_TLS_PATH` | Gateway 的 TLS CA 材料 | `cmd/api/main.go:235` |
| `FABRIC_CHANNEL_NAME` | 交易提交到的通道 | `cmd/api/main.go:237` |
| `JWT_SECRET` | 设置（≥32 字节）即开启 JWT；同时是 `POST /api/v1/auth/token` 校验的密钥 | `cmd/api/main.go:252` |
| `JWT_IDENTITY_MAP` | 可选的可信主体映射 `subject:mspId[:role];...`：设置后，校验通过的 claims 必须命中其中一条绑定（subject、mspId，以及绑定中钉住的 role），否则在任何 Fabric 调用之前被拒；签发令牌时同样拒绝未绑定的主体；取值格式非法会直接拒绝启动。未设置表示不校验主体绑定 | `cmd/api/main.go:259`、`cmd/api/main.go:329`、`internal/auth/identity.go:68` |
| `GRPC_PORT` | 启动 gRPC 监听器；未设 `JWT_SECRET` 时拒绝启动 | `cmd/api/main.go:265` |
| `TLS_CERT_PATH` | 与 `TLS_KEY_PATH` **同时**设置时，REST/gRPC 以 TLS 提供服务 | `cmd/api/main.go:501`、`:286` |
| `TLS_KEY_PATH` | 同上；只配一半会回退明文 | `cmd/api/main.go:501`、`:417` |
| `PORT` | HTTP 监听端口，默认 `8080` | `cmd/api/main.go:248-251` |

链码名在 `cmd/api/main.go:236` 固定为 `playsmith-assets`。

部署前需要知道的两个注意点：

- **Fabric 身份是共享的。** 启动时只构造一个 `GatewayClient`，所有调用方都经它
  提交（`cmd/api/main.go:243`）。
  设置了 `JWT_IDENTITY_MAP` 时，它会拦截哪些校验通过的主体有资格行动，但映射
  本身从不挑选不同的证书：按主体选择签名身份仍未落地，缺口登记在 `todo.md:76`。
- **TLS 是全有或全无。** 只有证书/私钥成对齐全时两面才启用 TLS；少配一边就走
  明文，只允许出现在开发环境。

`X-Issue-Secret` 是请求头而非环境变量：签发令牌时必须与 `JWT_SECRET` 相同
（`cmd/api/main.go:252`、`cmd/api/main.go:324-333`）。

## 索引器（`cmd/indexer`）

| 变量 | 作用 | 读取位置 |
|---|---|---|
| `POSTGRES_DSN` | 必填。缺失则启动失败 | `cmd/indexer/main.go:380` |
| `FABRIC_GATEWAY_ENDPOINT` | 事件流使用的 Fabric 端点 | `cmd/indexer/main.go:452` |
| `FABRIC_CERT_PATH` | 身份证书 | `cmd/indexer/main.go:453` |
| `FABRIC_KEY_PATH` | 身份私钥 | `cmd/indexer/main.go:454` |
| `FABRIC_TLS_PATH` | Gateway TLS 材料 | `cmd/indexer/main.go:455` |
| `FABRIC_CHANNEL_NAME` | 订阅的通道 | `cmd/indexer/main.go:456` |
| `FABRIC_MSP_ID` | 订阅身份的 MSP | `cmd/indexer/main.go:457` |

投影 schema 是启动时幂等执行的内嵌 DDL（`cmd/indexer/main.go:30`），没有独立
迁移文件。

## 部署契约

`deploy/collections.json` 是唯一的部署输入：它声明 `playsmithPvt` 集合、两组织的
读写成员、传播法定人数、生命周期与背书策略。本地 fabric-samples/test-network 的
演练见 `deploy/README.md`。
