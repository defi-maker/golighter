# Example Programs

The Go examples mirror the scenarios covered by the Python SDK samples in
`../lighter-python/examples`. They demonstrate how to query public data, submit
transactions, and stream market updates using the helpers provided by this
repository.

## Prerequisites

- Go 1.23+
- Environment variables (can be loaded via `.env`):
  - `LIGHTER_ENDPOINT` – default `https://mainnet.zklighter.elliot.ai`
  - `LIGHTER_ACCOUNT_INDEX` – numeric account identifier
  - `LIGHTER_API_KEY_INDEX` – API key slot to use
  - `LIGHTER_API_KEY_PRIVATE_KEY` – hex string (0x-prefixed or raw 40 bytes)
  - `LIGHTER_CHAIN_ID` – optional, defaults to `0`
- A funded Lighter account with an active API key. To bootstrap keys on testnet,
  follow the Python walkthrough in `../lighter-python/examples/system_setup.py`;
  once you have the generated values you can reuse them with the Go samples.

`dotenv -f .env -- go run …` is handy when iterating locally.

## Available Recipes

| Go sample | Mirrors Python script | Highlights |
|-----------|-----------------------|------------|
| `go run examples/get_info` | `get_info.py` | Basic REST queries: status, blocks, order book, nonce lookup |
| `go run examples/create_cancel_order` | `create_cancel_order.py` | Submit a limit order and cancel it using the signing helpers |
| `go run examples/send_tx_batch` | `send_tx_batch.py` | Bundle multiple signed transactions through the batch endpoint |
| `go run examples/ws` | `ws.py` | Subscribe to public order-book updates over WebSocket |

Additional flows such as change-api-key, leverage updates, or async WebSocket
handling follow the same patterns as their Python counterparts; porting them is
mostly a matter of translating the control flow with the `client.TxClient`
helpers. Contributions welcome.

## Common Troubleshooting

- 4XX responses usually indicate stale nonce or mismatched account/key indices.
  Re-run `NextNonceValue` or fetch `Apikeys` to verify.
- WebSocket endpoints are `wss://…/stream`. The helper `DefaultWSConfig` is
  pre-populated for mainnet; adjust `cfg.URL` if you target testnet.
- When experimenting against testnet, keep the generated API key private key
  safe—transactions are fully authenticated with it.
