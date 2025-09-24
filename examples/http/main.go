package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

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

	httpClient := client.NewHTTPClient(endpoint)
	if httpClient == nil {
		log.Fatalf("[demo] invalid endpoint: %s", endpoint)
	}

	status, err := httpClient.GetStatus()
	if err != nil {
		log.Fatalf("[demo] get status failed: %v", err)
	}
	log.Printf("[demo] status=%d network=%d timestamp=%d", status.Status, status.NetworkID, status.Timestamp)

	blocks, err := httpClient.GetBlocks(5, nil, nil)
	if err != nil {
		log.Fatalf("[demo] get blocks failed: %v", err)
	}
	if len(blocks.Blocks) > 0 {
		latest := blocks.Blocks[len(blocks.Blocks)-1]
		log.Printf("[demo] latest block height=%d commitment=%s", latest.Height, latest.Commitment)

		blockByHeight, err := httpClient.GetBlock(client.BlockQueryByHeight, fmt.Sprint(latest.Height))
		if err != nil {
			log.Printf("[demo] get block by height failed: %v", err)
		} else {
			log.Printf("[demo] matched %d block(s) by height", len(blockByHeight.Blocks))
		}
	}

	if orderBook, err := httpClient.GetOrderBookOrders(1, 10); err != nil {
		log.Printf("[demo] get order book failed: %v", err)
	} else {
		log.Printf("[demo] order book snapshot bids=%d asks=%d", len(orderBook.Bids), len(orderBook.Asks))
	}

	if token, accountIdx, ok := authToken(httpClient); ok {
		if referral, err := httpClient.GetReferralPoints(accountIdx, &token); err != nil {
			log.Printf("[demo] get referral points failed: %v", err)
		} else {
			log.Printf("[demo] referral total=%.2f last_week=%.2f", referral.UserTotalPoints, referral.UserLastWeekPoints)
		}
	} else {
		log.Printf("[demo] auth variables not found, skipping private endpoints (set LIGHTER_SECRET, LIGHTER_ACCOUNT_INDEX, LIGHTER_API_KEY_INDEX)")
	}
}

func authToken(httpClient *client.HTTPClient) (string, int64, bool) {
	secret := os.Getenv("LIGHTER_SECRET")
	accountIndexStr := os.Getenv("LIGHTER_ACCOUNT_INDEX")
	apiKeyIndexStr := os.Getenv("LIGHTER_API_KEY_INDEX")

	if secret == "" || accountIndexStr == "" || apiKeyIndexStr == "" {
		return "", 0, false
	}

	accountIndex, err := strconv.ParseInt(accountIndexStr, 10, 64)
	if err != nil {
		log.Printf("[demo] invalid LIGHTER_ACCOUNT_INDEX: %v", err)
		return "", 0, false
	}

	apiKeyIndexValue, err := strconv.ParseUint(apiKeyIndexStr, 10, 8)
	if err != nil {
		log.Printf("[demo] invalid LIGHTER_API_KEY_INDEX: %v", err)
		return "", 0, false
	}

	txClient, err := client.NewTxClient(httpClient, secret, accountIndex, uint8(apiKeyIndexValue), 0)
	if err != nil {
		log.Printf("[demo] new tx client failed: %v", err)
		return "", 0, false
	}

	token, err := txClient.GetAuthToken(time.Now().Add(5 * time.Minute))
	if err != nil {
		log.Printf("[demo] build auth token failed: %v", err)
		return "", 0, false
	}

	return token, accountIndex, true
}
