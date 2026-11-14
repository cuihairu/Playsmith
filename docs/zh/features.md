# 功能与路由

本页只列真实存在的路由。REST 表取自网关路由表 `cmd/api/main.go:282-296`，
所有路由都包在鉴权中间件里；请求字段名是 `cmd/api/main.go:33-86` 那组 struct 的
JSON tag。

## REST 路由

| 方法 | 路径 | 请求体 | 链码操作 |
|---|---|---|---|
| POST | `/api/v1/auth/token` | `{ subject, mspId, role }` | 无——签发 JWT，由 `X-Issue-Secret` 把关 |
| POST | `/api/v1/assets/mint` | `{ tokenId, to, uri, amount }` | `Mint`——JWT 需 `role=MINTER` |
| POST | `/api/v1/assets/batchMint` | `{ ids, tos, amounts, uris }` | `BatchMint`——JWT 需 `role=MINTER` |
| POST | `/api/v1/assets/transfer` | `{ tokenId, from, to, amount }` | `Transfer` |
| POST | `/api/v1/assets/batchTransfer` | `{ ids, from, to, amounts }` | `BatchTransfer` |
| POST | `/api/v1/assets/approveAll` | `{ owner, operator, approved }` | `SetApprovalForAll` |
| POST | `/api/v1/market/list` | `{ tokenId, seller, amount, price }` | `ListForSale` |
| POST | `/api/v1/market/list-private` | `{ tokenId, seller, amount, price }` | `ListForSalePrivate`——价格经 transient 传递 |
| POST | `/api/v1/market/cancel` | `{ tokenId, seller, listingId }` | `CancelListing` |
| POST | `/api/v1/market/buy` | `{ tokenId, seller, listingId, buyer, amount }` | `Buy` |
| GET | `/api/v1/assets/{tokenId}/uri` | — | `URI` |
| GET | `/api/v1/assets/{tokenId}/exists` | — | `Exists` |
| GET | `/api/v1/assets/{tokenId}/totalSupply` | — | `TotalSupply` |
| GET | `/api/v1/assets/{tokenId}/balance/{owner}` | — | `BalanceOf` |

四个资产读接口共用一个前缀处理器：`cmd/api/main.go:428` 的 `assetRoute` 按
`/api/v1/assets/` 之后的路径段分发。卖单/成交检索（listings 浏览）、Web UI 与
Caliper 类压测都建在索引器投影之上，目前尚未成为路由。

提交成功写 `{ "result": ..., "txId": "..." }`；失败写 HTTP 500 加
`{ "error": "..." }`（`cmd/api/main.go:214-222`）。上游错误目前尚未映射为
400/404——那套映射是 `docs/SPEC.md` 里的目标，不是当前行为。

## gRPC 面

设了 `GRPC_PORT` 后会在同一个 `Server` 上起第二个监听器。协议是
`proto/playsmith/v1/gateway.proto`，生成绑定放在 `internal/gatewaypb/gen`。
它镜像 REST 面，返回交易结果与 `tx_id`——即使链码没有返回值，调用也能被追踪。
区块号与事件回执属规划项，当前响应只有 `tx_id` 和 `result`。`GRPC_PORT` 要求
同时设 `JWT_SECRET`，否则进程拒绝启动。

## 账本协议

状态写在复合键下，键空间的分片把热点压得很窄：

| 键 | 值 |
|---|---|
| `bal~{id}~{owner}` | uint64 余额，十进制字符串 |
| `meta~{id}` | `{ tokenId, uri, creator, supplyCap? }` |
| `supply~{id}` | uint64 总供应量 |
| `op~{owner}~{operator}` | 授权 `"1"` / `"0"` |
| `ask~{id}~{seller}~{listingId}` | `{ amount, price?, priceHash?, ts }` |
| `askpvt~{id}~{seller}~{listingId}` | 集合 `playsmithPvt` 内的 `{ price, ts }` |

铸造要求 X.509 属性 `role=MINTER`；转移要求所有者或已授权 operator 权限；上架要求
卖家持有不少于挂单数量。私有上架把价格哈希进公开状态、原值留在集合里，因此结算
证据不会离开交易方组织。

链码事件（`TransferSingle`、`TransferBatch`、`ApprovalForAll`、`URI`、
`AskCreated`、`AskCanceled`、`Trade`）是索引器的变更源，投影方式见
[架构解说](/architecture#5-event-projection)（英文）。
