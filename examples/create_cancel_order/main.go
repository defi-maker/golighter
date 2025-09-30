package main

import (
	"context"
	"log"
	"time"

	"github.com/joho/godotenv"

	"github.com/defi-maker/golighter/client"
	"github.com/defi-maker/golighter/examples/internal/shared"
	"github.com/elliottech/lighter-go/types"
)

func main() {
	_ = godotenv.Load()

	endpoint := shared.Endpoint()
	accountIndex, err := shared.RequireInt64("LIGHTER_ACCOUNT_INDEX")
	if err != nil {
		log.Fatalf("[create-cancel-order] %v", err)
	}
	apiKeyIndex, err := shared.RequireUint8("LIGHTER_API_KEY_INDEX")
	if err != nil {
		log.Fatalf("[create-cancel-order] %v", err)
	}
	apiKeyPrivateKey, err := shared.RequireString("LIGHTER_API_KEY_PRIVATE_KEY")
	if err != nil {
		log.Fatalf("[create-cancel-order] %v", err)
	}
	chainID, err := shared.Uint32OrDefault("LIGHTER_CHAIN_ID", 0)
	if err != nil {
		log.Fatalf("[create-cancel-order] %v", err)
	}

	restClient, err := client.New(endpoint, client.WithChannelName("golighter-examples"))
	if err != nil {
		log.Fatalf("[create-cancel-order] new client: %v", err)
	}

	txClient, err := client.NewTxClient(restClient, apiKeyPrivateKey, accountIndex, apiKeyIndex, chainID)
	if err != nil {
		log.Fatalf("[create-cancel-order] tx client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := txClient.CheckClient(ctx); err != nil {
		log.Fatalf("[create-cancel-order] check client failed: %v", err)
	}

	log.Printf("[create-cancel-order] placing order on %s", endpoint)
	createReq := &types.CreateOrderTxReq{
		MarketIndex:      0,
		ClientOrderIndex: 123,
		BaseAmount:       100000,
		Price:            405000,
		IsAsk:            1,
		Type:             0,
		TimeInForce:      0,
		ReduceOnly:       0,
		TriggerPrice:     0,
	}

	createTx, err := txClient.GetCreateOrderTransaction(createReq, nil)
	if err != nil {
		log.Fatalf("[create-cancel-order] build create order tx: %v", err)
	}

	txHash, err := txClient.SendRawTx(ctx, createTx, nil)
	if err != nil {
		log.Fatalf("[create-cancel-order] submit create order tx: %v", err)
	}
	log.Printf("[create-cancel-order] order submitted tx_hash=%s", txHash)

	time.Sleep(2 * time.Second)

	cancelTx, err := txClient.GetCancelOrderTransaction(&types.CancelOrderTxReq{MarketIndex: createReq.MarketIndex, Index: createReq.ClientOrderIndex}, nil)
	if err != nil {
		log.Fatalf("[create-cancel-order] build cancel order tx: %v", err)
	}

	cancelHash, err := txClient.SendRawTx(ctx, cancelTx, nil)
	if err != nil {
		log.Fatalf("[create-cancel-order] submit cancel order tx: %v", err)
	}

	log.Printf("[create-cancel-order] cancel submitted tx_hash=%s", cancelHash)
}
