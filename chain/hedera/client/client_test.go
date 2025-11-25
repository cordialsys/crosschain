package client_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/hedera/client"
	xclient "github.com/cordialsys/crosschain/client"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/stretchr/testify/require"
)

func newTestClient(indexerUrl string, grpcUrl string) xclient.Client {
	cfg := xc.NewChainConfig("HBAR").
		WithIndexer("rpc", indexerUrl).
		WithUrl(grpcUrl).
		WithChainID("0.0.4")
	c, err := client.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	return c
}

func TestNewClient(t *testing.T) {
	v := []struct {
		name       string
		indexerUrl string
		grpcUrl    string
		nodeId     string
		err        bool
	}{
		{
			name:       "ValidConfig",
			indexerUrl: "validurl.com:5000",
			grpcUrl:    "validurl.com:5005",
			nodeId:     "0.0.1",
			err:        false,
		},
	}

	for _, v := range v {
		t.Run(v.name, func(t *testing.T) {
			cfg := xc.NewChainConfig("HBAR").
				WithIndexer("rpc", v.indexerUrl).
				WithUrl(v.grpcUrl).
				WithChainID(v.nodeId)
			c, err := client.NewClient(cfg)
			if v.err == false {
				require.NotNil(t, c)
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestFetchBalance(t *testing.T) {
	v := []struct {
		name                string
		contract            string
		accountInfoResponse string
		expectedBalance     uint64
		err                 bool
	}{
		{
			name:                "NativeBalance",
			contract:            "",
			accountInfoResponse: `{"account":"0.0.7182039","alias":"HIQQHBU2PTHO5ZGVNFE3ELLHYASWPJS3EP5B7PHHOG3VGM2X42QQKR6S","auto_renew_period":7776000,"balance":{"balance":97869764942,"timestamp":"1762254858.110234731","tokens":[{"token_id":"0.0.7190398","balance":4899}]},"created_timestamp":"1762161760.716980902","decline_reward":false,"deleted":false,"ethereum_nonce":6,"evm_address":"0x4ad30627995a51f582c2e4c832e38b4c799104a9","key":{"_type":"ECDSA_SECP256K1","key":"03869a7cceeee4d56949b22d67c02567a65b23fa1fbce771b7533357e6a10547d2"},"max_automatic_token_associations":-1,"memo":"","pending_reward":0,"receiver_sig_required":false,"staked_account_id":null,"staked_node_id":null,"stake_period_start":null}`,
			expectedBalance:     97869764942,
		},
		{
			name:                "TokenBalance",
			contract:            "0.0.7190398",
			accountInfoResponse: `{"account":"0.0.7182039","alias":"HIQQHBU2PTHO5ZGVNFE3ELLHYASWPJS3EP5B7PHHOG3VGM2X42QQKR6S","auto_renew_period":7776000,"balance":{"balance":97869764942,"timestamp":"1762254858.110234731","tokens":[{"token_id":"0.0.7190398","balance":4899}]},"created_timestamp":"1762161760.716980902","decline_reward":false,"deleted":false,"ethereum_nonce":6,"evm_address":"0x4ad30627995a51f582c2e4c832e38b4c799104a9","key":{"_type":"ECDSA_SECP256K1","key":"03869a7cceeee4d56949b22d67c02567a65b23fa1fbce771b7533357e6a10547d2"},"max_automatic_token_associations":-1,"memo":"","pending_reward":0,"receiver_sig_required":false,"staked_account_id":null,"staked_node_id":null,"stake_period_start":null}`,
			expectedBalance:     4899,
		},
		{
			name:     "MissingAccount",
			contract: "",
			accountInfoResponse: `{"_status":{"messages":[{"message":"Not found"}]}}
`,
			err: true,
		},
	}

	for _, v := range v {
		t.Run(v.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if v.err {
					w.WriteHeader(404)
				}
				_, err := w.Write([]byte(v.accountInfoResponse))
				require.NoError(t, err)
			}))
			c := newTestClient(server.URL, "grpc.url")
			options := make([]xclient.GetBalanceOption, 0)
			if v.contract != "" {
				options = append(options, xclient.BalanceOptionContract(xc.ContractAddress(v.contract)))
			}
			args := xclient.NewBalanceArgs("0x4ad30627995a51f582c2e4c832e38b4c799104a9", options...)
			b, err := c.FetchBalance(context.TODO(), args)
			if v.err {
				require.Error(t, err)
			} else {
				require.Equal(t, v.expectedBalance, b.Uint64())
			}
		})
	}
}

func TestFetchTxInfo(t *testing.T) {
	v := []struct {
		name                     string
		fetchTxResponse          string
		fetchRawBlockResponse    string
		fetchLatestBlockResponse string
		addressInfoResponses     []string
		expectedInfo             txinfo.TxInfo
		err                      bool
	}{
		{
			name:                     "NativeOnlyTxInfo",
			fetchTxResponse:          `{"transactions":[{"batch_key":null,"bytes":null,"charged_tx_fee":585704,"consensus_timestamp":"1762248133.170914767","entity_id":null,"max_fee":"100000000","max_custom_fees":[],"memo_base64":"","name":"CRYPTOTRANSFER","nft_transfers":[],"node":"0.0.6","nonce":0,"parent_consensus_timestamp":null,"result":"SUCCESS","scheduled":false,"staking_reward_transfers":[],"token_transfers":[{"token_id":"0.0.7190398","account":"0.0.7182039","amount":-100,"is_approval":false},{"token_id":"0.0.7190398","account":"0.0.7182146","amount":100,"is_approval":false}],"transaction_hash":"V7JvAxTwfKqF0PgkwucHKONHX/iuiOlpFqzKqPmaFHsy1KfFZf2fXwwRrRpVAt/Y","transaction_id":"0.0.7182039-1762248127-858889420","transfers":[{"account":"0.0.6","amount":27447,"is_approval":false},{"account":"0.0.98","amount":446607,"is_approval":false},{"account":"0.0.800","amount":55825,"is_approval":false},{"account":"0.0.801","amount":55825,"is_approval":false},{"account":"0.0.7182039","amount":-585704,"is_approval":false},{"account":"0.0.7182040","amount":300,"is_approval":false}],"valid_duration_seconds":"120","valid_start_timestamp":"1762248127.858889420"}]}`,
			fetchRawBlockResponse:    `{"blocks":[{"count":12,"hapi_version":"0.67.2","hash":"0x02a7a011fe1d5e7ed41c1b7322295af8a3b0daa12841ebdb2a422ac026745c685abb47ad800fb0e129f608384ad91850","name":"2025-11-04T09_22_12.944420809Z.rcd.gz","number":27100872,"previous_hash":"0xbea6d83958bf88d4a14901e983bc099673267b7b3e5744a8743c8933d9d079e5a688df554880d7e64232a5e291eb7107","size":4314,"timestamp":{"from":"1762248132.944420809","to":"1762248133.904457163"},"gas_used":0,"logs_bloom":"0x"}],"links":{"next":"/api/v1/blocks?limit=1&order=asc&timestamp=gt:1762248133.170914767&block.number=gt:27100872"}}`,
			fetchLatestBlockResponse: `{"blocks":[{"count":119,"hapi_version":"0.67.2","hash":"0xfb007c705d366e3e325901dfb0e15ce7ee20f7b6a5db28c803da4fe974c58efd3bc780655d49f2ac3ec370bc567d554e","name":"2025-11-10T22_09_24.025675586Z.rcd.gz","number":27383088,"previous_hash":"0x80ac1b7c3eb97e0701066a2df8c82d24aae9910432688c22ce34e4f8ec58741f92bb837ec460056ae07b3caa71177ab0","size":35750,"timestamp":{"from":"1762812564.025675586","to":"1762812565.986935000"},"gas_used":0,"logs_bloom":"0x"}],"links":{"next":"/api/v1/blocks?limit=1&block.number=lt:27383088"}}`,
			addressInfoResponses: []string{
				`{ "account": "0.0.7182039", "evm_address": "0x4ad30627995a51f582c2e4c832e38b4c799104a9"}`,
				`{ "account": "0.0.7182040", "evm_address": "0x00000000000000000000000000000000796d96d8"}`,
				`{ "account": "0.0.7182146", "evm_address": "0x0000000000000000000000000000000006d9742" }`,
			},
			expectedInfo: txinfo.TxInfo{
				Name:     "chains/HBAR/transactions/0x57b26f0314f07caa85d0f824c2e70728e3475ff8ae88e96916accaa8f99a147b32d4a7c565fd9f5f0c11ad1a5502dfd8",
				Hash:     "0x57b26f0314f07caa85d0f824c2e70728e3475ff8ae88e96916accaa8f99a147b32d4a7c565fd9f5f0c11ad1a5502dfd8",
				LookupId: "0.0.7182039-1762248127-858889420",
				XChain:   "HBAR",
				State:    txinfo.Succeeded,
				Final:    true,
				Block: &txinfo.Block{
					Chain:  "HBAR",
					Height: xc.NewAmountBlockchainFromUint64(27100872),
					Hash:   "0x02a7a011fe1d5e7ed41c1b7322295af8a3b0daa12841ebdb2a422ac026745c685abb47ad800fb0e129f608384ad91850",
					Time:   TimeParse("2025-11-04T10:22:13.904457163+01:00"),
				},
				Movements: []*txinfo.Movement{
					NewMovement(
						"HBAR",
						"HBAR",
						[]*txinfo.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(585704),
								XAddress:  "chains/HBAR/addresses/0x4ad30627995a51f582c2e4c832e38b4c799104a9",
								AddressId: "0x4ad30627995a51f582c2e4c832e38b4c799104a9",
								Event: &txinfo.Event{
									Id:      "4",
									Variant: txinfo.MovementVariantNative,
								},
							},
						},
						[]*txinfo.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(300),
								XAddress:  "chains/HBAR/addresses/0x00000000000000000000000000000000796d96d8",
								AddressId: "0x00000000000000000000000000000000796d96d8",
								Event: &txinfo.Event{
									Id:      "5",
									Variant: txinfo.MovementVariantNative,
								},
							},
						},
					),
					NewMovement(
						"HBAR",
						"0.0.7190398",
						[]*txinfo.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(100),
								XAddress:  "chains/HBAR/addresses/0x4ad30627995a51f582c2e4c832e38b4c799104a9",
								AddressId: "0x4ad30627995a51f582c2e4c832e38b4c799104a9",
								Event: &txinfo.Event{
									Id:      "6",
									Variant: txinfo.MovementVariantToken,
								},
							},
						},
						[]*txinfo.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(100),
								XAddress:  "chains/HBAR/addresses/0x0000000000000000000000000000000006d9742",
								AddressId: "0x0000000000000000000000000000000006d9742",
								Event: &txinfo.Event{
									Id:      "7",
									Variant: txinfo.MovementVariantToken,
								},
							},
						},
					),
				},
				Fees: []*txinfo.Balance{
					{
						Balance:  xc.NewAmountBlockchainFromUint64(585404),
						Contract: "HBAR",
						Asset:    "chains/HBAR/assets/HBAR",
					},
				},
				Confirmations: 282216,
				Error:         nil,
			},
			err: false,
		},
	}
	for _, v := range v {
		t.Run(v.name, func(t *testing.T) {

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if v.err {
					w.WriteHeader(404)
				}
				url := r.URL.String()
				fmt.Printf("fetching: %s\n", url)
				var err error
				if strings.Contains(url, "transactions") {
					_, err = w.Write([]byte(v.fetchTxResponse))
				} else if strings.Contains(url, "blocks") && strings.Contains(url, "order") {
					_, err = w.Write([]byte(v.fetchRawBlockResponse))
				} else if strings.Contains(url, "blocks") {
					_, err = w.Write([]byte(v.fetchLatestBlockResponse))
				} else if strings.Contains(url, "accounts") {
					response := v.addressInfoResponses[0]
					v.addressInfoResponses = v.addressInfoResponses[1:]
					_, err = w.Write([]byte(response))
				}
				require.NoError(t, err)
			}))

			client := newTestClient(server.URL, "grpc")
			args := txinfo.NewArgs("hash")
			info, err := client.FetchTxInfo(context.Background(), args)
			require.NotNil(t, info)
			require.NoError(t, err)
			require.Equal(t, v.expectedInfo, info)
		})
	}
}

func TimeParse(t string) time.Time {
	ts, err := time.Parse(time.RFC3339Nano, t)
	if err != nil {
		panic(err)
	}

	ut := ts.UnixNano()
	return time.Unix(0, ut)
}

func NewMovement(
	chain string,
	contract string,
	from []*txinfo.BalanceChange,
	to []*txinfo.BalanceChange,
) *txinfo.Movement {
	movement := txinfo.NewMovement(xc.NativeAsset(chain), xc.ContractAddress(contract))
	movement.From = from
	movement.To = to
	return movement
}
