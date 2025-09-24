# golighter

A thin wrapper library around github.com/elliottech/lighter-go that extracts additional functionality (HTTP endpoints and WebSocket client/services)

## HTTP 客户端常用接口

`client.HTTPClient` 封装了 OpenAPI 中最常用的 REST 接口，涵盖行情、账户、系统信息以及桥接等场景。下面给出初始化和部分接口的调用示例：

```go
package main

import (
    "log"

    "github.com/defi-maker/golighter/client"
)

func main() {
    httpClient := client.NewHTTPClient("https://mainnet.zklighter.elliot.ai")

    status, err := httpClient.GetStatus()
    if err != nil {
        log.Fatalf("get status failed: %v", err)
    }
    log.Printf("status=%d network=%d at=%d", status.Status, status.NetworkID, status.Timestamp)

    blocks, err := httpClient.GetBlocks(10, nil, nil)
    if err != nil {
        log.Fatalf("get blocks failed: %v", err)
    }
    log.Printf("latest block commitment=%s", blocks.Blocks[0].Commitment)

    orders, err := httpClient.GetOrderBookOrders(1, 50)
    if err != nil {
        log.Fatalf("get order book orders failed: %v", err)
    }
    log.Printf("asks=%d bids=%d", orders.TotalAsks, orders.TotalBids)

    referral, err := httpClient.GetReferralPoints(12345, nil)
    if err != nil {
        log.Fatalf("get referral points failed: %v", err)
    }
    log.Printf("total points=%d", referral.UserTotalPoints)
}
```

> 若需要从 `.env` 加载认证信息，请使用 `dotenv`（或兼容工具）运行示例，例如：
>
> ```bash
> dotenv -f .env -- go run examples/http/main.go
> ```

完整示例代码见 `examples/http/main.go`。

### 市场做市示例

- `examples/market_maker/main.go` 复刻了 `LIGHTER_Market_Making/market_maker.py` 的核心逻辑，包括 Avellaneda-Stoikov 报价、动态下单、自动撤单与仓位跟踪。
- 运行前请准备好 `.env` 文件（私钥、账户/Key 索引、可选的 Avellaneda 参数路径等），然后执行：

```bash
go run examples/market_maker/main.go
```

- 如需自动加载 `.env`，可以使用 `dotenv -f .env -- go run examples/market_maker/main.go`。

### 功能概览

- 系统状态：`GetStatus`、`GetSystemInfo`、`GetSystemStatus`、`GetAnnouncements`。
- 区块与交易：`GetBlocks`、`GetBlock`（使用 `client.BlockQueryType` 常量）、`GetCurrentHeight`、`GetBlockTxs`、`GetTxFromL1TxHash`。
- 行情数据：`GetOrderBookOrders` 获取盘口挂单快照。
- 桥接与资产：`GetFastBridgeInfo`、`GetTransferFeeInfo`、`SendRawTx` / `SendTxBatch`、`GetWithdrawalDelay`。
- 账户资料：`GetAccount`、`GetAccountByL1Address`、`GetL1Metadata`、`GetAccountLimits`、`GetAccountMetadata`、`AckNotification`（通知确认）。
- 佣金/积分：`GetReferralPoints` 返回当前账户的积分统计。

大部分接口都接受可选参数（`*string`/`*int64` 等），仅在您传入非 `nil` 时才会包含在查询字符串中。
