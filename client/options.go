package client

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	lighterapi "github.com/defi-maker/golighter/api"
)

type Option func(*options)

type options struct {
	httpClient      lighterapi.HttpRequestDoer
	requestEditors  []lighterapi.RequestEditorFn
	priceProtection *bool
}

func (o options) toClientOptions() []lighterapi.ClientOption {
	opts := make([]lighterapi.ClientOption, 0, 1+len(o.requestEditors))
	if o.httpClient != nil {
		opts = append(opts, lighterapi.WithHTTPClient(o.httpClient))
	}
	for _, editor := range o.requestEditors {
		if editor != nil {
			opts = append(opts, lighterapi.WithRequestEditorFn(editor))
		}
	}
	return opts
}

func defaultOptions() options {
	return options{
		httpClient: DefaultHTTPClient(),
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(o *options) {
		if client != nil {
			o.httpClient = client
		}
	}
}

func WithRequestEditor(fn lighterapi.RequestEditorFn) Option {
	return func(o *options) {
		if fn != nil {
			o.requestEditors = append(o.requestEditors, fn)
		}
	}
}

func WithStaticHeader(key, value string) Option {
	if key == "" {
		return func(o *options) {}
	}
	return WithRequestEditor(func(ctx context.Context, req *http.Request) error {
		if value == "" {
			req.Header.Del(key)
		} else {
			req.Header.Set(key, value)
		}
		return nil
	})
}

func WithChannelName(channel string) Option {
	return WithStaticHeader("Channel-Name", channel)
}

func WithPriceProtection(enabled bool) Option {
	v := enabled
	return func(o *options) {
		o.priceProtection = &v
	}
}

func DefaultHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 60 * time.Second,
	}
	transport := &http.Transport{
		DialContext:         dialer.DialContext,
		MaxConnsPerHost:     1000,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     10 * time.Second,
		TLSClientConfig: &tls.Config{ // #nosec G402 -- copying legacy behaviour; callers should override via WithHTTPClient
			InsecureSkipVerify: true,
		},
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
}
