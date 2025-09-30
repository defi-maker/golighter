package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	lighterapi "github.com/defi-maker/golighter/api"
)

func (c *Client) SendTx(ctx context.Context, req lighterapi.ReqSendTx) (*lighterapi.RespSendTx, error) {
	form := url.Values{}
	form.Set("tx_type", strconv.Itoa(int(req.TxType)))
	form.Set("tx_info", req.TxInfo)

	priceProtection := req.PriceProtection
	if priceProtection == nil {
		priceProtection = c.opts.priceProtection
	}
	if priceProtection != nil {
		form.Set("price_protection", strconv.FormatBool(*priceProtection))
	}

	resp, err := c.api.SendTxWithBodyWithResponse(ctx, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}

	return nil, resultCodeError(resp.StatusCode(), resp.Body, resp.JSON400)
}

func (c *Client) SendTxBatch(ctx context.Context, req lighterapi.ReqSendTxBatch) (*lighterapi.RespSendTxBatch, error) {
	form := url.Values{}
	form.Set("tx_types", req.TxTypes)
	form.Set("tx_infos", req.TxInfos)

	resp, err := c.api.SendTxBatchWithBodyWithResponse(ctx, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}

	return nil, resultCodeError(resp.StatusCode(), resp.Body, resp.JSON400)
}

func (c *Client) NotificationAck(ctx context.Context, body lighterapi.ReqAckNotif, authorization *string) error {
	form := url.Values{}
	form.Set("account_index", strconv.FormatInt(body.AccountIndex, 10))
	form.Set("notif_id", body.NotifId)
	if body.Auth != nil {
		form.Set("auth", *body.Auth)
	}

	params := &lighterapi.NotificationAckParams{}
	if authorization != nil {
		params.Authorization = authorization
	}

	resp, err := c.api.NotificationAckWithBodyWithResponse(ctx, params, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	if resp.JSON200 != nil && (resp.JSON200.Code == 0 || resp.JSON200.Code == 200) {
		return nil
	}
	return resultCodeError(resp.StatusCode(), resp.Body, resp.JSON400)
}

func (c *Client) ChangeAccountTier(ctx context.Context, params *lighterapi.ChangeAccountTierParams, body lighterapi.ReqChangeAccountTier) (*lighterapi.RespChangeAccountTier, error) {
	form := url.Values{}
	form.Set("account_index", strconv.FormatInt(body.AccountIndex, 10))
	form.Set("new_tier", body.NewTier)
	if body.Auth != nil {
		form.Set("auth", *body.Auth)
	}

	resp, err := c.api.ChangeAccountTierWithBodyWithResponse(ctx, params, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	if resp.JSON200 != nil {
		if resp.JSON200.Code == 0 || resp.JSON200.Code == 200 {
			return resp.JSON200, nil
		}
		return nil, fmt.Errorf("lighter api: code=%d message=%s", resp.JSON200.Code, deref(resp.JSON200.Message))
	}

	return nil, resultCodeError(resp.StatusCode(), resp.Body, resp.JSON400)
}
