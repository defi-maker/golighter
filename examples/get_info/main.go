package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	lighterapi "github.com/defi-maker/golighter/api"
	"github.com/defi-maker/golighter/client"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("[demo] unable to load .env automatically: %v", err)
	}

	endpoint := os.Getenv("LIGHTER_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://mainnet.zklighter.elliot.ai"
	}

	ctx := context.Background()

	httpClient, err := client.New(endpoint)
	if err != nil {
		log.Fatalf("[demo] invalid endpoint %q: %v", endpoint, err)
	}

	status, err := httpClient.Status(ctx)
	if err != nil {
		log.Fatalf("[demo] get status failed: %v", err)
	}
	log.Printf("[demo] status=%d network=%d timestamp=%d", status.Status, status.NetworkId, status.Timestamp)

	blocks, err := httpClient.Blocks(ctx, &lighterapi.BlocksParams{Limit: 5})
	if err != nil {
		log.Fatalf("[demo] get blocks failed: %v", err)
	}
	if len(blocks.Blocks) > 0 {
		latest := blocks.Blocks[len(blocks.Blocks)-1]
		log.Printf("[demo] latest block height=%d commitment=%s", latest.Height, latest.Commitment)

		blockByHeight, err := httpClient.Block(ctx, &lighterapi.BlockParams{By: "height", Value: fmt.Sprint(latest.Height)})
		if err != nil {
			log.Printf("[demo] get block by height failed: %v", err)
		} else {
			log.Printf("[demo] matched %d block(s) by height", len(blockByHeight.Blocks))
		}
	}

	if orderBook, err := httpClient.OrderBookOrders(ctx, &lighterapi.OrderBookOrdersParams{MarketId: 1, Limit: 10}); err != nil {
		log.Printf("[demo] get order book failed: %v", err)
	} else {
		log.Printf("[demo] order book snapshot bids=%d asks=%d", len(orderBook.Bids), len(orderBook.Asks))
	}

	if accountIndex, apiKeyIndex, ok := accountContext(); ok {
		nonce, err := httpClient.NextNonceValue(ctx, accountIndex, apiKeyIndex)
		if err != nil {
			log.Printf("[demo] get next nonce failed: %v", err)
		} else {
			log.Printf("[demo] next nonce for account=%d key=%d is %d", accountIndex, apiKeyIndex, nonce)
		}
	} else {
		log.Printf("[demo] account context not configured, skipping private endpoints")
	}
}

func accountContext() (int64, uint8, bool) {
	accountIndexStr := os.Getenv("LIGHTER_ACCOUNT_INDEX")
	apiKeyIndexStr := os.Getenv("LIGHTER_API_KEY_INDEX")
	if accountIndexStr == "" || apiKeyIndexStr == "" {
		return 0, 0, false
	}

	accountIndex, err := strconv.ParseInt(accountIndexStr, 10, 64)
	if err != nil {
		log.Printf("[demo] invalid LIGHTER_ACCOUNT_INDEX: %v", err)
		return 0, 0, false
	}

	apiKeyIndexValue, err := strconv.ParseUint(apiKeyIndexStr, 10, 8)
	if err != nil {
		log.Printf("[demo] invalid LIGHTER_API_KEY_INDEX: %v", err)
		return 0, 0, false
	}

	return accountIndex, uint8(apiKeyIndexValue), true
}
