# TODO

> 代码类发现账本（文档审计波次 5，2026-11-05 记入；只动文档不改代码，代码问题在此登记）。

## 链码（chaincode/playsmith-assets）

- [ ] 鉴权落地：`Mint` 校验 `role=MINTER` 证书属性；`Transfer` 校验调用方为 owner 或已授权 operator（代码内 TODO，`main.go:40` / `main.go:59`）
- [ ] `ListForSale` 校验 seller 余额 ≥ amount（SPEC.md Authorization 一节的 spec target）
- [ ] `Trade` 事件补 `price`/`priceHash` 字段（SPEC.md 承诺携带；当前公共/私有上架均不带）
- [ ] `ask` 公共卖单值改为 JSON（含 `ts`），与私有卖单统一（当前为 `"<price>:<amount>"` 拼接串）

## API Gateway（cmd/api）

- [ ] 修复 `GET /api/v1/assets/{id}/balance/{owner}` 路由解析：`main.go:179` 只在 `len(parts)==4` 时命中 `balance`，实际路径切分后为 3 段，永远 404
- [ ] 接入 Fabric Gateway SDK，替换 `internal/sdk` 的 `NoopClient`（当前所有提交返回 `TX_NOOP`）
- [ ] JWT 鉴权与 gRPC 面（README 路线图项）
- [ ] `market/cancel` 请求体 struct 无 json tag（当前只接受 Go 默认大写键 `TokenID/Seller/ListingID`，与其余端点的小写键风格不一致）

## Indexer / 部署

- [ ] `cmd/indexer` 真实现：订阅链码事件（TransferSingle/TransferBatch/AskCreated/AskCanceled/Trade）并 upsert Postgres（当前为 10s tick 占位）
- [ ] 补 `playsmithPvt` 私有数据集合的 collections 配置 JSON（`ListForSalePrivate`/`CancelListing` 已调用 `PutPrivateData`，无配置文件则部署失败）
