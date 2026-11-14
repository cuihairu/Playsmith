---
layout: home
hero:
  name: Playsmith
  text: 许可链上的游戏资产
  tagline: Hyperledger Fabric 上的 ERC-1155 风格铸造、转移与市场，外加事件驱动的 PostgreSQL 投影
  actions:
    - theme: brand
      text: 功能与路由
      link: /features
    - theme: alt
      text: 配置
      link: /configuration
features:
  - title: 一份链码承载账本协议
    details: 铸造、转移、批量、授权、元数据、供应量与上架都在 `playsmith-assets` 链码模块里。复合键（`bal`、`meta`、`supply`、`op`、`ask`）让每次写入只碰极少量键，交易之间不易撞车。
  - title: 两种调用面
    details: 面向游戏服与服务的 JSON REST 网关，加上由 `proto/playsmith/v1/gateway.proto` 生成的可选 gRPC 面。两面都返回交易结果与 Fabric 交易标识。
  - title: 按操作授权
    details: 平台托管的 JWT 声明携带 `mspId` 与 `role`；每个已鉴权调用都必须带齐两者，`Mint`/`BatchMint` 还额外要求 `role=MINTER`——与链码自身的门禁同规则。
  - title: 事件变成可查模型
    details: 索引器消费链码 JSON 事件并投影进 PostgreSQL，Web 与统计查询不再扫描账本。私有上架的价格留在 `playsmithPvt` 集合里。
---

Playsmith 围绕三个进程与一份链码组织：`cmd/api` 对外服务，`cmd/indexer` 维护派生视图，`chaincode/playsmith-assets` 持有账本协议。所有运行时状态都是远端的——账本加 PostgreSQL；没有 Redis、MySQL、进程内持久缓存或应用文件存储。

源码级的解释见[架构解说](/architecture)与[术语表](/glossary)（目前为英文）。
