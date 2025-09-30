package client

import (
	"errors"

	lighterapi "github.com/defi-maker/golighter/api"
)

type Client struct {
	api  lighterapi.ClientWithResponsesInterface
	opts options
}

func New(baseURL string, opts ...Option) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("baseURL cannot be empty")
	}

	cfg := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	clientOpts := cfg.toClientOptions()

	apiClient, err := lighterapi.NewClientWithResponses(baseURL, clientOpts...)
	if err != nil {
		return nil, err
	}

	return &Client{api: apiClient, opts: cfg}, nil
}

func (c *Client) API() lighterapi.ClientWithResponsesInterface {
	return c.api
}
