package client_test

import (
	"context"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/solana/client"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	testtypes "github.com/cordialsys/crosschain/testutil"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"
)

/*
# Get Vote accounts
curl https://api.mainnet.solana.com -X POST -H "Content-Type: application/json" -d '

	{
	  "jsonrpc": "2.0",
	  "id": 1,
	  "method": "getVoteAccounts",
	  "params": [
	    {
	  }
	  ]
	}

'
*/
func TestFetchStakingInput(t *testing.T) {

	vectors := []struct {
		description string
		resp        interface{}
		expected    *tx_input.StakingInput
		validator   string
		err         string
	}{
		{
			description: "get staking info",
			resp: []string{
				// valid blockhash
				`{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				// vote account list
				`{"jsonrpc":"2.0","result":{"current":[{"activatedStake":41061582618205,"commission":7,"epochCredits":[[645,196752727,196348579],[646,197152299,196752727],[647,197549513,197152299],[648,197955321,197549513],[649,198362807,197955321]],"epochVoteAccount":true,"lastVote":280799357,"nodePubkey":"CVAAQGA8GBzKi4kLdmpDuJnpkSik6PMWSvRk3RDds9K8","rootSlot":280799326,"votePubkey":"XBtfuT5gYU27UAukT3pEzgiKgHpHNQhSoa3zX2PYtiT"},{"activatedStake":32347208647108,"commission":7,"epochCredits":[[645,33158933,32754517],[646,33548343,33158933],[647,33944043,33548343],[648,34350430,33944043],[649,34758214,34350430]],"epochVoteAccount":true,"lastVote":280799357,"nodePubkey":"EqgfgrWR3D1As2aS7tYjoHfNxgxcfNYvdUL5zCsXFXBt","rootSlot":280799326,"votePubkey":"3m8Ct5n9feJFEuuXFb67oqt9XEJeBYkGyEdQRX33QQ5H"}]}}`,
			},
			validator: "3m8Ct5n9feJFEuuXFb67oqt9XEJeBYkGyEdQRX33QQ5H",
			expected: &tx_input.StakingInput{
				TxInput: tx_input.TxInput{
					TxInputEnvelope:   xc.TxInputEnvelope{Type: xc.DriverSolana},
					RecentBlockHash:   solana.MustHashFromBase58("DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK"),
					PrioritizationFee: xc.NewAmountBlockchainFromUint64(100000),
					BaseFee:           xc.NewAmountBlockchainFromUint64(5000),
				},
				ValidatorVoteAccount: solana.MustPublicKeyFromBase58("3m8Ct5n9feJFEuuXFb67oqt9XEJeBYkGyEdQRX33QQ5H"),
			},
		},
		{
			// we need the validator vote account, but we can identify it by the validator identity
			description: "get staking info by validator identity",
			resp: []string{
				// valid blockhash
				`{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				// vote account list
				`{"jsonrpc":"2.0","result":{"current":[{"activatedStake":41061582618205,"commission":7,"epochCredits":[[645,196752727,196348579],[646,197152299,196752727],[647,197549513,197152299],[648,197955321,197549513],[649,198362807,197955321]],"epochVoteAccount":true,"lastVote":280799357,"nodePubkey":"CVAAQGA8GBzKi4kLdmpDuJnpkSik6PMWSvRk3RDds9K8","rootSlot":280799326,"votePubkey":"XBtfuT5gYU27UAukT3pEzgiKgHpHNQhSoa3zX2PYtiT"},{"activatedStake":32347208647108,"commission":7,"epochCredits":[[645,33158933,32754517],[646,33548343,33158933],[647,33944043,33548343],[648,34350430,33944043],[649,34758214,34350430]],"epochVoteAccount":true,"lastVote":280799357,"nodePubkey":"EqgfgrWR3D1As2aS7tYjoHfNxgxcfNYvdUL5zCsXFXBt","rootSlot":280799326,"votePubkey":"3m8Ct5n9feJFEuuXFb67oqt9XEJeBYkGyEdQRX33QQ5H"}]}}`,
			},
			validator: "EqgfgrWR3D1As2aS7tYjoHfNxgxcfNYvdUL5zCsXFXBt",
			expected: &tx_input.StakingInput{
				TxInput: tx_input.TxInput{
					TxInputEnvelope:   xc.TxInputEnvelope{Type: xc.DriverSolana},
					RecentBlockHash:   solana.MustHashFromBase58("DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK"),
					PrioritizationFee: xc.NewAmountBlockchainFromUint64(100000),
					BaseFee:           xc.NewAmountBlockchainFromUint64(5000),
				},
				ValidatorVoteAccount: solana.MustPublicKeyFromBase58("3m8Ct5n9feJFEuuXFb67oqt9XEJeBYkGyEdQRX33QQ5H"),
			},
		},
		{
			// we need the validator vote account, but we can identify it by the validator identity
			description: "invalid validator",
			resp: []string{
				// valid blockhash
				`{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				// vote account list
				`{"jsonrpc":"2.0","result":{"current":[{"activatedStake":41061582618205,"commission":7,"epochCredits":[[645,196752727,196348579],[646,197152299,196752727],[647,197549513,197152299],[648,197955321,197549513],[649,198362807,197955321]],"epochVoteAccount":true,"lastVote":280799357,"nodePubkey":"CVAAQGA8GBzKi4kLdmpDuJnpkSik6PMWSvRk3RDds9K8","rootSlot":280799326,"votePubkey":"XBtfuT5gYU27UAukT3pEzgiKgHpHNQhSoa3zX2PYtiT"},{"activatedStake":32347208647108,"commission":7,"epochCredits":[[645,33158933,32754517],[646,33548343,33158933],[647,33944043,33548343],[648,34350430,33944043],[649,34758214,34350430]],"epochVoteAccount":true,"lastVote":280799357,"nodePubkey":"EqgfgrWR3D1As2aS7tYjoHfNxgxcfNYvdUL5zCsXFXBt","rootSlot":280799326,"votePubkey":"3m8Ct5n9feJFEuuXFb67oqt9XEJeBYkGyEdQRX33QQ5H"}]}}`,
			},
			validator: "o7hZ7ceQYKTXgwJdEkczQmZmcrZTszmiTw1K1sPEaYn",
			err:       "validator vote account not found",
		},
	}

	for i, v := range vectors {
		t.Run(fmt.Sprintf("%d - %s", i, v.description), func(t *testing.T) {
			server, close := testtypes.MockJSONRPC(t, v.resp)
			defer close()
			chainCfg := xc.NewChainConfig(xc.SOL).
				WithUrl(server.URL).
				WithDecimals(9)

			client, _ := client.NewClient(chainCfg)
			from := xc.Address("4ixwJt7DDGUV3xxi3mvZuEjLn4kDC39ogknnHQ4Crv5a")
			args := buildertest.MustNewStakingArgs(
				xc.SOL,
				from,
				builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(1000)),
				builder.OptionValidator(v.validator),
			)

			input, err := client.FetchStakingInput(context.Background(), args)

			if v.err != "" {
				require.Nil(t, input)
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)

				// this is randomly generately, so we omit from test
				require.Len(t, input.(*tx_input.StakingInput).StakingKey, 64)
				input.(*tx_input.StakingInput).StakingKey = nil

				require.Equal(t, v.expected, input)
			}
		})
	}
}

/*
# Get Stake accounts
curl https://api.mainnet.solana.com -X POST -H "Content-Type: application/json" -d '
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "getProgramAccounts",
  "params": [
    "Stake11111111111111111111111111111111111111",
    {
      "encoding": "jsonParsed",
      "filters": [
        {
          "memcmp": {
            "offset": 12,
            "bytes": "83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"
          }
        }
      ]
    }
  ]
}
'
*/

func TestFetchUnstakingInput(t *testing.T) {

	vectors := []struct {
		description string
		resp        interface{}
		expected    *tx_input.UnstakingInput
		validator   string
		err         string
	}{
		{
			description: "get unstaking info",
			resp: []string{
				// stake accounts
				`[{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH","withdrawer":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":101316504,"delegation":{"activationEpoch":"650","deactivationEpoch":"650","stake":"7717120","voter":"J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp","warmupCooldownRate":0.25}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":10000000,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":18446744073709552000,"space":200},"pubkey":"GuXr1c5KyuJxpsoKMDiDBAJZq4GczPMNUmp4UKY9LbAE"},{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH","withdrawer":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":101731298,"delegation":{"activationEpoch":"650","deactivationEpoch":"18446744073709551615","stake":"37731751","voter":"J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp","warmupCooldownRate":0.25}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":40016458,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":18446744073709552000,"space":200},"pubkey":"6LFjBX1yUwSr8SWsyZUc5okZiVo8ZdmVQ9keJAazRmnh"},{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH","withdrawer":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":101316504,"delegation":{"activationEpoch":"649","deactivationEpoch":"650","stake":"717400","voter":"J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp","warmupCooldownRate":0.25}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":3000322,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":18446744073709552000,"space":200},"pubkey":"8zrSGLMdE6dK57Q7a8N8TDohmyft1MrsLYdRqhDvCerc"},{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH","withdrawer":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":102020250,"delegation":{"activationEpoch":"652","deactivationEpoch":"18446744073709551615","stake":"7717120","voter":"J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp","warmupCooldownRate":0.25}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":10000000,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":18446744073709552000,"space":200},"pubkey":"BYoo5izmpyrkc4fKkJy2gp6Bwc9evt4vgCYYMY3NHu9C"},{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH","withdrawer":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":101731298,"delegation":{"activationEpoch":"650","deactivationEpoch":"18446744073709551615","stake":"1717786","voter":"J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp","warmupCooldownRate":0.25}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":4000749,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":18446744073709552000,"space":200},"pubkey":"CCTFhyxoUHGmdQvuUxFquyYMK4H5hdqwCCN7XAXtK9HC"}]`,
				// valid blockhash
				`{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				// epoch info
				`{"absoluteSlot": 166598,"blockHeight": 166500,"epoch": 650,"slotIndex": 2790,"slotsInEpoch": 8192,"transactionCount": 22661093}`,
			},
			validator: "J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp",
			expected: &tx_input.UnstakingInput{
				TxInput: tx_input.TxInput{
					TxInputEnvelope:   xc.TxInputEnvelope{Type: xc.DriverSolana},
					RecentBlockHash:   solana.MustHashFromBase58("DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK"),
					PrioritizationFee: xc.NewAmountBlockchainFromUint64(100000),
					BaseFee:           xc.NewAmountBlockchainFromUint64(5000),
				},
				EligibleStakes: []*tx_input.ExistingStake{
					{
						ActivationEpoch:   xc.NewAmountBlockchainFromUint64(650),
						DeactivationEpoch: xc.NewAmountBlockchainFromUint64(18446744073709551615),
						AmountActive:      xc.NewAmountBlockchainFromUint64(37731751),
						AmountInactive:    xc.NewAmountBlockchainFromUint64(2282880),
					},
					{
						ActivationEpoch:   xc.NewAmountBlockchainFromUint64(650),
						DeactivationEpoch: xc.NewAmountBlockchainFromUint64(18446744073709551615),
						AmountActive:      xc.NewAmountBlockchainFromUint64(1717786),
						AmountInactive:    xc.NewAmountBlockchainFromUint64(2282880),
					},
					{
						ActivationEpoch:   xc.NewAmountBlockchainFromUint64(652),
						DeactivationEpoch: xc.NewAmountBlockchainFromUint64(18446744073709551615),
						AmountActive:      xc.NewAmountBlockchainFromUint64(7717120),
						AmountInactive:    xc.NewAmountBlockchainFromUint64(2282880),
					},
				},
			},
		},
	}

	for i, v := range vectors {
		t.Run(fmt.Sprintf("%d - %s", i, v.description), func(t *testing.T) {
			server, close := testtypes.MockJSONRPC(t, v.resp)
			defer close()
			chainCfg := xc.NewChainConfig("SOL").WithUrl(server.URL).WithDecimals(9)

			client, _ := client.NewClient(chainCfg)
			from := xc.Address("4ixwJt7DDGUV3xxi3mvZuEjLn4kDC39ogknnHQ4Crv5a")
			args := buildertest.MustNewStakingArgs(
				xc.SOL,
				from,
				builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(100000000)),
				builder.OptionValidator(v.validator),
			)

			input, err := client.FetchUnstakingInput(context.Background(), args)

			if v.err != "" {
				require.Nil(t, input)
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)

				// this is randomly generately, so we omit from test
				require.Len(t, input.(*tx_input.UnstakingInput).StakingKey, 64)
				input.(*tx_input.UnstakingInput).StakingKey = nil

				// do not test all fields
				for _, stake := range input.(*tx_input.UnstakingInput).EligibleStakes {
					require.Len(t, stake.StakeAccount, 32)
					// require.Len(t, stake.ValidatorVoteAccount, 32)
					stake.StakeAccount = solana.PublicKey{}
					// stake.ValidatorVoteAccount = solana.PublicKey{}
				}

				require.Equal(t, v.expected, input)
			}
		})
	}
}

func TestFetchWithdrawInput(t *testing.T) {

	vectors := []struct {
		description string
		resp        interface{}
		expected    *tx_input.WithdrawInput
		validator   string
		err         string
	}{
		{
			description: "get withdraw info",
			resp: []string{
				// stake accounts
				`[{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH","withdrawer":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":102020250,"delegation":{"activationEpoch":"652","deactivationEpoch":"18446744073709551615","stake":"7717120","voter":"J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp","warmupCooldownRate":0.25}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":10000000,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":18446744073709552000,"space":200},"pubkey":"BYoo5izmpyrkc4fKkJy2gp6Bwc9evt4vgCYYMY3NHu9C"},{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH","withdrawer":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":101731298,"delegation":{"activationEpoch":"650","deactivationEpoch":"18446744073709551615","stake":"27727872","voter":"J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp","warmupCooldownRate":0.25}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":30012094,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":18446744073709552000,"space":200},"pubkey":"GSQJ1PmGtY11efVjmEuUyim4PqXKsB7tnPp1jvpoFeRz"},{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH","withdrawer":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":101316504,"delegation":{"activationEpoch":"650","deactivationEpoch":"650","stake":"7717120","voter":"J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp","warmupCooldownRate":0.25}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":10000000,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":18446744073709552000,"space":200},"pubkey":"GuXr1c5KyuJxpsoKMDiDBAJZq4GczPMNUmp4UKY9LbAE"},{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH","withdrawer":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":101731298,"delegation":{"activationEpoch":"650","deactivationEpoch":"18446744073709551615","stake":"17723993","voter":"J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp","warmupCooldownRate":0.25}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":20007731,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":18446744073709552000,"space":200},"pubkey":"FXw5CT4CeyZoBd5Nzqad2CoPUxSwJhx23dDkhxq4sDHs"},{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH","withdrawer":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":101731298,"delegation":{"activationEpoch":"650","deactivationEpoch":"18446744073709551615","stake":"37731751","voter":"J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp","warmupCooldownRate":0.25}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":40016458,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":18446744073709552000,"space":200},"pubkey":"6LFjBX1yUwSr8SWsyZUc5okZiVo8ZdmVQ9keJAazRmnh"},{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH","withdrawer":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":101316504,"delegation":{"activationEpoch":"649","deactivationEpoch":"650","stake":"717400","voter":"J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp","warmupCooldownRate":0.25}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":3000322,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":18446744073709552000,"space":200},"pubkey":"8zrSGLMdE6dK57Q7a8N8TDohmyft1MrsLYdRqhDvCerc"},{"account":{"data":{"parsed":{"info":{"meta":{"authorized":{"staker":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH","withdrawer":"83wDqn8DFg5oh1WetQJwcyZySjxGkxWVKf3p39T6GMQH"},"lockup":{"custodian":"11111111111111111111111111111111","epoch":0,"unixTimestamp":0},"rentExemptReserve":"2282880"},"stake":{"creditsObserved":101731298,"delegation":{"activationEpoch":"650","deactivationEpoch":"18446744073709551615","stake":"1717786","voter":"J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp","warmupCooldownRate":0.25}}},"type":"delegated"},"program":"stake","space":200},"executable":false,"lamports":4000749,"owner":"Stake11111111111111111111111111111111111111","rentEpoch":18446744073709552000,"space":200},"pubkey":"CCTFhyxoUHGmdQvuUxFquyYMK4H5hdqwCCN7XAXtK9HC"}]`,
				// valid blockhash
				`{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				// epoch info
				`{"absoluteSlot": 166598,"blockHeight": 166500,"epoch": 652,"slotIndex": 2790,"slotsInEpoch": 8192,"transactionCount": 22661093}`,
			},
			validator: "J2nUHEAgZFRyuJbFjdqPrAa9gyWDuc7hErtDQHPhsYRp",
			expected: &tx_input.WithdrawInput{
				TxInput: tx_input.TxInput{
					TxInputEnvelope:   xc.TxInputEnvelope{Type: xc.DriverSolana},
					RecentBlockHash:   solana.MustHashFromBase58("DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK"),
					PrioritizationFee: xc.NewAmountBlockchainFromUint64(100000),
					BaseFee:           xc.NewAmountBlockchainFromUint64(5000),
				},
				EligibleStakes: []*tx_input.ExistingStake{
					{
						ActivationEpoch:   xc.NewAmountBlockchainFromUint64(649),
						DeactivationEpoch: xc.NewAmountBlockchainFromUint64(650),
						AmountActive:      xc.NewAmountBlockchainFromUint64(0),
						AmountInactive:    xc.NewAmountBlockchainFromUint64(3000280),
					},
					{
						ActivationEpoch:   xc.NewAmountBlockchainFromUint64(650),
						DeactivationEpoch: xc.NewAmountBlockchainFromUint64(650),
						AmountActive:      xc.NewAmountBlockchainFromUint64(0),
						AmountInactive:    xc.NewAmountBlockchainFromUint64(10000000),
					},
				},
			},
		},
	}

	for i, v := range vectors {
		t.Run(fmt.Sprintf("%d - %s", i, v.description), func(t *testing.T) {
			server, close := testtypes.MockJSONRPC(t, v.resp)
			defer close()
			chainCfg := xc.NewChainConfig("SOL").WithUrl(server.URL).WithDecimals(9)

			client, _ := client.NewClient(chainCfg)
			from := xc.Address("4ixwJt7DDGUV3xxi3mvZuEjLn4kDC39ogknnHQ4Crv5a")
			options := []builder.BuilderOption{}
			if v.validator != "" {
				options = append(options, builder.OptionValidator(v.validator))
			}

			options = append(options, builder.OptionStakeAmount(xc.NewAmountBlockchainFromUint64(100000000)))
			args := buildertest.MustNewStakingArgs(
				xc.SOL,
				from,
				options...,
			)

			input, err := client.FetchWithdrawInput(context.Background(), args)

			if v.err != "" {
				require.Nil(t, input)
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)

				// do not test all fields
				for _, stake := range input.(*tx_input.WithdrawInput).EligibleStakes {
					require.Len(t, stake.StakeAccount, 32)
					// require.Len(t, stake.ValidatorVoteAccount, 32)
					stake.StakeAccount = solana.PublicKey{}
					// stake.ValidatorVoteAccount = solana.PublicKey{}
				}

				require.Equal(t, v.expected, input)
			}
		})
	}
}
