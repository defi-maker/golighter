package client

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	lighterapi "github.com/defi-maker/golighter/api"
	"github.com/elliottech/lighter-go/signer"
	"github.com/elliottech/lighter-go/types"
	"github.com/elliottech/lighter-go/types/txtypes"
)

const (
	defaultExpireTime = time.Minute*10 - time.Second // matches SDK behaviour
)

type TxClient struct {
	api          *Client
	chainID      uint32
	keyManager   signer.KeyManager
	accountIndex int64
	apiKeyIndex  uint8
}

func NewTxClient(api *Client, apiKeyPrivateKey string, accountIndex int64, apiKeyIndex uint8, chainID uint32) (*TxClient, error) {
	if api == nil {
		return nil, fmt.Errorf("client: REST client is required")
	}
	if len(apiKeyPrivateKey) < 2 {
		return nil, fmt.Errorf("client: empty private key")
	}
	if apiKeyPrivateKey[:2] == "0x" {
		apiKeyPrivateKey = apiKeyPrivateKey[2:]
	}
	rawKey, err := hex.DecodeString(apiKeyPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("client: decode private key: %w", err)
	}
	keyManager, err := signer.NewKeyManager(rawKey)
	if err != nil {
		return nil, fmt.Errorf("client: create key manager: %w", err)
	}

	return &TxClient{
		api:          api,
		chainID:      chainID,
		keyManager:   keyManager,
		accountIndex: accountIndex,
		apiKeyIndex:  apiKeyIndex,
	}, nil
}

func (c *TxClient) GetAccountIndex() int64 { return c.accountIndex }

func (c *TxClient) GetApiKeyIndex() uint8 { return c.apiKeyIndex }

func (c *TxClient) GetKeyManager() signer.KeyManager { return c.keyManager }

func (c *TxClient) SwitchAPIKey(apiKey uint8) { c.apiKeyIndex = apiKey }

func (c *TxClient) CheckClient(ctx context.Context) error {
	_, err := c.api.NextNonce(ctx, &lighterapi.NextNonceParams{
		AccountIndex: c.accountIndex,
		ApiKeyIndex:  c.apiKeyIndex,
	})
	return err
}

func (c *TxClient) GetAuthToken(deadline time.Time) (string, error) {
	if time.Until(deadline) > 7*time.Hour {
		return "", fmt.Errorf("deadline should be within 7 hours")
	}

	return types.ConstructAuthToken(c.keyManager, deadline, &types.TransactOpts{
		ApiKeyIndex:      &c.apiKeyIndex,
		FromAccountIndex: &c.accountIndex,
	})
}

func (c *TxClient) fulfillDefaultOps(ops *types.TransactOpts) (*types.TransactOpts, error) {
	if ops == nil {
		ops = new(types.TransactOpts)
	}
	if ops.ExpiredAt == 0 {
		ops.ExpiredAt = time.Now().Add(defaultExpireTime).UnixMilli()
	}
	if ops.FromAccountIndex == nil {
		ops.FromAccountIndex = &c.accountIndex
	}
	if ops.ApiKeyIndex == nil {
		ops.ApiKeyIndex = &c.apiKeyIndex
	}
	if ops.Nonce == nil {
		nonce, err := c.api.NextNonceValue(context.Background(), *ops.FromAccountIndex, *ops.ApiKeyIndex)
		if err != nil {
			return nil, err
		}
		ops.Nonce = &nonce
	}
	return ops, nil
}

func (c *TxClient) Send(ctx context.Context, info txtypes.TxInfo, priceProtection *bool) (*lighterapi.RespSendTx, error) {
	txInfo, err := info.GetTxInfo()
	if err != nil {
		return nil, err
	}
	req := lighterapi.ReqSendTx{
		TxType: uint8(info.GetTxType()),
		TxInfo: txInfo,
	}
	if priceProtection != nil {
		req.PriceProtection = priceProtection
	}
	return c.api.SendTx(ctx, req)
}

func (c *TxClient) SendRawTx(ctx context.Context, info txtypes.TxInfo, priceProtection *bool) (string, error) {
	resp, err := c.Send(ctx, info, priceProtection)
	if err != nil {
		return "", err
	}
	return resp.TxHash, nil
}

func (c *TxClient) SendBatch(ctx context.Context, infos []txtypes.TxInfo) (*lighterapi.RespSendTxBatch, error) {
	txTypes := make([]int, 0, len(infos))
	txInfos := make([]string, 0, len(infos))
	for _, info := range infos {
		payload, err := info.GetTxInfo()
		if err != nil {
			return nil, err
		}
		txTypes = append(txTypes, int(info.GetTxType()))
		txInfos = append(txInfos, payload)
	}

	typesJSON, err := json.Marshal(txTypes)
	if err != nil {
		return nil, err
	}
	infosJSON, err := json.Marshal(txInfos)
	if err != nil {
		return nil, err
	}

	return c.api.SendTxBatch(ctx, lighterapi.ReqSendTxBatch{
		TxTypes: string(typesJSON),
		TxInfos: string(infosJSON),
	})
}

func (c *TxClient) FullFillDefaultOps(ops *types.TransactOpts) (*types.TransactOpts, error) {
	return c.fulfillDefaultOps(ops)
}

func (c *TxClient) GetChangePubKeyTransaction(tx *types.ChangePubKeyReq, ops *types.TransactOpts) (*txtypes.L2ChangePubKeyTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructChangePubKeyTx(c.keyManager, c.chainID, tx, ops)
}

func (c *TxClient) GetCreateOrderTransaction(tx *types.CreateOrderTxReq, ops *types.TransactOpts) (*txtypes.L2CreateOrderTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructCreateOrderTx(c.keyManager, c.chainID, tx, ops)
}

func (c *TxClient) GetCancelOrderTransaction(tx *types.CancelOrderTxReq, ops *types.TransactOpts) (*txtypes.L2CancelOrderTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructL2CancelOrderTx(c.keyManager, c.chainID, tx, ops)
}

func (c *TxClient) GetCancelAllOrdersTransaction(tx *types.CancelAllOrdersTxReq, ops *types.TransactOpts) (*txtypes.L2CancelAllOrdersTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructL2CancelAllOrdersTx(c.keyManager, c.chainID, tx, ops)
}

func (c *TxClient) GetCreateSubAccountTransaction(ops *types.TransactOpts) (*txtypes.L2CreateSubAccountTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructCreateSubAccountTx(c.keyManager, c.chainID, ops)
}

func (c *TxClient) GetCreatePublicPoolTransaction(tx *types.CreatePublicPoolTxReq, ops *types.TransactOpts) (*txtypes.L2CreatePublicPoolTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructCreatePublicPoolTx(c.keyManager, c.chainID, tx, ops)
}

func (c *TxClient) GetUpdatePublicPoolTransaction(tx *types.UpdatePublicPoolTxReq, ops *types.TransactOpts) (*txtypes.L2UpdatePublicPoolTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructUpdatePublicPoolTx(c.keyManager, c.chainID, tx, ops)
}

func (c *TxClient) GetTransferTransaction(tx *types.TransferTxReq, ops *types.TransactOpts) (*txtypes.L2TransferTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructTransferTx(c.keyManager, c.chainID, tx, ops)
}

func (c *TxClient) GetWithdrawTransaction(tx *types.WithdrawTxReq, ops *types.TransactOpts) (*txtypes.L2WithdrawTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructWithdrawTx(c.keyManager, c.chainID, tx, ops)
}

func (c *TxClient) GetUpdateLeverageTransaction(tx *types.UpdateLeverageTxReq, ops *types.TransactOpts) (*txtypes.L2UpdateLeverageTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructUpdateLeverageTx(c.keyManager, c.chainID, tx, ops)
}

func (c *TxClient) GetModifyOrderTransaction(tx *types.ModifyOrderTxReq, ops *types.TransactOpts) (*txtypes.L2ModifyOrderTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructL2ModifyOrderTx(c.keyManager, c.chainID, tx, ops)
}

func (c *TxClient) GetMintSharesTransaction(tx *types.MintSharesTxReq, ops *types.TransactOpts) (*txtypes.L2MintSharesTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructMintSharesTx(c.keyManager, c.chainID, tx, ops)
}

func (c *TxClient) GetBurnSharesTransaction(tx *types.BurnSharesTxReq, ops *types.TransactOpts) (*txtypes.L2BurnSharesTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructBurnSharesTx(c.keyManager, c.chainID, tx, ops)
}

func (c *TxClient) GetUpdateMarginTransaction(tx *types.UpdateMarginTxReq, ops *types.TransactOpts) (*txtypes.L2UpdateMarginTxInfo, error) {
	ops, err := c.fulfillDefaultOps(ops)
	if err != nil {
		return nil, err
	}
	return types.ConstructUpdateMarginTx(c.keyManager, c.chainID, tx, ops)
}
