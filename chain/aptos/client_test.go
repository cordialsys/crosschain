package aptos

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/aptos/tx_input"
	"github.com/cordialsys/crosschain/client"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	testtypes "github.com/cordialsys/crosschain/testutil"
	"github.com/stretchr/testify/require"
)

func (s *AptosTestSuite) TestNewClient() {
	require := s.Require()
	resp := `{"chain_id":38,"epoch":"133","ledger_version":"13087045","oldest_ledger_version":"0","ledger_timestamp":"1669676013555573","node_role":"full_node","oldest_block_height":"0","block_height":"5435983","git_hash":"2c74a456298fcd520241a562119b6fe30abdaae2"}`
	server, close := testtypes.MockHTTP(s.T(), resp, 200)
	defer close()

	cfg := xc.NewChainConfig(xc.APTOS).WithUrl(server.URL)
	client, err := NewClient(cfg)
	require.NotNil(client)
	require.Nil(err)
}

// A subset of the gas schedule, just enough to test the gas estimation
// See the full schedule here:
// https://api.mainnet.aptoslabs.com/v1/accounts/0x1/resource/0x1::gas_schedule::GasScheduleV2
var gasSchedule = `{
  "type": "0x1::gas_schedule::GasScheduleV2",
  "data": {
    "entries": [
      {
        "key": "txn.min_transaction_gas_units",
        "val": "2760000"
      },
      {
        "key": "txn.min_price_per_gas_unit",
        "val": "100"
      },
      {
        "key": "txn.gas_unit_scaling_factor",
        "val": "1000000"
      }
    ],
    "feature_version": "36"
  }
}`

func TestFetchTxInput(t *testing.T) {
	require := require.New(t)

	vectors := []struct {
		asset xc.ITask
		resp  []string
		from  string
		input *tx_input.TxInput
		err   string
	}{
		{
			asset: xc.NewChainConfig(""),
			// valid blockhash
			resp: []string{
				`{"chain_id":58,"epoch":"61","ledger_version":"3524910","oldest_ledger_version":"0","ledger_timestamp":"1683057860656414","node_role":"full_node","oldest_block_height":"0","block_height":"1317171","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
				`{"chain_id":58,"epoch":"61","ledger_version":"3524910","oldest_ledger_version":"0","ledger_timestamp":"1683057860656414","node_role":"full_node","oldest_block_height":"0","block_height":"1317171","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
				`{"sequence_number":"2","authentication_key":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682"}`,
				gasSchedule,
			},
			from: "0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682",
			input: &tx_input.TxInput{
				TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverAptos),
				SequenceNumber:  2,
				GasLimit:        DefaultGasLimit,
				GasPrice:        100,
				Timestamp:       1683057860656414,
				ChainId:         58,
			},
			err: "",
		},
		{
			asset: xc.NewChainConfig(""),
			// valid blockhash
			resp: []string{
				`{"chain_id":58,"epoch":"61","ledger_version":"3524910","oldest_ledger_version":"0","ledger_timestamp":"1683057860656414","node_role":"full_node","oldest_block_height":"0","block_height":"1317171","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
				`{"chain_id":58,"epoch":"61","ledger_version":"3524910","oldest_ledger_version":"0","ledger_timestamp":"1683057860656414","node_role":"full_node","oldest_block_height":"0","block_height":"1317171","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
				`{"message":"Account not found by Address(0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f681) and Ledger version(3545185)","error_code":"account_not_found","vm_error_code":null}`,
				gasSchedule,
			},
			from:  "0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f680",
			input: &tx_input.TxInput{},
			err:   "Account not found",
		},
	}

	for i, v := range vectors {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {

			resp := `{"chain_id":38,"epoch":"133","ledger_version":"13087045","oldest_ledger_version":"0","ledger_timestamp":"1669676013555573","node_role":"full_node","oldest_block_height":"0","block_height":"5435983","git_hash":"2c74a456298fcd520241a562119b6fe30abdaae2"}`

			// satisfy the gas estimate to go with default value
			blockNotFound := `{"message":"block not found","error_code":"block_not_found","vm_error_code":null}`
			for i := 0; i < 20; i++ {
				v.resp = append(v.resp, blockNotFound)
			}
			server, close := testtypes.MockHTTP(t, resp, 200)
			asset := v.asset.GetChain()
			asset.URL = server.URL
			client, _ := NewClient(asset)
			if v.err != "" {
				// errors should return 400 status code.
				server.StatusCodes = []int{200, 200, 400}
			}
			server.Response = v.resp
			input, err := client.FetchLegacyTxInput(context.Background(), xc.Address(v.from), "")

			if v.err != "" {
				require.ErrorContains(err, v.err)
			} else {
				require.NoError(err)
				require.NotNil(input)
				require.Equal(v.input, input)
			}
			close()
		})
	}
}

func (s *AptosTestSuite) TestSubmitTx() {
	require := s.Require()
	server, close := testtypes.MockHTTP(s.T(), []string{
		`{"chain_id":58,"epoch":"61","ledger_version":"3524910","oldest_ledger_version":"0","ledger_timestamp":"1683057860656414","node_role":"full_node","oldest_block_height":"0","block_height":"1317171","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
		// transaction submitted
		`{"hash":"0x5ec9ac15dee869a7364f31534e9d98db09c6dd028a64aa95b2b6d896348c4c94","sender":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","sequence_number":"2","max_gas_amount":"2000","gas_unit_price":"100","expiration_timestamp_secs":"1683068558920777","payload":{"function":"0x1::aptos_account::transfer","type_arguments":[],"arguments":["0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365","12300000"],"type":"entry_function_payload"},"signature":{"public_key":"0xa09bb3957ad788bfcfd3f7c5eda9ab2876ff0de8db38dafdf439cfe3f96673b6","signature":"0xc32be4211fe1655e86d4d1558fdc48252e01e9f8ca9d14a1c815fce0913e9eac0360eb2991d3ea58a19e64461e9404a41e31aa20d4ba4bc184a353cecb8c9d0e","type":"ed25519_signature"}}`,
		// 2nd submit should be an error
		`{"message": "error"}`,
	}, 200)
	server.StatusCodes = []int{200, 200, 400}
	defer close()
	asset := xc.NewChainConfig(xc.APTOS).WithUrl(server.URL).WithNet("devnet")

	builder, _ := NewTxBuilder(asset.Base())
	from := xc.Address("0xa589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab85")
	to := xc.Address("0xbb89a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab00")
	amount := xc.NewAmountBlockchainFromUint64(1)
	pubkey := []byte{1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8}
	input := &tx_input.TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverAptos),
		SequenceNumber:  3,
		GasLimit:        2000,
		GasPrice:        10,
		Timestamp:       12345,
		ChainId:         1,
	}

	args := buildertest.MustNewTransferArgs(asset.ChainBaseConfig, from, to, amount, buildertest.OptionPublicKey(pubkey))
	tf, err := builder.Transfer(args, input)
	require.NoError(err)
	require.NotNil(tf)

	// add signature
	sig := []byte{}
	for i := 0; i < 64; i++ {
		sig = append(sig, byte(i))
	}
	err = tf.SetSignatures(&xc.SignatureResponse{
		Signature: sig,
		PublicKey: pubkey,
		Address:   from,
	})
	require.NoError(err)
	hash := tf.Hash()
	require.Len(hash, 64)

	client, err := NewClient(asset)
	require.NoError(err)

	submitReq, err := xctypes.SubmitTxReqFromTx(xc.APTOS, tf)
	require.NoError(err)
	err = client.SubmitTx(s.Ctx, submitReq)
	require.NoError(err)

	// second submit is error
	err = client.SubmitTx(s.Ctx, submitReq)
	require.Error(err)

}

func TestFetchTxInfo(t *testing.T) {
	require := require.New(t)

	vectors := []struct {
		tx              string
		resp            interface{}
		val             txinfo.LegacyTxInfo
		err             string
		httpStatusCodes []int
	}{
		{
			// 1.234 APTOS on old coin-store token standard
			tx: "0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3222",
			resp: []string{
				`{"chain_id":58,"epoch":"61","ledger_version":"3524910","oldest_ledger_version":"0","ledger_timestamp":"1683057860656414","node_role":"full_node","oldest_block_height":"0","block_height":"1317171","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
				`{"version":"3509309","hash":"0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3222","state_change_hash":"0xe0e855e3d08f97fc71a5b41b368800588ac7f8b2e49b29daef4d2577c761fe80","event_root_hash":"0x3846412f44cf58865775791b67093d555c854fbffe153965e325f8744c988a71","state_checkpoint_hash":null,"gas_used":"6","success":true,"vm_status":"Executed successfully","accumulator_root_hash":"0x30c4b395b9da13dfdeb74a341798f20d6c65872594f1e22f8fc734c9378c0747","changes":[{"address":"0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365","state_key_hash":"0xe01499453a6e852f925a06b9e38a8bdf534ef104f757b9d84c45587fadbc87dc","data":{"type":"0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>","data":{"coin":{"value":"100189876100"},"deposit_events":{"counter":"731","guid":{"id":{"addr":"0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365","creation_num":"2"}}},"frozen":false,"withdraw_events":{"counter":"728","guid":{"id":{"addr":"0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365","creation_num":"3"}}}}},"type":"write_resource"},{"address":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","state_key_hash":"0x8f0e7c53d3d2b93d3854528797be26b4be8e98c63f558eed57715518930c7c57","data":{"type":"0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>","data":{"coin":{"value":"876098800"},"deposit_events":{"counter":"10","guid":{"id":{"addr":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","creation_num":"2"}}},"frozen":false,"withdraw_events":{"counter":"2","guid":{"id":{"addr":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","creation_num":"3"}}}}},"type":"write_resource"},{"address":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","state_key_hash":"0xefe1a94a04b9d4f93d082e4d13e33d2139a22674e7af2a9fc3e1dbc5a0d6a65e","data":{"type":"0x1::account::Account","data":{"authentication_key":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","coin_register_events":{"counter":"1","guid":{"id":{"addr":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","creation_num":"0"}}},"guid_creation_num":"4","key_rotation_events":{"counter":"0","guid":{"id":{"addr":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","creation_num":"1"}}},"rotation_capability_offer":{"for":{"vec":[]}},"sequence_number":"2","signer_capability_offer":{"for":{"vec":[]}}}},"type":"write_resource"},{"state_key_hash":"0x6e4b28d40f98a106a65163530924c0dcb40c1349d3aa915d108b4d6cfc1ddb19","handle":"0x1b854694ae746cdbd8d44186ca4929b2b337df21d1c74633be19b2710552fdca","key":"0x0619dc29a0aac8fa146714058e8dd6d2d0f3bdf5f6331907bf91f3acd81e6935","value":"0xeb7691bb4cfe08000100000000000000","data":null,"type":"write_table_item"}],"sender":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","sequence_number":"1","max_gas_amount":"2000","gas_unit_price":"100","expiration_timestamp_secs":"1683055757286067","payload":{"function":"0x1::aptos_account::transfer","type_arguments":[],"arguments":["0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365","123400000"],"type":"entry_function_payload"},"signature":{"public_key":"0xa09bb3957ad788bfcfd3f7c5eda9ab2876ff0de8db38dafdf439cfe3f96673b6","signature":"0xd488cd2fda4ef325c68e3c7503a7075841f5ba08808fa2014407e18680fc3d4f515be9cdf6c619baa0e680990d7aad2f5f066cdba778598b28cc8dc3108f420c","type":"ed25519_signature"},"events":[{"guid":{"creation_number":"3","account_address":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682"},"sequence_number":"1","type":"0x1::coin::WithdrawEvent","data":{"amount":"123400000"}},{"guid":{"creation_number":"2","account_address":"0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365"},"sequence_number":"730","type":"0x1::coin::DepositEvent","data":{"amount":"123400000"}}],"timestamp":"1683055759739669","type":"user_transaction"}`,
				`{"block_height":"1309838","block_hash":"0x77eb1ba86353da0133d76892773ecbf18db68555ada5ab358d451ad23653cc31","block_timestamp":"1683055759739669","first_version":"3509308","last_version":"3509310","transactions":null}`,
				`{"chain_id":58,"epoch":"61","ledger_version":"3524912","oldest_ledger_version":"0","ledger_timestamp":"1683057861003497","node_role":"full_node","oldest_block_height":"0","block_height":"1317172","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
			},
			val: txinfo.LegacyTxInfo{
				TxID:            "0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3222",
				BlockHash:       "3509309",
				LookupId:        "3509309",
				From:            "0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682",
				To:              "0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365",
				ToAlt:           "",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(123400000),
				Fee:             xc.NewAmountBlockchainFromUint64(600),
				BlockIndex:      1309838,
				BlockTime:       1683055759,
				Confirmations:   7334,
				Sources: []*txinfo.LegacyTxInfoEndpoint{{
					Address:         "0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682",
					Amount:          xc.NewAmountBlockchainFromUint64(123400000),
					ContractAddress: "APTOS",
					ContractId:      "0x1::aptos_coin::AptosCoin",
					NativeAsset:     "APTOS",
					Event:           txinfo.NewEventFromIndex(0, txinfo.MovementVariantNative),
				}},
				Destinations: []*txinfo.LegacyTxInfoEndpoint{{
					Address:         "0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365",
					Amount:          xc.NewAmountBlockchainFromUint64(123400000),
					ContractAddress: "APTOS",
					NativeAsset:     "APTOS",
					ContractId:      "0x1::aptos_coin::AptosCoin",
					Event:           txinfo.NewEventFromIndex(1, txinfo.MovementVariantNative),
				}},
			},
		},
		{
			// 10000 of a token on new fungible-asset token standard
			tx: "0x17df567aa2daf8c146a0e9b827415402722b4f3b8178025ea703d9b03dc33f29",
			resp: []string{
				`{"chain_id":58,"epoch":"61","ledger_version":"3524910","oldest_ledger_version":"0","ledger_timestamp":"1683057860656414","node_role":"full_node","oldest_block_height":"0","block_height":"1317171","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
				`{"version":"6717630095","hash":"0x17df567aa2daf8c146a0e9b827415402722b4f3b8178025ea703d9b03dc33f29","state_change_hash":"0x53ed331b0b7fe53d61993ef37fd6696adacd796e9ba16b5b0505703b107c9036","event_root_hash":"0x3f0278519f54ae1d71470c25b9bf42fdda8940b8671605b7c9162a262d0d4b5e","state_checkpoint_hash":null,"gas_used":"551","success":true,"vm_status":"Executed successfully","accumulator_root_hash":"0x82a45dfc98abb34ae5c976cc5ffa72a2a013b211e1ba7dc974a8f1b2394d91dd","changes":[{"address":"0xa","state_key_hash":"0x1db5441d8fa4229c5844f73fd66da4ad8176cb8793d8b3a7f6ca858722030043","data":{"type":"0x1::coin::PairedCoinType","data":{"type":{"account_address":"0x1","module_name":"0x6170746f735f636f696e","struct_name":"0x4170746f73436f696e"}}},"type":"write_resource"},{"address":"0xa","state_key_hash":"0x1db5441d8fa4229c5844f73fd66da4ad8176cb8793d8b3a7f6ca858722030043","data":{"type":"0x1::coin::PairedFungibleAssetRefs","data":{"burn_ref_opt":{"vec":[{"metadata":{"inner":"0xa"}}]},"mint_ref_opt":{"vec":[{"metadata":{"inner":"0xa"}}]},"transfer_ref_opt":{"vec":[{"metadata":{"inner":"0xa"}}]}}},"type":"write_resource"},{"address":"0xa","state_key_hash":"0x1db5441d8fa4229c5844f73fd66da4ad8176cb8793d8b3a7f6ca858722030043","data":{"type":"0x1::fungible_asset::ConcurrentSupply","data":{"current":{"max_value":"340282366920938463463374607431768211455","value":"68059588654519445"}}},"type":"write_resource"},{"address":"0xa","state_key_hash":"0x1db5441d8fa4229c5844f73fd66da4ad8176cb8793d8b3a7f6ca858722030043","data":{"type":"0x1::fungible_asset::Metadata","data":{"decimals":8,"icon_uri":"","name":"Aptos Coin","project_uri":"","symbol":"APT"}},"type":"write_resource"},{"address":"0xa","state_key_hash":"0x1db5441d8fa4229c5844f73fd66da4ad8176cb8793d8b3a7f6ca858722030043","data":{"type":"0x1::object::ObjectCore","data":{"allow_ungated_transfer":true,"guid_creation_num":"1125899906842625","owner":"0x1","transfer_events":{"counter":"0","guid":{"id":{"addr":"0xa","creation_num":"1125899906842624"}}}}},"type":"write_resource"},{"address":"0xa","state_key_hash":"0x1db5441d8fa4229c5844f73fd66da4ad8176cb8793d8b3a7f6ca858722030043","data":{"type":"0x1::primary_fungible_store::DeriveRefPod","data":{"metadata_derive_ref":{"self":"0xa"}}},"type":"write_resource"},{"address":"0x9b1be2b34453eeefaf428a1e50b6ca8f1b333c01a523a0b093ef61ea4777eeb","state_key_hash":"0x5f21a06fca23d83d8eb770d6f4ad20cfd119fc7f810de70272c383819aa2b4c4","data":{"type":"0x1::fungible_asset::FungibleStore","data":{"balance":"399839400","frozen":false,"metadata":{"inner":"0xa"}}},"type":"write_resource"},{"address":"0x9b1be2b34453eeefaf428a1e50b6ca8f1b333c01a523a0b093ef61ea4777eeb","state_key_hash":"0x5f21a06fca23d83d8eb770d6f4ad20cfd119fc7f810de70272c383819aa2b4c4","data":{"type":"0x1::object::ObjectCore","data":{"allow_ungated_transfer":false,"guid_creation_num":"1125899906842625","owner":"0x5249a0f1ccb427e6595343ef001ec18765fd325beb70fbea0a9c25807167e60d","transfer_events":{"counter":"0","guid":{"id":{"addr":"0x9b1be2b34453eeefaf428a1e50b6ca8f1b333c01a523a0b093ef61ea4777eeb","creation_num":"1125899906842624"}}}}},"type":"write_resource"},{"address":"0x448c83c716bd1144173c0522f108ff507bb8f4ddbe34a925bf3e1d7ea2632cfa","state_key_hash":"0x2288c3a5537388991d3a91b2b89b673b17aefdd4f738c3d77fbfcd271a96cdf7","data":{"type":"0x1::coin::CoinInfo<0x448c83c716bd1144173c0522f108ff507bb8f4ddbe34a925bf3e1d7ea2632cfa::test_faucet::TestFaucetCoin>","data":{"decimals":6,"name":"Test Coin","supply":{"vec":[{"aggregator":{"vec":[]},"integer":{"vec":[{"limit":"340282366920938463463374607431768211455","value":"18000000"}]}}]},"symbol":"TC"}},"type":"write_resource"},{"address":"0x5249a0f1ccb427e6595343ef001ec18765fd325beb70fbea0a9c25807167e60d","state_key_hash":"0xfc1d25fd25a4b7a3b2a808c0a6b3c7aebc9c28a5b82f95624ed1d0ca74b57a0c","data":{"type":"0x1::account::Account","data":{"authentication_key":"0x5249a0f1ccb427e6595343ef001ec18765fd325beb70fbea0a9c25807167e60d","coin_register_events":{"counter":"0","guid":{"id":{"addr":"0x5249a0f1ccb427e6595343ef001ec18765fd325beb70fbea0a9c25807167e60d","creation_num":"0"}}},"guid_creation_num":"2","key_rotation_events":{"counter":"0","guid":{"id":{"addr":"0x5249a0f1ccb427e6595343ef001ec18765fd325beb70fbea0a9c25807167e60d","creation_num":"1"}}},"rotation_capability_offer":{"for":{"vec":[]}},"sequence_number":"7","signer_capability_offer":{"for":{"vec":[]}}}},"type":"write_resource"},{"address":"0x7a4842208fb122689ca0f33bdb43a6a6d1fc9af24fedff2f38646a5a95eb442e","state_key_hash":"0x35cc1e6c2cfcf4cd356309a220b042e03fad26cc245cdb8d3d99a5a8c07b5d2e","data":{"type":"0x1::fungible_asset::FungibleStore","data":{"balance":"4990000","frozen":false,"metadata":{"inner":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20"}}},"type":"write_resource"},{"address":"0x7a4842208fb122689ca0f33bdb43a6a6d1fc9af24fedff2f38646a5a95eb442e","state_key_hash":"0x35cc1e6c2cfcf4cd356309a220b042e03fad26cc245cdb8d3d99a5a8c07b5d2e","data":{"type":"0x1::object::ObjectCore","data":{"allow_ungated_transfer":false,"guid_creation_num":"1125899906842625","owner":"0x5249a0f1ccb427e6595343ef001ec18765fd325beb70fbea0a9c25807167e60d","transfer_events":{"counter":"0","guid":{"id":{"addr":"0x7a4842208fb122689ca0f33bdb43a6a6d1fc9af24fedff2f38646a5a95eb442e","creation_num":"1125899906842624"}}}}},"type":"write_resource"},{"address":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20","state_key_hash":"0xd66a9c67279a27cf56cfaeb9b364b8acc602e62cd3779db854451b2e22e30498","data":{"type":"0x1::coin::PairedCoinType","data":{"type":{"account_address":"0x448c83c716bd1144173c0522f108ff507bb8f4ddbe34a925bf3e1d7ea2632cfa","module_name":"0x746573745f666175636574","struct_name":"0x54657374466175636574436f696e"}}},"type":"write_resource"},{"address":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20","state_key_hash":"0xd66a9c67279a27cf56cfaeb9b364b8acc602e62cd3779db854451b2e22e30498","data":{"type":"0x1::coin::PairedFungibleAssetRefs","data":{"burn_ref_opt":{"vec":[{"metadata":{"inner":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20"}}]},"mint_ref_opt":{"vec":[{"metadata":{"inner":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20"}}]},"transfer_ref_opt":{"vec":[{"metadata":{"inner":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20"}}]}}},"type":"write_resource"},{"address":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20","state_key_hash":"0xd66a9c67279a27cf56cfaeb9b364b8acc602e62cd3779db854451b2e22e30498","data":{"type":"0x1::fungible_asset::ConcurrentSupply","data":{"current":{"max_value":"340282366920938463463374607431768211455","value":"17000000"}}},"type":"write_resource"},{"address":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20","state_key_hash":"0xd66a9c67279a27cf56cfaeb9b364b8acc602e62cd3779db854451b2e22e30498","data":{"type":"0x1::fungible_asset::Metadata","data":{"decimals":6,"icon_uri":"","name":"Test Coin","project_uri":"","symbol":"TC"}},"type":"write_resource"},{"address":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20","state_key_hash":"0xd66a9c67279a27cf56cfaeb9b364b8acc602e62cd3779db854451b2e22e30498","data":{"type":"0x1::object::ObjectCore","data":{"allow_ungated_transfer":true,"guid_creation_num":"1125899906842625","owner":"0xa","transfer_events":{"counter":"0","guid":{"id":{"addr":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20","creation_num":"1125899906842624"}}}}},"type":"write_resource"},{"address":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20","state_key_hash":"0xd66a9c67279a27cf56cfaeb9b364b8acc602e62cd3779db854451b2e22e30498","data":{"type":"0x1::primary_fungible_store::DeriveRefPod","data":{"metadata_derive_ref":{"self":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20"}}},"type":"write_resource"},{"address":"0xf739abb65036bfbc1d12653549394392eeaa9f67f24eeb22f69d81f5c053702a","state_key_hash":"0xb4b8d776a1ef9774b015257aacaf352726e743bf565a137edcbc6a60afc9a341","data":{"type":"0x1::fungible_asset::FungibleStore","data":{"balance":"10000","frozen":false,"metadata":{"inner":"0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20"}}},"type":"write_resource"},{"address":"0xf739abb65036bfbc1d12653549394392eeaa9f67f24eeb22f69d81f5c053702a","state_key_hash":"0xb4b8d776a1ef9774b015257aacaf352726e743bf565a137edcbc6a60afc9a341","data":{"type":"0x1::object::ObjectCore","data":{"allow_ungated_transfer":false,"guid_creation_num":"1125899906842625","owner":"0xb7c348a3253a8102e354b21972319600f0933dd454db12d3e2fc81623ffa49be","transfer_events":{"counter":"0","guid":{"id":{"addr":"0xf739abb65036bfbc1d12653549394392eeaa9f67f24eeb22f69d81f5c053702a","creation_num":"1125899906842624"}}}}},"type":"write_resource"}],"sender":"0x5249a0f1ccb427e6595343ef001ec18765fd325beb70fbea0a9c25807167e60d","sequence_number":"6","max_gas_amount":"716","gas_unit_price":"100","expiration_timestamp_secs":"1746985549280360","payload":{"function":"0x1::aptos_account::transfer_coins","type_arguments":["0x448c83c716bd1144173c0522f108ff507bb8f4ddbe34a925bf3e1d7ea2632cfa::test_faucet::TestFaucetCoin"],"arguments":["0xb7c348a3253a8102e354b21972319600f0933dd454db12d3e2fc81623ffa49be","10000"],"type":"entry_function_payload"},"signature":{"public_key":"0x6e6335ae12bf871f4e4d066508aa5965cfa20111b14cf5278d6bb3fc63ff78db","signature":"0x89b71d1531db790e9c7b1e09f6c7a5f53e8ae1d6c75e0450654c6a88745569eec56985f115a3e6b6b84c3ee8ea9f988b1e3cf50caab89b88a9e80befb3da350a","type":"ed25519_signature"},"events":[{"guid":{"creation_number":"0","account_address":"0x0"},"sequence_number":"0","type":"0x1::fungible_asset::Withdraw","data":{"amount":"10000","store":"0x7a4842208fb122689ca0f33bdb43a6a6d1fc9af24fedff2f38646a5a95eb442e"}},{"guid":{"creation_number":"0","account_address":"0x0"},"sequence_number":"0","type":"0x1::fungible_asset::Deposit","data":{"amount":"10000","store":"0xf739abb65036bfbc1d12653549394392eeaa9f67f24eeb22f69d81f5c053702a"}},{"guid":{"creation_number":"0","account_address":"0x0"},"sequence_number":"0","type":"0x1::transaction_fee::FeeStatement","data":{"execution_gas_units":"8","io_gas_units":"12","storage_fee_octas":"53240","storage_fee_refund_octas":"0","total_charge_gas_units":"551"}}],"timestamp":"1746985555790511","type":"user_transaction"}`,
				`{"block_height":"1309838","block_hash":"0x77eb1ba86353da0133d76892773ecbf18db68555ada5ab358d451ad23653cc31","block_timestamp":"1683055759739669","first_version":"3509308","last_version":"3509310","transactions":null}`,
				`{"chain_id":58,"epoch":"61","ledger_version":"3524912","oldest_ledger_version":"0","ledger_timestamp":"1683057861003497","node_role":"full_node","oldest_block_height":"0","block_height":"1317172","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
			},
			val: txinfo.LegacyTxInfo{
				TxID:            "0x17df567aa2daf8c146a0e9b827415402722b4f3b8178025ea703d9b03dc33f29",
				BlockHash:       "6717630095",
				LookupId:        "6717630095",
				From:            "0x5249a0f1ccb427e6595343ef001ec18765fd325beb70fbea0a9c25807167e60d",
				To:              "0xb7c348a3253a8102e354b21972319600f0933dd454db12d3e2fc81623ffa49be",
				ToAlt:           "",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(10000),
				Fee:             xc.NewAmountBlockchainFromUint64(55100),
				BlockIndex:      1309838,
				BlockTime:       1746985555,
				Confirmations:   7334,
				Sources: []*txinfo.LegacyTxInfoEndpoint{{
					Address:         "0x5249a0f1ccb427e6595343ef001ec18765fd325beb70fbea0a9c25807167e60d",
					Amount:          xc.NewAmountBlockchainFromUint64(10000),
					ContractAddress: "0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20",
					NativeAsset:     "APTOS",
					Event:           txinfo.NewEventFromIndex(0, txinfo.MovementVariantNative),
				}},
				Destinations: []*txinfo.LegacyTxInfoEndpoint{{
					Address:         "0xb7c348a3253a8102e354b21972319600f0933dd454db12d3e2fc81623ffa49be",
					Amount:          xc.NewAmountBlockchainFromUint64(10000),
					ContractAddress: "0x971225a0feba8d24b7658bc2e2e3155ccdb17eff3e2ef320d1428879f10ebf20",
					NativeAsset:     "APTOS",
					Event:           txinfo.NewEventFromIndex(1, txinfo.MovementVariantNative),
				}},
			},
		},
		// not found
		{
			tx: "0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3221",
			resp: []string{
				`{"chain_id":58,"epoch":"61","ledger_version":"3532090","oldest_ledger_version":"0","ledger_timestamp":"1683058921700697","node_role":"full_node","oldest_block_height":"0","block_height":"1320608","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
				`{"message":"Transaction not found by Transaction hash(0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3221)","error_code":"transaction_not_found","vm_error_code":null}`,
			},
			val:             txinfo.LegacyTxInfo{},
			err:             "TransactionNotFound: Transaction not found by Transaction",
			httpStatusCodes: []int{404, 404, 404, 404, 404},
		},
		// multisend
		{
			tx: "0x06a6c25f3601895a3d3330f6ba4696fb8e677973aa56aa7b0ea362915bcff39c",
			resp: []string{
				`{"chain_id":1,"epoch":"3257","ledger_version":"177862741","oldest_ledger_version":"0","ledger_timestamp":"1689016648102542","node_role":"full_node","oldest_block_height":"0","block_height":"68013901","git_hash":"f43ce082abbdaa3a8b38ac07d928feed4248eb73"}`,
				`{"version":"176278674","hash":"0x06a6c25f3601895a3d3330f6ba4696fb8e677973aa56aa7b0ea362915bcff39c","state_change_hash":"0x54c0f9f65c09363bac0084508611955ef3b2620ed87b7eb21e5112dd3ad5c01b","event_root_hash":"0xb30f3ea2b5b1205124765f0d44f7781cc11d78fffcb641e1db0ab0e3c4b427ec","state_checkpoint_hash":null,"gas_used":"6","success":true,"vm_status":"Executed successfully","accumulator_root_hash":"0x1e65441a0d757bed561c9083ac7d21b952e21094cb70e31590d886c7ef4df0fb","changes":[{"address":"0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f","state_key_hash":"0x6363246fdc38ed0edfc16cb286ebd2655557f2f1ef51514e085916112abbeaae","data":{"type":"0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>","data":{"coin":{"value":"15665049485514"},"deposit_events":{"counter":"332","guid":{"id":{"addr":"0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f","creation_num":"2"}}},"frozen":false,"withdraw_events":{"counter":"100197","guid":{"id":{"addr":"0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f","creation_num":"3"}}}}},"type":"write_resource"},{"address":"0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f","state_key_hash":"0xe593d185e2822f4e8db7136902bcb36da32cffa54db72febd1602d34c4c9ef31","data":{"type":"0x1::account::Account","data":{"authentication_key":"0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f","coin_register_events":{"counter":"2","guid":{"id":{"addr":"0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f","creation_num":"0"}}},"guid_creation_num":"6","key_rotation_events":{"counter":"0","guid":{"id":{"addr":"0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f","creation_num":"1"}}},"rotation_capability_offer":{"for":{"vec":[]}},"sequence_number":"93926","signer_capability_offer":{"for":{"vec":[]}}}},"type":"write_resource"},{"address":"0xcdaa56944a811c22398165b6c885b8aaad39fe7b91b008bb6334d639cbaec8f7","state_key_hash":"0x389b85a47be61dbc4152278b89759d8e5605e0ac5079f0b2245611a29f1f9964","data":{"type":"0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>","data":{"coin":{"value":"35183208052790"},"deposit_events":{"counter":"1575","guid":{"id":{"addr":"0xcdaa56944a811c22398165b6c885b8aaad39fe7b91b008bb6334d639cbaec8f7","creation_num":"2"}}},"frozen":false,"withdraw_events":{"counter":"1154","guid":{"id":{"addr":"0xcdaa56944a811c22398165b6c885b8aaad39fe7b91b008bb6334d639cbaec8f7","creation_num":"3"}}}}},"type":"write_resource"},{"state_key_hash":"0x6e4b28d40f98a106a65163530924c0dcb40c1349d3aa915d108b4d6cfc1ddb19","handle":"0x1b854694ae746cdbd8d44186ca4929b2b337df21d1c74633be19b2710552fdca","key":"0x0619dc29a0aac8fa146714058e8dd6d2d0f3bdf5f6331907bf91f3acd81e6935","value":"0xa053b8c62ec572010000000000000000","data":null,"type":"write_table_item"}],"sender":"0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f","sequence_number":"93925","max_gas_amount":"100000","gas_unit_price":"100","expiration_timestamp_secs":"1688879062","payload":{"function":"0x1::aptos_account::batch_transfer_coins","type_arguments":["0x1::aptos_coin::AptosCoin"],"arguments":[["0xcdaa56944a811c22398165b6c885b8aaad39fe7b91b008bb6334d639cbaec8f7"],["140099000000"]],"type":"entry_function_payload"},"signature":{"public_key":"0x803c8816dce93b76daf22ee1c96410a4a23b47026e8dd3bdd6c6f2794b6c8e05","signature":"0x22e2668d051c9c78c0a5931d10823eccf507cc23026950ecc9f4021f66d5a48de0ffbb94b28decc881c841261a0403d99769e1cbcc321585936dcd96dc83c300","type":"ed25519_signature"},"events":[{"guid":{"creation_number":"3","account_address":"0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f"},"sequence_number":"100196","type":"0x1::coin::WithdrawEvent","data":{"amount":"140099000000"}},{"guid":{"creation_number":"2","account_address":"0xcdaa56944a811c22398165b6c885b8aaad39fe7b91b008bb6334d639cbaec8f7"},"sequence_number":"1574","type":"0x1::coin::DepositEvent","data":{"amount":"140099000000"}}],"timestamp":"1688879042654550","type":"user_transaction"}`,
				`{"block_height":"67467400","block_hash":"0x65a99fc435368aef999ebd6ce962347ae15adb659de5a5e4f4fd3e5736eead70","block_timestamp":"1688879042654550","first_version":"176278669","last_version":"176278675","transactions":null}`,
				`{"chain_id":1,"epoch":"3257","ledger_version":"177862741","oldest_ledger_version":"0","ledger_timestamp":"1689016648102542","node_role":"full_node","oldest_block_height":"0","block_height":"68013901","git_hash":"f43ce082abbdaa3a8b38ac07d928feed4248eb73"}`,
			},
			val: txinfo.LegacyTxInfo{
				TxID:            "0x06a6c25f3601895a3d3330f6ba4696fb8e677973aa56aa7b0ea362915bcff39c",
				BlockHash:       "176278674",
				From:            "0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f",
				To:              "0xcdaa56944a811c22398165b6c885b8aaad39fe7b91b008bb6334d639cbaec8f7",
				LookupId:        "176278674",
				ToAlt:           "",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(140099000000),
				Fee:             xc.NewAmountBlockchainFromUint64(600),
				BlockIndex:      67467400,
				BlockTime:       1688879042,
				Confirmations:   546501,
				Sources: []*txinfo.LegacyTxInfoEndpoint{{
					Address:         "0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f",
					Amount:          xc.NewAmountBlockchainFromUint64(140099000000),
					NativeAsset:     "APTOS",
					ContractAddress: "APTOS",
					ContractId:      "0x1::aptos_coin::AptosCoin",
					Event:           txinfo.NewEventFromIndex(0, txinfo.MovementVariantNative),
				}},
				Destinations: []*txinfo.LegacyTxInfoEndpoint{{
					Address:         "0xcdaa56944a811c22398165b6c885b8aaad39fe7b91b008bb6334d639cbaec8f7",
					Amount:          xc.NewAmountBlockchainFromUint64(140099000000),
					NativeAsset:     "APTOS",
					ContractAddress: "APTOS",
					ContractId:      "0x1::aptos_coin::AptosCoin",
					Event:           txinfo.NewEventFromIndex(1, txinfo.MovementVariantNative),
				}},
			},
			err: "",
		},
		{
			// 1.234 APTOS failed transaction
			tx: "0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3222",
			resp: []string{
				`{"chain_id":58,"epoch":"61","ledger_version":"3524910","oldest_ledger_version":"0","ledger_timestamp":"1683057860656414","node_role":"full_node","oldest_block_height":"0","block_height":"1317171","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
				`{"version":"3509309","hash":"0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3222","state_change_hash":"0xe0e855e3d08f97fc71a5b41b368800588ac7f8b2e49b29daef4d2577c761fe80","event_root_hash":"0x3846412f44cf58865775791b67093d555c854fbffe153965e325f8744c988a71","state_checkpoint_hash":null,"gas_used":"6","success":false,"vm_status":"wrong object type","accumulator_root_hash":"0x30c4b395b9da13dfdeb74a341798f20d6c65872594f1e22f8fc734c9378c0747","changes":[],"sender":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","sequence_number":"1","max_gas_amount":"2000","gas_unit_price":"100","expiration_timestamp_secs":"1683055757286067","payload":{"function":"0x1::aptos_account::transfer","type_arguments":[],"arguments":["0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365","123400000"],"type":"entry_function_payload"},"signature":{"public_key":"0xa09bb3957ad788bfcfd3f7c5eda9ab2876ff0de8db38dafdf439cfe3f96673b6","signature":"0xd488cd2fda4ef325c68e3c7503a7075841f5ba08808fa2014407e18680fc3d4f515be9cdf6c619baa0e680990d7aad2f5f066cdba778598b28cc8dc3108f420c","type":"ed25519_signature"},"events":[{"guid":{"creation_number":"3","account_address":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682"},"sequence_number":"1","type":"0x1::coin::WithdrawEvent","data":{"amount":"123400000"}},{"guid":{"creation_number":"2","account_address":"0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365"},"sequence_number":"730","type":"0x1::coin::DepositEvent","data":{"amount":"123400000"}}],"timestamp":"1683055759739669","type":"user_transaction"}`,
				`{"block_height":"1309838","block_hash":"0x77eb1ba86353da0133d76892773ecbf18db68555ada5ab358d451ad23653cc31","block_timestamp":"1683055759739669","first_version":"3509308","last_version":"3509310","transactions":null}`,
				`{"chain_id":58,"epoch":"61","ledger_version":"3524912","oldest_ledger_version":"0","ledger_timestamp":"1683057861003497","node_role":"full_node","oldest_block_height":"0","block_height":"1317172","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
			},
			val: txinfo.LegacyTxInfo{
				TxID:            "0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3222",
				BlockHash:       "3509309",
				LookupId:        "3509309",
				From:            "0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682",
				To:              "",
				ToAlt:           "",
				ContractAddress: "",
				Fee:             xc.NewAmountBlockchainFromUint64(600),
				BlockIndex:      1309838,
				BlockTime:       1683055759,
				Confirmations:   7334,
				Sources:         nil,
				Destinations:    nil,
				Error:           "wrong object type",
			},
		},
	}

	for i, v := range vectors {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			resp := `{"chain_id":38,"epoch":"133","ledger_version":"13087045","oldest_ledger_version":"0","ledger_timestamp":"1669676013555573","node_role":"full_node","oldest_block_height":"0","block_height":"5435983","git_hash":"2c74a456298fcd520241a562119b6fe30abdaae2"}`
			server, close := testtypes.MockHTTP(t, resp, 0)

			asset := xc.NewChainConfig("APTOS").WithNet("devnet")
			asset.NativeAssets = []*xc.AdditionalNativeAsset{
				{
					AssetId:    "APTOS",
					ContractId: "0x1::aptos_coin::AptosCoin",
					Aliases:    []string{"0xa"},
					Decimals:   8,
				},
			}
			asset.URL = server.URL
			client, _ := NewClient(asset)
			server.StatusCodes = v.httpStatusCodes
			server.Response = v.resp
			txInfo, err := client.FetchLegacyTxInfo(context.Background(), xc.TxHash(v.tx))

			if v.err != "" {
				require.Equal(txinfo.LegacyTxInfo{}, txInfo)
				require.ErrorContains(err, v.err)
			} else {
				require.Nil(err)
				require.NotNil(txInfo)
				require.Equal(v.val, txInfo)
			}
			close()
		})
	}
}

func TestFetchBalance(t *testing.T) {
	require := require.New(t)
	vectors := []struct {
		asset    xc.ITask
		contract xc.ContractAddress
		resp     interface{}
		val      string
		err      string
	}{
		{
			asset: xc.NewChainConfig(""),
			resp:  `1000000`,
			val:   "1000000",
			err:   "",
		},
	}

	for i, v := range vectors {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			resp := `{"chain_id":38,"epoch":"133","ledger_version":"13087045","oldest_ledger_version":"0","ledger_timestamp":"1669676013555573","node_role":"full_node","oldest_block_height":"0","block_height":"5435983","git_hash":"2c74a456298fcd520241a562119b6fe30abdaae2"}`
			server, close := testtypes.MockHTTP(t, resp, 200)
			defer close()

			asset := v.asset
			asset.GetChain().URL = server.URL
			rpcClient, _ := NewClient(asset)
			if v.err != "" {
				// errors should return 400 status code.
				server.StatusCodes = []int{400, 400, 400}
			}
			server.Response = v.resp
			from := xc.Address("0xa589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab85")
			fmt.Println(v.asset)
			args := client.NewBalanceArgs(from)
			if v.contract != "" {
				args.SetContract(v.contract)
			}
			balance, err := rpcClient.FetchBalance(context.Background(), args)

			if v.err != "" {
				require.Equal("0", balance.String())
				require.ErrorContains(err, v.err)
			} else {
				require.NoError(err)
				require.NotNil(balance)
				require.Equal(v.val, balance.String())
			}
		})
	}
}

func (s *AptosTestSuite) TestNewNativeTransfer() {
	require := s.Require()

	asset := xc.NewChainConfig("APTOS").WithNet("devnet")
	builder, _ := NewTxBuilder(asset.Base())
	from := xc.Address("0xa589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab85")
	to := xc.Address("0xbb89a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab00")
	amount := xc.NewAmountBlockchainFromUint64(1)
	pubkey := []byte{1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8}
	input := &tx_input.TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverAptos),
		SequenceNumber:  3,
		GasLimit:        2000,
		GasPrice:        10,
		Timestamp:       12345,
		ChainId:         1,
	}
	args := buildertest.MustNewTransferArgs(asset.ChainBaseConfig, from, to, amount, buildertest.OptionPublicKey(pubkey))
	tf, err := builder.Transfer(args, input)
	require.NoError(err)
	require.NotNil(tf)

	// add signature
	sig := []byte{}
	for i := 0; i < 64; i++ {
		sig = append(sig, byte(i))
	}
	err = tf.SetSignatures(&xc.SignatureResponse{
		Signature: sig,
		PublicKey: pubkey,
		Address:   from,
	})
	require.NoError(err)
	hash := tf.Hash()
	require.Len(hash, 64)

	ser, err := tf.Serialize()
	require.NoError(err)
	require.True(len(ser) > 64)
	require.Equal("a589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab8503000000000000000200000000000000000000000000000000000000000000000000000000000000010d6170746f735f6163636f756e74087472616e73666572000220bb89a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab00080100000000000000d0070000000000000a00000000000000493e000000000000010020010203040506070801020304050607080102030405060708010203040506070840000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f", hex.EncodeToString(ser))
}

func (s *AptosTestSuite) TestNewTokenTransfer() {
	require := s.Require()

	native_asset := xc.NewChainConfig("APTOS").WithNet("devnet")
	builder, _ := NewTxBuilder(native_asset.Base())
	from := xc.Address("0xa589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab85")
	to := xc.Address("0xbb89a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab00")
	amount := xc.NewAmountBlockchainFromUint64(1)
	pubkey := []byte{1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8}
	input := &tx_input.TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverAptos),
		SequenceNumber:  3,
		GasLimit:        2000,
		GasPrice:        10,
		Timestamp:       12345,
		ChainId:         1,
	}
	args := buildertest.MustNewTransferArgs(
		native_asset.ChainBaseConfig, from, to, amount,
		buildertest.OptionContractAddress("0x1::Coin::USDC"),
		buildertest.OptionPublicKey(pubkey),
	)
	tf, err := builder.Transfer(args, input)
	require.NoError(err)
	require.NotNil(tf)

	// add signature
	sig := []byte{}
	for i := 0; i < 64; i++ {
		sig = append(sig, byte(i))
	}
	err = tf.SetSignatures(&xc.SignatureResponse{
		Signature: sig,
		PublicKey: pubkey,
		Address:   from,
	})
	require.NoError(err)
	hash := tf.Hash()
	require.Len(hash, 64)

	ser, err := tf.Serialize()
	require.NoError(err)
	require.True(len(ser) > 64)
	require.Equal(
		"a589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab8503000000000000000200000000000000000000000000000000000000000000000000000000000000010d6170746f735f6163636f756e740e7472616e736665725f636f696e730107000000000000000000000000000000000000000000000000000000000000000104436f696e0455534443000220bb89a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab00080100000000000000d0070000000000000a000000000000009984000000000000010020010203040506070801020304050607080102030405060708010203040506070840000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f",
		hex.EncodeToString(ser),
	)

	// Use new fungible asset contract address
	tf, err = builder.NewTokenTransfer(from, from, to, amount, xc.ContractAddress("0x112233445566778899112233445566778899112233445566778899"), input)
	require.NoError(err)
	err = tf.SetSignatures(&xc.SignatureResponse{
		Signature: sig,
		PublicKey: pubkey,
		Address:   from,
	})
	require.NoError(err)
	ser2, err := tf.Serialize()
	require.NoError(err)
	require.Equal(
		"a589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab8503000000000000000200000000000000000000000000000000000000000000000000000000000000010d6170746f735f6163636f756e74187472616e736665725f66756e6769626c655f617373657473000320000000000011223344556677889911223344556677889911223344556677889920bb89a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab00080100000000000000d0070000000000000a000000000000009984000000000000010020010203040506070801020304050607080102030405060708010203040506070840000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f",
		hex.EncodeToString(ser2),
	)
	require.NotEqual(ser, ser2)
}

func (s *AptosTestSuite) TestFeePayerNewTokenTransfer() {
	require := s.Require()

	native_asset := xc.NewChainConfig("APTOS").WithNet("devnet")
	builder, _ := NewTxBuilder(native_asset.Base())
	from := xc.Address("0xa589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab85")
	to := xc.Address("0xbb89a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab00")
	feePayer := xc.Address("0x8ba6e5f0fd111dc60c5ad827c7f4110930f22a483a6697b7f888df0057e9b19")
	amount := xc.NewAmountBlockchainFromUint64(1)
	pubkey := []byte{1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 8}
	input := &tx_input.TxInput{
		TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverAptos),
		SequenceNumber:  3,
		GasLimit:        2000,
		GasPrice:        10,
		Timestamp:       12345,
		ChainId:         1,
	}
	args := buildertest.MustNewTransferArgs(
		native_asset.ChainBaseConfig,
		from, to, amount,
		buildertest.OptionContractAddress("0x1::Coin::USDC"),
		buildertest.OptionFeePayer(feePayer, []byte{}),
		buildertest.OptionPublicKey(pubkey),
	)
	tf, err := builder.Transfer(args, input)
	require.NoError(err)
	require.NotNil(tf)

	// add signature
	sig := []byte{}
	for i := 0; i < 64; i++ {
		sig = append(sig, byte(i))
	}
	err = tf.SetSignatures(&xc.SignatureResponse{
		Signature: sig,
		PublicKey: pubkey,
		Address:   from,
	}, &xc.SignatureResponse{
		Signature: sig,
		PublicKey: pubkey,
		Address:   feePayer,
	})
	require.NoError(err)

	hash := tf.Hash()
	require.Len(hash, 64)

	ser, err := tf.Serialize()
	require.NoError(err)
	require.True(len(ser) > 64)
	require.Equal(
		"a589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab8503000000000000000200000000000000000000000000000000000000000000000000000000000000010d6170746f735f6163636f756e740e7472616e736665725f636f696e730107000000000000000000000000000000000000000000000000000000000000000104436f696e0455534443000220bb89a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab00080100000000000000d0070000000000000a00000000000000998400000000000001030020010203040506070801020304050607080102030405060708010203040506070840000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f000008ba6e5f0fd111dc60c5ad827c7f4110930f22a483a6697b7f888df0057e9b190020010203040506070801020304050607080102030405060708010203040506070840000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f",
		hex.EncodeToString(ser),
	)
}
