package main

import (
	"context"
	"log"
	"time"

	"github.com/joho/godotenv"

	"github.com/defi-maker/golighter/client"
	"github.com/defi-maker/golighter/examples/internal/shared"
	"github.com/elliottech/lighter-go/types"
	"github.com/elliottech/lighter-go/types/txtypes"
)

func main() {
	_ = godotenv.Load()

	endpoint := shared.Endpoint()
	accountIndex, err := shared.RequireInt64("LIGHTER_ACCOUNT_INDEX")
	if err != nil {
		log.Fatalf("[send-tx-batch] %v", err)
	}
	apiKeyIndex, err := shared.RequireUint8("LIGHTER_API_KEY_INDEX")
	if err != nil {
		log.Fatalf("[send-tx-batch] %v", err)
	}
	apiKeyPrivateKey, err := shared.RequireString("LIGHTER_API_KEY_PRIVATE_KEY")
	if err != nil {
		log.Fatalf("[send-tx-batch] %v", err)
	}
	chainID, err := shared.Uint32OrDefault("LIGHTER_CHAIN_ID", 0)
	if err != nil {
		log.Fatalf("[send-tx-batch] %v", err)
	}

	restClient, err := client.New(endpoint)
	if err != nil {
		log.Fatalf("[send-tx-batch] new client: %v", err)
	}

	txClient, err := client.NewTxClient(restClient, apiKeyPrivateKey, accountIndex, apiKeyIndex, chainID)
	if err != nil {
		log.Fatalf("[send-tx-batch] tx client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := txClient.CheckClient(ctx); err != nil {
		log.Fatalf("[send-tx-batch] check client failed: %v", err)
	}

	orderA, err := txClient.GetCreateOrderTransaction(&types.CreateOrderTxReq{
		MarketIndex:      0,
		ClientOrderIndex: 201,
		BaseAmount:       100000,
		Price:            400000,
		IsAsk:            0,
		Type:             0,
		TimeInForce:      0,
	}, nil)
	if err != nil {
		log.Fatalf("[send-tx-batch] build order A: %v", err)
	}

	orderB, err := txClient.GetCreateOrderTransaction(&types.CreateOrderTxReq{
		MarketIndex:      0,
		ClientOrderIndex: 202,
		BaseAmount:       120000,
		Price:            410000,
		IsAsk:            1,
		Type:             0,
		TimeInForce:      0,
	}, nil)
	if err != nil {
		log.Fatalf("[send-tx-batch] build order B: %v", err)
	}

	resp, err := txClient.SendBatch(ctx, []txtypes.TxInfo{orderA, orderB})
	if err != nil {
		log.Fatalf("[send-tx-batch] send batch: %v", err)
	}

	log.Printf("[send-tx-batch] tx_hash=%v predicted_execution_time_ms=%d", resp.TxHash, resp.PredictedExecutionTimeMs)
}
