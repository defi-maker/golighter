package client

import (
	"context"

	lighterapi "github.com/defi-maker/golighter/api"
)

func (c *Client) NextNonceValue(ctx context.Context, accountIndex int64, apiKeyIndex uint8) (int64, error) {
	resp, err := c.NextNonce(ctx, &lighterapi.NextNonceParams{AccountIndex: accountIndex, ApiKeyIndex: apiKeyIndex})
	if err != nil {
		return 0, err
	}
	return resp.Nonce, nil
}

func (c *Client) ApiKeysFor(ctx context.Context, accountIndex int64, apiKeyIndex *uint8) (*lighterapi.AccountApiKeys, error) {
	params := &lighterapi.ApikeysParams{AccountIndex: accountIndex}
	if apiKeyIndex != nil {
		params.ApiKeyIndex = apiKeyIndex
	}
	return c.Apikeys(ctx, params)
}
