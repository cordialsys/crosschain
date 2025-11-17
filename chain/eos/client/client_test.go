package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/eos/client"
	"github.com/cordialsys/crosschain/chain/eos/tx_input"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/cordialsys/crosschain/testutil"
	"github.com/stretchr/testify/require"
)

func TestFetchTxInput(t *testing.T) {
	vectors := []struct {
		name            string
		err             string
		expectedTxInput tx_input.TxInput

		getInfoResult        json.RawMessage
		getAccountResults    []json.RawMessage
		getAccountInfoResult json.RawMessage
	}{
		{
			name: "Test valid Tx input",

			getInfoResult: json.RawMessage(`{"server_version":"14092132","chain_id":"73e4385a2708e6d7048834fbc1079f2fabb17b3c125b146af438971e90716c4d","head_block_num":209671574,"last_irreversible_block_num":209671572,"last_irreversible_block_id":"0c7f5594163655e89089f38432cef53c7dbc0ad5c9abe268fbf74335e977f7e9","head_block_id":"0c7f55960e5abdad44bb04734a992848ddef624f58b83cb5c8c9040a4832f764","head_block_time":"2025-06-23T14:08:39.000","head_block_producer":"eosriobrazil","virtual_block_cpu_limit":200000000,"virtual_block_net_limit":1048576000,"block_cpu_limit":200000,"block_net_limit":1048576,"server_version_string":"v1.2.0-rc3","fork_db_head_block_num":209671574,"fork_db_head_block_id":"0c7f55960e5abdad44bb04734a992848ddef624f58b83cb5c8c9040a4832f764","server_full_version_string":"v1.2.0-rc3-14092132c2ff43404f40737b63920ca535be3341","total_cpu_weight":"120570311364597","total_net_weight":"117540044345254","earliest_available_block_num":1,"last_irreversible_block_time":"2025-06-23T14:08:38.000"}`),
			getAccountResults: []json.RawMessage{
				json.RawMessage(`{"accounts":[{"account_name":"cordialsysaa","permission_name":"active","authorizing_key":"EOS88T1B86LmH3GNhFfKx268N6LnFBVCGzLBWGLMZ9wAQDr6e3ezk","weight":1,"threshold":1},{"account_name":"cordialsysaa","permission_name":"owner","authorizing_key":"EOS88T1B86LmH3GNhFfKx268N6LnFBVCGzLBWGLMZ9wAQDr6e3ezk","weight":1,"threshold":1}]}`),
			},
			getAccountInfoResult: json.RawMessage(`{"account_name":"cordialsysaa","head_block_num":209671575,"head_block_time":"2025-06-23T14:08:39.500","privileged":false,"last_code_update":"1970-01-01T00:00:00.000","created":"2025-06-18T22:01:08.500","core_liquid_balance":"62.2841 EOS","ram_quota":5675,"net_weight":20000,"cpu_weight":125000,"net_limit":{"used":430,"available":30401,"max":30831,"last_usage_update_time":"2025-06-23T13:34:20.500","current_used":419},"cpu_limit":{"used":1380,"available":34449,"max":35829,"last_usage_update_time":"2025-06-23T13:34:20.500","current_used":1347},"ram_usage":4675,"permissions":[{"perm_name":"active","parent":"owner","required_auth":{"threshold":1,"keys":[{"key":"EOS88T1B86LmH3GNhFfKx268N6LnFBVCGzLBWGLMZ9wAQDr6e3ezk","weight":1}],"accounts":[],"waits":[]},"linked_actions":[]},{"perm_name":"owner","parent":"","required_auth":{"threshold":1,"keys":[{"key":"EOS88T1B86LmH3GNhFfKx268N6LnFBVCGzLBWGLMZ9wAQDr6e3ezk","weight":1}],"accounts":[],"waits":[]},"linked_actions":[]}],"total_resources":{"owner":"cordialsysaa","net_weight":"2.0000 EOS","cpu_weight":"12.5000 EOS","ram_bytes":4275},"self_delegated_bandwidth":{"from":"cordialsysaa","to":"cordialsysaa","net_weight":"2.0000 EOS","cpu_weight":"12.5000 EOS"},"refund_request":{"owner":"cordialsysaa","request_time":"2025-06-21T13:18:24","net_amount":"6.0000 EOS","cpu_amount":"1.0000 EOS"},"voter_info":{"owner":"cordialsysaa","proxy":"","producers":[],"staked":145000,"last_vote_weight":"0.00000000000000000","proxied_vote_weight":"0.00000000000000000","is_proxy":0,"flags1":0,"reserved2":0,"reserved3":"0 "},"rex_info":null,"subjective_cpu_bill_limit":{"used":0,"available":0,"max":0,"last_usage_update_time":"2000-01-01T00:00:00.000","current_used":0},"eosio_any_linked_actions":[]}`),
			expectedTxInput: tx_input.TxInput{
				TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverEOS),
				ChainID:         testutil.FromHex("73e4385a2708e6d7048834fbc1079f2fabb17b3c125b146af438971e90716c4d"),
				HeadBlockID:     testutil.FromHex("0c7f55960e5abdad44bb04734a992848ddef624f58b83cb5c8c9040a4832f764"),
				FromAccount:     "cordialsysaa",
				Symbol:          "EOS",
				AvailableRam:    1000,
				AvailableCPU:    34449,
				AvailableNET:    30401,
				TargetRam:       1000,
				EosBalance:      xc.NewAmountBlockchainFromUint64(622841),
			},
		},
	}

	for _, vector := range vectors {
		t.Run(vector.name, func(t *testing.T) {
			accountIndexCounter := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				url := r.URL.String()
				var err error
				if strings.HasSuffix(url, "/v1/chain/get_info") {
					err = json.NewEncoder(w).Encode(vector.getInfoResult)
				} else if strings.Contains(url, "/v1/chain/get_accounts_by_authorizers") {
					err = json.NewEncoder(w).Encode(vector.getAccountResults[accountIndexCounter])
					accountIndexCounter++
				} else if strings.Contains(url, "/v1/chain/get_account") {
					err = json.NewEncoder(w).Encode(vector.getAccountInfoResult)
				} else {
					t.Errorf("unexpected url: %s", url)
				}
				require.NoError(t, err)
			}))
			defer server.Close()

			chainCfg := xc.NewChainConfig(xc.EOS).
				WithUrl(server.URL).
				WithDecimals(4)

			client, _ := client.NewClient(chainCfg)
			from := xc.Address("EOS8muuPYjH9rEf4KrXzD5fnbcqzAms8pFbREGhQpUKZ3pSZGW9HN")
			to := xc.Address("EOS8WvdPVMuZW8cqjN18DFfM8hGkfbswHL4zoyqNi6wQexja5bhqE")
			args, _ := builder.NewTransferArgs(chainCfg.Base(), from, to, xc.NewAmountBlockchainFromUint64(10000))
			input, err := client.FetchTransferInput(
				context.Background(),
				args,
			)
			if err != nil {
				t.Logf("Error: %v", err)
				require.ErrorContains(t, err, vector.err)
				require.NotEqual(t, vector.err, "")
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				txInput := input.(*tx_input.TxInput)
				// ignore the timestamp
				txInput.SetUnix(0)
				require.Equal(t, vector.expectedTxInput, *txInput)
			}
		})
	}
}

func TestFetchTxInfo(t *testing.T) {
	vectors := []struct {
		name           string
		err            string
		expectedTxInfo txinfo.TxInfo

		getTxResult   json.RawMessage
		getInfoResult json.RawMessage
	}{
		{
			name: "Test valid Tx info",

			getTxResult:   json.RawMessage(`{"query_time_ms":19.297,"executed":true,"trx_id":"4c66c16b364ba919281b0388f3048ec04afec2b5ed5704bb2169772783d84cc3","lib":209672969,"cached_lib":false,"actions":[{"action_ordinal":1,"creator_action_ordinal":0,"act":{"account":"eosio.token","name":"transfer","authorization":[{"actor":"crosschaino2","permission":"active"},{"actor":"cordialsysaa","permission":"active"}],"data":{"from":"crosschaino2","to":"crosschaino3","amount":0.01,"symbol":"EOS","memo":"","quantity":"0.0100 EOS"}},"@timestamp":"2025-06-21T20:18:42.000","block_num":209370387,"block_id":"0c7abd131d6352acdc395ef6585a0f567dd10d22fc39d34fc75e3d5c7e5cec10","producer":"eosarabianet","trx_id":"4c66c16b364ba919281b0388f3048ec04afec2b5ed5704bb2169772783d84cc3","global_sequence":279446531,"cpu_usage_us":274,"net_usage_words":20,"signatures":["SIG_K1_HDzqM2Zcg1N9R5niP8TNLyNk8ii2nnNhtPxgQRGiTyZ1q9ZxuKM4DEvrmQmTHhLFdKJJDHoHAUGaYEPpUVfVYnKaknUj1b","SIG_K1_Gg4VnfjGduFMNDHGLgna2XuURydt49YG9DQdH9txKaMUMyQQt4nrDuajjdm8LiXDw9pdN6c7ippQjRKoPyht8DYrGxUcji"],"inline_count":2,"inline_filtered":false,"receipts":[{"receiver":"eosio.token","global_sequence":"279446531","recv_sequence":"8533662","auth_sequence":[{"account":"cordialsysaa","sequence":"356"},{"account":"crosschaino2","sequence":"95"}]},{"receiver":"crosschaino2","global_sequence":"279446532","recv_sequence":"39","auth_sequence":[{"account":"cordialsysaa","sequence":"357"},{"account":"crosschaino2","sequence":"96"}]},{"receiver":"crosschaino3","global_sequence":"279446533","recv_sequence":"35","auth_sequence":[{"account":"cordialsysaa","sequence":"358"},{"account":"crosschaino2","sequence":"97"}]}],"code_sequence":4,"abi_sequence":5,"act_digest":"B3AEB3B6827E5C2FCE0CC987D9BE1B40B7DC36BF46A389377BD63EC101A534B8","timestamp":"2025-06-21T20:18:42.000"}],"last_indexed_block":209672971,"last_indexed_block_time":"2025-06-23T14:20:17.500"}`),
			getInfoResult: json.RawMessage(`{"server_version":"14092132","chain_id":"73e4385a2708e6d7048834fbc1079f2fabb17b3c125b146af438971e90716c4d","head_block_num":209672971,"last_irreversible_block_num":209672969,"last_irreversible_block_id":"0c7f5b09149217949634096d1994a923978260a212e254358924287c5bb983af","head_block_id":"0c7f5b0b70ad72daf18944fc3ffad30f06f0ef42cfbab33ce87a47e1855bbce5","head_block_time":"2025-06-23T14:20:17.500","head_block_producer":"atticlabeosb","virtual_block_cpu_limit":200000000,"virtual_block_net_limit":1048576000,"block_cpu_limit":200000,"block_net_limit":1048576,"server_version_string":"v1.2.0-rc3","fork_db_head_block_num":209672971,"fork_db_head_block_id":"0c7f5b0b70ad72daf18944fc3ffad30f06f0ef42cfbab33ce87a47e1855bbce5","server_full_version_string":"v1.2.0-rc3-14092132c2ff43404f40737b63920ca535be3341","total_cpu_weight":"120570311364597","total_net_weight":"117540044345254","earliest_available_block_num":1,"last_irreversible_block_time":"2025-06-23T14:20:16.500"}`),
			expectedTxInfo: txinfo.TxInfo{
				Name:   "chains/EOS/transactions/4c66c16b364ba919281b0388f3048ec04afec2b5ed5704bb2169772783d84cc3",
				Hash:   "4c66c16b364ba919281b0388f3048ec04afec2b5ed5704bb2169772783d84cc3",
				XChain: xc.EOS,
				State:  txinfo.Succeeded,
				Final:  true,
				Block: &txinfo.Block{
					Chain:  xc.EOS,
					Height: xc.NewAmountBlockchainFromUint64(209370387),
					Hash:   "0c7abd131d6352acdc395ef6585a0f567dd10d22fc39d34fc75e3d5c7e5cec10",
					Time:   testutil.FromTimeStamp("2025-06-21T20:18:42Z"),
				},
				Movements: []*txinfo.Movement{
					{
						XAsset:     "chains/EOS/assets/EOS",
						XContract:  "EOS",
						AssetId:    "EOS",
						ContractId: "",
						From: []*txinfo.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(100),
								XAddress:  "chains/EOS/addresses/crosschaino2",
								AddressId: "crosschaino2",
							},
						},
						To: []*txinfo.BalanceChange{
							{
								Balance:   xc.NewAmountBlockchainFromUint64(100),
								XAddress:  "chains/EOS/addresses/crosschaino3",
								AddressId: "crosschaino3",
							},
						},
					},
				},
				Fees:          []*txinfo.Balance{},
				Confirmations: 302584,
			},
		},
	}

	for _, vector := range vectors {
		t.Run(vector.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				url := r.URL.String()
				var err error
				if strings.HasSuffix(url, "/v1/chain/get_info") {
					err = json.NewEncoder(w).Encode(vector.getInfoResult)
				} else if strings.Contains(url, "/v2/history/get_transaction") {
					err = json.NewEncoder(w).Encode(vector.getTxResult)
				} else {
					t.Errorf("unexpected url: %s", url)
				}
				require.NoError(t, err)
			}))
			defer server.Close()

			chainCfg := xc.NewChainConfig(xc.EOS).
				WithUrl(server.URL).
				WithDecimals(4)

			client, _ := client.NewClient(chainCfg)
			args := txinfo.NewArgs(xc.TxHash("abc"))
			txInfo, err := client.FetchTxInfo(
				context.Background(),
				args,
			)
			if err != nil {
				t.Logf("Error: %v", err)
				require.ErrorContains(t, err, vector.err)
				require.NotEqual(t, vector.err, "")
			} else {
				require.NoError(t, err)
				require.NotNil(t, txInfo)
				testutil.TxInfoEqual(t, vector.expectedTxInfo, txInfo)
			}
		})
	}
}
