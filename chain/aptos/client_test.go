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
	xclient "github.com/cordialsys/crosschain/client"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
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

func (s *AptosTestSuite) TestFetchTxInput() {
	require := s.Require()

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
			},
			from: "0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682",
			input: &tx_input.TxInput{
				TxInputEnvelope: *xc.NewTxInputEnvelope(xc.DriverAptos),
				SequenceNumber:  2,
				GasLimit:        1000,
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
			},
			from:  "0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f680",
			input: &tx_input.TxInput{},
			err:   "Account not found",
		},
	}

	for _, v := range vectors {

		resp := `{"chain_id":38,"epoch":"133","ledger_version":"13087045","oldest_ledger_version":"0","ledger_timestamp":"1669676013555573","node_role":"full_node","oldest_block_height":"0","block_height":"5435983","git_hash":"2c74a456298fcd520241a562119b6fe30abdaae2"}`

		// satisfy the gas estimate to go with default value
		blockNotFound := `{"message":"block not found","error_code":"block_not_found","vm_error_code":null}`
		for i := 0; i < 20; i++ {
			v.resp = append(v.resp, blockNotFound)
		}
		server, close := testtypes.MockHTTP(s.T(), resp, 200)
		asset := v.asset.GetChain()
		asset.URL = server.URL
		client, _ := NewClient(asset)
		if v.err != "" {
			// errors should return 400 status code.
			server.StatusCodes = []int{200, 200, 400}
		}
		server.Response = v.resp
		input, err := client.FetchLegacyTxInput(s.Ctx, xc.Address(v.from), "")

		if v.err != "" {
			require.ErrorContains(err, v.err)
		} else {
			require.NoError(err)
			require.NotNil(input)
			require.Equal(v.input, input)
		}
		close()
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
		Pubkey:          pubkey,
	}
	args := buildertest.MustNewTransferArgs(from, to, amount)
	tf, err := builder.Transfer(args, input)
	require.NoError(err)
	require.NotNil(tf)
	hash := tf.Hash()
	require.Len(hash, 64)

	// add signature
	sig := []byte{}
	for i := 0; i < 64; i++ {
		sig = append(sig, byte(i))
	}
	err = tf.AddSignatures(xc.TxSignature(sig))
	require.NoError(err)

	client, err := NewClient(asset)
	require.NoError(err)
	err = client.SubmitTx(s.Ctx, tf)
	require.NoError(err)

	// second submit is error
	err = client.SubmitTx(s.Ctx, tf)
	require.Error(err)

}

func TestFetchTxInfo(t *testing.T) {
	require := require.New(t)

	vectors := []struct {
		tx              string
		resp            interface{}
		val             xclient.LegacyTxInfo
		err             string
		httpStatusCodes []int
	}{
		{
			// 1.234 APTOS
			tx: "0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3222",
			resp: []string{
				`{"chain_id":58,"epoch":"61","ledger_version":"3524910","oldest_ledger_version":"0","ledger_timestamp":"1683057860656414","node_role":"full_node","oldest_block_height":"0","block_height":"1317171","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
				`{"version":"3509309","hash":"0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3222","state_change_hash":"0xe0e855e3d08f97fc71a5b41b368800588ac7f8b2e49b29daef4d2577c761fe80","event_root_hash":"0x3846412f44cf58865775791b67093d555c854fbffe153965e325f8744c988a71","state_checkpoint_hash":null,"gas_used":"6","success":true,"vm_status":"Executed successfully","accumulator_root_hash":"0x30c4b395b9da13dfdeb74a341798f20d6c65872594f1e22f8fc734c9378c0747","changes":[{"address":"0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365","state_key_hash":"0xe01499453a6e852f925a06b9e38a8bdf534ef104f757b9d84c45587fadbc87dc","data":{"type":"0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>","data":{"coin":{"value":"100189876100"},"deposit_events":{"counter":"731","guid":{"id":{"addr":"0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365","creation_num":"2"}}},"frozen":false,"withdraw_events":{"counter":"728","guid":{"id":{"addr":"0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365","creation_num":"3"}}}}},"type":"write_resource"},{"address":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","state_key_hash":"0x8f0e7c53d3d2b93d3854528797be26b4be8e98c63f558eed57715518930c7c57","data":{"type":"0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>","data":{"coin":{"value":"876098800"},"deposit_events":{"counter":"10","guid":{"id":{"addr":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","creation_num":"2"}}},"frozen":false,"withdraw_events":{"counter":"2","guid":{"id":{"addr":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","creation_num":"3"}}}}},"type":"write_resource"},{"address":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","state_key_hash":"0xefe1a94a04b9d4f93d082e4d13e33d2139a22674e7af2a9fc3e1dbc5a0d6a65e","data":{"type":"0x1::account::Account","data":{"authentication_key":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","coin_register_events":{"counter":"1","guid":{"id":{"addr":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","creation_num":"0"}}},"guid_creation_num":"4","key_rotation_events":{"counter":"0","guid":{"id":{"addr":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","creation_num":"1"}}},"rotation_capability_offer":{"for":{"vec":[]}},"sequence_number":"2","signer_capability_offer":{"for":{"vec":[]}}}},"type":"write_resource"},{"state_key_hash":"0x6e4b28d40f98a106a65163530924c0dcb40c1349d3aa915d108b4d6cfc1ddb19","handle":"0x1b854694ae746cdbd8d44186ca4929b2b337df21d1c74633be19b2710552fdca","key":"0x0619dc29a0aac8fa146714058e8dd6d2d0f3bdf5f6331907bf91f3acd81e6935","value":"0xeb7691bb4cfe08000100000000000000","data":null,"type":"write_table_item"}],"sender":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","sequence_number":"1","max_gas_amount":"2000","gas_unit_price":"100","expiration_timestamp_secs":"1683055757286067","payload":{"function":"0x1::aptos_account::transfer","type_arguments":[],"arguments":["0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365","123400000"],"type":"entry_function_payload"},"signature":{"public_key":"0xa09bb3957ad788bfcfd3f7c5eda9ab2876ff0de8db38dafdf439cfe3f96673b6","signature":"0xd488cd2fda4ef325c68e3c7503a7075841f5ba08808fa2014407e18680fc3d4f515be9cdf6c619baa0e680990d7aad2f5f066cdba778598b28cc8dc3108f420c","type":"ed25519_signature"},"events":[{"guid":{"creation_number":"3","account_address":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682"},"sequence_number":"1","type":"0x1::coin::WithdrawEvent","data":{"amount":"123400000"}},{"guid":{"creation_number":"2","account_address":"0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365"},"sequence_number":"730","type":"0x1::coin::DepositEvent","data":{"amount":"123400000"}}],"timestamp":"1683055759739669","type":"user_transaction"}`,
				`{"block_height":"1309838","block_hash":"0x77eb1ba86353da0133d76892773ecbf18db68555ada5ab358d451ad23653cc31","block_timestamp":"1683055759739669","first_version":"3509308","last_version":"3509310","transactions":null}`,
				`{"chain_id":58,"epoch":"61","ledger_version":"3524912","oldest_ledger_version":"0","ledger_timestamp":"1683057861003497","node_role":"full_node","oldest_block_height":"0","block_height":"1317172","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
			},
			val: xclient.LegacyTxInfo{
				TxID:            "0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3222",
				BlockHash:       "3509309",
				From:            "0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682",
				To:              "0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365",
				ToAlt:           "",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(123400000),
				Fee:             xc.NewAmountBlockchainFromUint64(600),
				BlockIndex:      1309838,
				BlockTime:       1683055759,
				Confirmations:   7334,
				Sources: []*xclient.LegacyTxInfoEndpoint{{
					Address:         "0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682",
					Amount:          xc.NewAmountBlockchainFromUint64(123400000),
					ContractAddress: "APTOS",
					ContractId:      "0x1::aptos_coin::AptosCoin",
					NativeAsset:     "APTOS",
					Event:           xclient.NewEventFromIndex(0, xclient.MovementVariantNative),
				}},
				Destinations: []*xclient.LegacyTxInfoEndpoint{{
					Address:         "0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365",
					Amount:          xc.NewAmountBlockchainFromUint64(123400000),
					ContractAddress: "APTOS",
					NativeAsset:     "APTOS",
					ContractId:      "0x1::aptos_coin::AptosCoin",
					Event:           xclient.NewEventFromIndex(1, xclient.MovementVariantNative),
				}},
			},
		},
		{
			tx: "0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3221",
			resp: []string{
				`{"chain_id":58,"epoch":"61","ledger_version":"3532090","oldest_ledger_version":"0","ledger_timestamp":"1683058921700697","node_role":"full_node","oldest_block_height":"0","block_height":"1320608","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
				`{"message":"Transaction not found by Transaction hash(0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3221)","error_code":"transaction_not_found","vm_error_code":null}`,
			},
			val:             xclient.LegacyTxInfo{},
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
			val: xclient.LegacyTxInfo{
				TxID:            "0x06a6c25f3601895a3d3330f6ba4696fb8e677973aa56aa7b0ea362915bcff39c",
				BlockHash:       "176278674",
				From:            "0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f",
				To:              "0xcdaa56944a811c22398165b6c885b8aaad39fe7b91b008bb6334d639cbaec8f7",
				ToAlt:           "",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(140099000000),
				Fee:             xc.NewAmountBlockchainFromUint64(600),
				BlockIndex:      67467400,
				BlockTime:       1688879042,
				Confirmations:   546501,
				Sources: []*xclient.LegacyTxInfoEndpoint{{
					Address:         "0x80174e0fe8cb2d32b038c6c888dd95c3e1560736f0d4a6e8bed6ae43b5c91f6f",
					Amount:          xc.NewAmountBlockchainFromUint64(140099000000),
					NativeAsset:     "APTOS",
					ContractAddress: "APTOS",
					ContractId:      "0x1::aptos_coin::AptosCoin",
					Event:           xclient.NewEventFromIndex(0, xclient.MovementVariantNative),
				}},
				Destinations: []*xclient.LegacyTxInfoEndpoint{{
					Address:         "0xcdaa56944a811c22398165b6c885b8aaad39fe7b91b008bb6334d639cbaec8f7",
					Amount:          xc.NewAmountBlockchainFromUint64(140099000000),
					NativeAsset:     "APTOS",
					ContractAddress: "APTOS",
					ContractId:      "0x1::aptos_coin::AptosCoin",
					Event:           xclient.NewEventFromIndex(1, xclient.MovementVariantNative),
				}},
			},
			err: "",
		},
		{
			// 1.234 APTOS failed transaction
			tx: "0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3222",
			resp: []string{
				`{"chain_id":58,"epoch":"61","ledger_version":"3524910","oldest_ledger_version":"0","ledger_timestamp":"1683057860656414","node_role":"full_node","oldest_block_height":"0","block_height":"1317171","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
				`{"version":"3509309","hash":"0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3222","state_change_hash":"0xe0e855e3d08f97fc71a5b41b368800588ac7f8b2e49b29daef4d2577c761fe80","event_root_hash":"0x3846412f44cf58865775791b67093d555c854fbffe153965e325f8744c988a71","state_checkpoint_hash":null,"gas_used":"6","success":false,"vm_status":"Executed successfully","accumulator_root_hash":"0x30c4b395b9da13dfdeb74a341798f20d6c65872594f1e22f8fc734c9378c0747","changes":[],"sender":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682","sequence_number":"1","max_gas_amount":"2000","gas_unit_price":"100","expiration_timestamp_secs":"1683055757286067","payload":{"function":"0x1::aptos_account::transfer","type_arguments":[],"arguments":["0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365","123400000"],"type":"entry_function_payload"},"signature":{"public_key":"0xa09bb3957ad788bfcfd3f7c5eda9ab2876ff0de8db38dafdf439cfe3f96673b6","signature":"0xd488cd2fda4ef325c68e3c7503a7075841f5ba08808fa2014407e18680fc3d4f515be9cdf6c619baa0e680990d7aad2f5f066cdba778598b28cc8dc3108f420c","type":"ed25519_signature"},"events":[{"guid":{"creation_number":"3","account_address":"0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682"},"sequence_number":"1","type":"0x1::coin::WithdrawEvent","data":{"amount":"123400000"}},{"guid":{"creation_number":"2","account_address":"0x2a5ddd8e5ac5e30f61e42e4dc54a2d6a904412810767fa2e1674b08ca3b04365"},"sequence_number":"730","type":"0x1::coin::DepositEvent","data":{"amount":"123400000"}}],"timestamp":"1683055759739669","type":"user_transaction"}`,
				`{"block_height":"1309838","block_hash":"0x77eb1ba86353da0133d76892773ecbf18db68555ada5ab358d451ad23653cc31","block_timestamp":"1683055759739669","first_version":"3509308","last_version":"3509310","transactions":null}`,
				`{"chain_id":58,"epoch":"61","ledger_version":"3524912","oldest_ledger_version":"0","ledger_timestamp":"1683057861003497","node_role":"full_node","oldest_block_height":"0","block_height":"1317172","git_hash":"57f8b499aead5adf38276acb585cd2c0de398568"}`,
			},
			val: xclient.LegacyTxInfo{
				TxID:            "0x15940935f6317d7a42085855aa8167106aff03aeff5528bed51da015940d3222",
				BlockHash:       "3509309",
				From:            "0xf08819a2ca002c1da8c6242040607617093f519eb2525201efaba47b0841f682",
				To:              "",
				ToAlt:           "",
				ContractAddress: "",
				Fee:             xc.NewAmountBlockchainFromUint64(600),
				BlockIndex:      1309838,
				BlockTime:       1683055759,
				Confirmations:   7334,
				Sources:         []*xclient.LegacyTxInfoEndpoint{},
				Destinations:    []*xclient.LegacyTxInfoEndpoint{},
				Error:           "transaction failed",
			},
		},
	}

	for i, v := range vectors {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			resp := `{"chain_id":38,"epoch":"133","ledger_version":"13087045","oldest_ledger_version":"0","ledger_timestamp":"1669676013555573","node_role":"full_node","oldest_block_height":"0","block_height":"5435983","git_hash":"2c74a456298fcd520241a562119b6fe30abdaae2"}`
			server, close := testtypes.MockHTTP(t, resp, 0)

			asset := xc.NewChainConfig("APTOS").WithNet("devnet").WithChainCoin("0x1::aptos_coin::AptosCoin")
			asset.URL = server.URL
			client, _ := NewClient(asset)
			server.StatusCodes = v.httpStatusCodes
			server.Response = v.resp
			txInfo, err := client.FetchLegacyTxInfo(context.Background(), xc.TxHash(v.tx))

			if v.err != "" {
				require.Equal(xclient.LegacyTxInfo{}, txInfo)
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

func (s *AptosTestSuite) TestFetchBalance() {
	require := s.Require()

	vectors := []struct {
		asset    xc.ITask
		contract xc.ContractAddress
		resp     interface{}
		val      string
		err      string
	}{
		{
			asset: xc.NewChainConfig(""),
			resp:  `{"type":"0x1::coin::CoinStore<0x1::aptos_coin::AptosCoin>","data":{"coin":{"value":"1000000"},"deposit_events":{"counter":"2","guid":{"id":{"addr":"0xa589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab85","creation_num":"2"}}},"frozen":false,"withdraw_events":{"counter":"0","guid":{"id":{"addr":"0xa589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab85","creation_num":"3"}}}}}`,
			val:   "1000000",
			err:   "",
		},
		{
			// TODO I can't find any tokens on aptos
			asset:    xc.NewChainConfig(""),
			contract: "0x1234::coin:USDC",
			resp: []string{
				`{}`,
				`{"message":"failed to parse path : failed to parse \"string(MoveStructTag)\": invalid struct tag: 0x1::coin::CoinStore<0x1::coin:USDC>, unrecognized token","error_code":"web_framework_error","vm_error_code":null}`,
			},
			val: "1000000",
			err: "failed to parse",
		},
		{
			asset: xc.NewChainConfig(""),
			resp:  `null`,
			val:   "0",
			err:   "",
		},
	}

	for _, v := range vectors {
		resp := `{"chain_id":38,"epoch":"133","ledger_version":"13087045","oldest_ledger_version":"0","ledger_timestamp":"1669676013555573","node_role":"full_node","oldest_block_height":"0","block_height":"5435983","git_hash":"2c74a456298fcd520241a562119b6fe30abdaae2"}`
		server, close := testtypes.MockHTTP(s.T(), resp, 200)
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
		balance, err := rpcClient.FetchBalance(s.Ctx, args)

		if v.err != "" {
			require.Equal("0", balance.String())
			require.ErrorContains(err, v.err)
		} else {
			require.Nil(err)
			require.NotNil(balance)
			require.Equal(v.val, balance.String())
		}
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
		Pubkey:          pubkey,
	}
	args := buildertest.MustNewTransferArgs(from, to, amount)
	tf, err := builder.Transfer(args, input)
	require.NoError(err)
	require.NotNil(tf)
	hash := tf.Hash()
	require.Len(hash, 64)

	// add signature
	sig := []byte{}
	for i := 0; i < 64; i++ {
		sig = append(sig, byte(i))
	}
	err = tf.AddSignatures(xc.TxSignature(sig))
	require.NoError(err)

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
		Pubkey:          pubkey,
	}
	args := buildertest.MustNewTransferArgs(
		from, to, amount,
		buildertest.OptionContractAddress("0x1::Coin::USDC"),
	)
	tf, err := builder.Transfer(args, input)
	require.NoError(err)
	require.NotNil(tf)
	hash := tf.Hash()
	require.Len(hash, 64)

	// add signature
	sig := []byte{}
	for i := 0; i < 64; i++ {
		sig = append(sig, byte(i))
	}
	err = tf.AddSignatures(xc.TxSignature(sig))
	require.NoError(err)

	ser, err := tf.Serialize()
	require.NoError(err)
	require.True(len(ser) > 64)
	require.Equal("a589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab85030000000000000002000000000000000000000000000000000000000000000000000000000000000104636f696e087472616e736665720107000000000000000000000000000000000000000000000000000000000000000104436f696e0455534443000220bb89a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab00080100000000000000d0070000000000000a00000000000000493e000000000000010020010203040506070801020304050607080102030405060708010203040506070840000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f", hex.EncodeToString(ser))

	// use invalid contract address
	_, err = builder.NewTokenTransfer(from, to, amount, xc.ContractAddress("0x112345"), input)
	require.ErrorContains(err, "Invalid struct tag string literal")
}
