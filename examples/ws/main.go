package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/defi-maker/golighter/client"
	"github.com/defi-maker/golighter/examples/internal/shared"
)

func main() {
	_ = godotenv.Load()

	endpoint := shared.Endpoint()
	wsURL := strings.Replace(endpoint, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)
	if !strings.HasSuffix(wsURL, "/stream") {
		wsURL = strings.TrimRight(wsURL, "/") + "/stream"
	}

	cfg := client.DefaultWSConfig()
	cfg.URL = wsURL

	wsClient := client.NewLighterWebsocketClient().SetConfig(cfg)

	publicSvc, err := wsClient.Public()
	if err != nil {
		log.Fatalf("[ws] create public service failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := publicSvc.Start(ctx, func(err error) {
		if err != nil {
			log.Printf("[ws] error: %v", err)
		}
	}); err != nil {
		log.Fatalf("[ws] start service failed: %v", err)
	}

	unsubscribe, err := publicSvc.SubscribeOrderBook(client.LighterOrderBookParamKey{MarketId: 0}, func(resp client.LighterOrderBookResponse) error {
		log.Printf("[ws] order book market=%d bids=%d asks=%d", resp.MarketId, len(resp.Bids), len(resp.Asks))
		return nil
	})
	if err != nil {
		log.Fatalf("[ws] subscribe order book: %v", err)
	}

	time.Sleep(15 * time.Second)

	if unsubscribe != nil {
		_ = unsubscribe()
	}
	_ = publicSvc.Close()
	log.Println("[ws] done")
}
