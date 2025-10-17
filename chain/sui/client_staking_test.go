package sui_test

import (
	"context"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/sui"
	xclient "github.com/cordialsys/crosschain/client"
	testtypes "github.com/cordialsys/crosschain/testutil"
	"github.com/stretchr/testify/require"
)

func TestFetchStakeBalance(t *testing.T) {
	vectors := []struct {
		name              string
		getStakesResponse string
		expectedBalances  []*xclient.StakedBalance
		err               string
	}{
		{
			name: "ActivatingStake",
			getStakesResponse: `{
			  "jsonrpc": "2.0",
			  "id": 1,
			  "result": [
				{
				  "validatorAddress": "0x9c1d75a4665a5461750992d08f36c883cc0dd87a103ea15536b4e6ea8830b8f9",
				  "stakingPool": "0xa586acdf2fc46ad3edf056dae212da21dcedc51b8f48dc02f0b40f52d083aae8",
				  "stakes": [
					{
					  "stakedSuiId": "0x426a3d561914ca66d8ba5cd344ed6e9c4a802fea2bb1603f161a89d68d031c91",
					  "stakeRequestEpoch": "45",
					  "stakeActiveEpoch": "46",
					  "principal": "2000000000",
					  "status": "Pending"
					}
				  ]
				}
			  ]
			}`,
			expectedBalances: []*xclient.StakedBalance{
				{
					Validator: "0x9c1d75a4665a5461750992d08f36c883cc0dd87a103ea15536b4e6ea8830b8f9",
					Account:   "0x426a3d561914ca66d8ba5cd344ed6e9c4a802fea2bb1603f161a89d68d031c91",
					Balance: xclient.StakedBalanceState{
						Activating: xc.NewAmountBlockchainFromUint64(2000000000),
					},
				},
			},
		},
		{
			name: "ActiveStake",
			getStakesResponse: `{
			  "jsonrpc": "2.0",
			  "id": 1,
			  "result": [
				{
				  "validatorAddress": "0x9c1d75a4665a5461750992d08f36c883cc0dd87a103ea15536b4e6ea8830b8f9",
				  "stakingPool": "0xa586acdf2fc46ad3edf056dae212da21dcedc51b8f48dc02f0b40f52d083aae8",
				  "stakes": [
					{
					  "stakedSuiId": "0x0b1b9d2dcfdc9cdea07375b756e2ea8032f4e6af58e1bfda8347399c17c7a8b4",
					  "stakeRequestEpoch": "15",
					  "stakeActiveEpoch": "16",
					  "principal": "1000000000",
					  "status": "Active",
					  "estimatedReward": "233452979"
					}
				  ]
				}
			  ]
			}`,
			expectedBalances: []*xclient.StakedBalance{
				{
					Validator: "0x9c1d75a4665a5461750992d08f36c883cc0dd87a103ea15536b4e6ea8830b8f9",
					Account:   "0x0b1b9d2dcfdc9cdea07375b756e2ea8032f4e6af58e1bfda8347399c17c7a8b4",
					Balance: xclient.StakedBalanceState{
						Active: xc.NewAmountBlockchainFromUint64(1000000000 + 233452979), // principal + rewards
					},
				},
			},
		},
	}

	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			server, close := testtypes.MockJSONRPC(t, v.getStakesResponse)
			defer close()

			asset := xc.NewChainConfig(xc.SUI).WithNet("devnet").WithUrl(server.URL)
			asset.URL = server.URL
			client, err := sui.NewClient(asset)
			require.NoError(t, err)

			stakeArgs, err := xclient.NewStakeBalanceArgs(xc.Address(""))
			require.NoError(t, err)
			balances, err := client.FetchStakeBalance(context.Background(), stakeArgs)
			require.NoError(t, err)

			if v.err != "" {
				require.ErrorContains(t, err, v.err)
			} else {
				require.Equal(t, v.expectedBalances, balances)
			}
		})
	}
}

func TestFetchUnstakingInput(t *testing.T) {
	// responses for base tx input + few stake objects:
	// 1. Balance: 1240330667, principal: 100000000, validator: 0x7e99c532dc9be514cf98650e6e80dd5a8d859eb298cc63f1bd18222694cbda1d
	// 2. Balance: 1220330667, principal: 100000000, validator: 0x7e99c532dc9be514cf98650e6e80dd5a8d859eb298cc63f1bd18222694cbda1d
	// 3. Balance: 200000000, principal: 200000000, validator: 0x9c1d75a4665a5461750992d08f36c883cc0dd87a103ea15536b4e6ea8830b8f9
	// 3. Balance: 400000000, principal: 200000000, validator: 0x9c1d75a4665a5461750992d08f36c883cc0dd87a103ea15536b4e6ea8830b8f9
	txInputResponses := []string{
		// get coins
		`{"data":[{"coinType":"0x2::sui::SUI","coinObjectId":"0xc587db1fbe680b769c1a562a09f2c871a087bafa542c7cb73db6064e2b791bdf","version":"1852491","digest":"GBm2HRW1WvNRrGX5iM3syjbD1PeaWQs69s42wJEam7HY","balance":"5845686480","previousTransaction":"4qkLLVGsxNwvvpJMwSbCh4jFmC9J8Cb1x1zhNaC7k5cK"}],"nextCursor":"0xc587db1fbe680b769c1a562a09f2c871a087bafa542c7cb73db6064e2b791bdf","hasNextPage":false}`,
		// get checkpoint
		`{"data":[{"epoch":"21","sequenceNumber":"2206686","digest":"HtsAAgd1ajMR8qMocnNF6XbAtiBHrxdauGhWtXqKouF3","networkTotalTransactions":"5164703","previousDigest":"H8oYvb73KoG7TWXpw4JPy2qZk7ddvHY3rYQ8kHcNmcua","epochRollingGasCostSummary":{"computationCost":"130960164300","storageCost":"499151462400","storageRebate":"422717709348","nonRefundableStorageFee":"4269875852"},"timestampMs":"1683320609521","transactions":["3yVjcHqKwLN8K8TrZZZMpMUp4VSGg4LRp4uuzvvzzrFD","Cv2NH6zJiRJMtPMzxzZABgDpBfNmb9eniWW9t5v2kPtz","GJaDtfzHap6V8ARdQTstkJm7PiWsEXWkUapXHA2nbmbD"],"checkpointCommitments":[],"validatorSignature":"i3aT5RVtIOvX0pEc/HU+xFTHbw2zV5SdT7q5n6GfS+e85CtkC8qqseeK2Hx9Nhia"}],"nextCursor":"2206686","hasNextPage":true}`,
		//reference gas
		"1000",
		// getStakes
		`{
		  "jsonrpc": "2.0",
		  "id": 1,
		  "result": [
			{
			  "validatorAddress": "0x7e99c532dc9be514cf98650e6e80dd5a8d859eb298cc63f1bd18222694cbda1d",
			  "stakingPool": "0x3a307858c118155331c1aa48a90c3d871ca03263f9cd85c2e8d4a75d93003b1c",
			  "stakes": [
				{
				  "stakedSuiId": "0x0b1b9d2dcfdc9cdea07375b756e2ea8032f4e6af58e1bfda8347399c17c7a8b4",
				  "stakeRequestEpoch": "15",
				  "stakeActiveEpoch": "16",
				  "principal": "1000000000",
				  "status": "Active",
				  "estimatedReward": "240330667"
				},
				{
				  "stakedSuiId": "0xacfa7891f8e419630cf010805e09f56454bb88e720e1d44e3b009ea424f1118a",
				  "stakeRequestEpoch": "15",
				  "stakeActiveEpoch": "16",
				  "principal": "1000000000",
				  "status": "Active",
				  "estimatedReward": "220330667"
				}
			  ]
			},
			{
			  "validatorAddress": "0x9c1d75a4665a5461750992d08f36c883cc0dd87a103ea15536b4e6ea8830b8f9",
			  "stakingPool": "0xa586acdf2fc46ad3edf056dae212da21dcedc51b8f48dc02f0b40f52d083aae8",
			  "stakes": [
				{
				  "stakedSuiId": "0x426a3d561914ca66d8ba5cd344ed6e9c4a802fea2bb1603f161a89d68d031c91",
				  "stakeRequestEpoch": "45",
				  "stakeActiveEpoch": "46",
				  "principal": "2000000000",
				  "status": "Active",
				  "estimatedReward": "0"
				},
				{
				  "stakedSuiId": "0xa78a9794ac51e7cfd6cd2c223d395baf8a2918eeda7643bfc5d016e65a75ae56",
				  "stakeRequestEpoch": "46",
				  "stakeActiveEpoch": "47",
				  "principal": "4000000000",
				  "status": "Pending"
				}
			  ]
			}
		  ]
		}`,
		// get object
		`{
		  "jsonrpc": "2.0",
		  "id": 1,
		  "result": {
			"data": {
			  "objectId": "0x0b1b9d2dcfdc9cdea07375b756e2ea8032f4e6af58e1bfda8347399c17c7a8b4",
			  "version": "88",
			  "digest": "8us3Bxf6dcAy83NfQ6rE5urJDqbonAvErBDXfXMmFynN"
			}
		  }
		}`,
		// get object
		`{
		  "jsonrpc": "2.0",
		  "id": 1,
		  "result": {
			"data": {
			  "objectId": "0xacfa7891f8e419630cf010805e09f56454bb88e720e1d44e3b009ea424f1118a",
			  "version": "87",
			  "digest": "Pywnt9e378vq3P3RkZkZ6D7SxcL99unbzxnWrukmV3A"
			}
		  }
		}`,
		// get object
		`{
		  "jsonrpc": "2.0",
		  "id": 1,
		  "result": {
			"data": {
			  "objectId": "0x426a3d561914ca66d8ba5cd344ed6e9c4a802fea2bb1603f161a89d68d031c91",
			  "version": "246",
			  "digest": "HQfBDAPc6yh17Bf26HEuhoUizWymo94PkEAwVJbpAhbK"
			}
		  }
		}`,
		// get object
		`{
		  "jsonrpc": "2.0",
		  "id": 1,
		  "result": {
			"data": {
			  "objectId": "0xa78a9794ac51e7cfd6cd2c223d395baf8a2918eeda7643bfc5d016e65a75ae56",
			  "version": "251",
			  "digest": "G9ymqNBws9JXSTVMKtXFDoQzqaWpyxUByRNdwJvbPVdd"
			}
		  }
		}`,
		// gas sim
		DryRunResponse(4000000, 3000000, 2000000),
	}

	vectors := []struct {
		name                  string
		amount                xc.AmountBlockchain
		expectedStakesToClose []sui.Stake
		expectedStakeToSplit  sui.Stake
		expectedRemainder     xc.AmountBlockchain
		err                   string
	}{
		{
			name:   "FullStake",
			amount: xc.NewAmountBlockchainFromUint64(1_240_330_667),
			// expect to close single stake with exact amount
			expectedStakesToClose: []sui.Stake{
				{
					Principal: xc.NewAmountBlockchainFromUint64(1_000_000_000),
					Rewards:   xc.NewAmountBlockchainFromUint64(240_330_667),
					ObjectId:  "0x0b1b9d2dcfdc9cdea07375b756e2ea8032f4e6af58e1bfda8347399c17c7a8b4",
					Version:   88,
					Digest:    "8us3Bxf6dcAy83NfQ6rE5urJDqbonAvErBDXfXMmFynN",
					State:     xclient.Active,
					Validator: "0x7e99c532dc9be514cf98650e6e80dd5a8d859eb298cc63f1bd18222694cbda1d",
				},
			},
			// both remainder and stake to split should be emtpy
			expectedStakeToSplit: sui.Stake{},
			expectedRemainder:    xc.NewAmountBlockchainFromUint64(0),
		},
		{
			name:   "CloseSmallStakeSplit3Balance",
			amount: xc.NewAmountBlockchainFromUint64(3_000_000_000),
			// smallest stake can be closed
			expectedStakesToClose: []sui.Stake{
				{
					Principal: xc.NewAmountBlockchainFromUint64(1_000_000_000),
					Rewards:   xc.NewAmountBlockchainFromUint64(220_330_667),
					ObjectId:  "0xacfa7891f8e419630cf010805e09f56454bb88e720e1d44e3b009ea424f1118a",
					Version:   88,
					Digest:    "8us3Bxf6dcAy83NfQ6rE5urJDqbonAvErBDXfXMmFynN",
					State:     xclient.Active,
					Validator: "0x7e99c532dc9be514cf98650e6e80dd5a8d859eb298cc63f1bd18222694cbda1d",
				},
			},
			// expect split of stake with 4.0 balance

			expectedStakeToSplit: sui.Stake{
				Principal: xc.NewAmountBlockchainFromUint64(4_000_000_000),
				Rewards:   xc.NewAmountBlockchainFromUint64(0),
				ObjectId:  "0xa78a9794ac51e7cfd6cd2c223d395baf8a2918eeda7643bfc5d016e65a75ae56",
				Version:   251,
				Digest:    "G9ymqNBws9JXSTVMKtXFDoQzqaWpyxUByRNdwJvbPVdd",
				State:     xclient.Activating,
				Validator: "0x9c1d75a4665a5461750992d08f36c883cc0dd87a103ea15536b4e6ea8830b8f9",
			},
			// remainder is 3.0 - we are not closing any other stakes
			expectedRemainder: xc.NewAmountBlockchainFromUint64(2_220_330_667),
		},
		{
			name:   "SingleSplit",
			amount: xc.NewAmountBlockchainFromUint64(1_540_330_667),
			// cannot close any stake - no balance for split
			expectedStakesToClose: []sui.Stake{},
			// expect to split 4.0 stake
			expectedStakeToSplit: sui.Stake{
				Principal: xc.NewAmountBlockchainFromUint64(4000000000),
				Rewards:   xc.NewAmountBlockchainFromUint64(0),
				ObjectId:  "0xa78a9794ac51e7cfd6cd2c223d395baf8a2918eeda7643bfc5d016e65a75ae56",
				Version:   251,
				Digest:    "G9ymqNBws9JXSTVMKtXFDoQzqaWpyxUByRNdwJvbPVdd",
				State:     xclient.Activating,
				Validator: "0x9c1d75a4665a5461750992d08f36c883cc0dd87a103ea15536b4e6ea8830b8f9",
			},
			expectedRemainder: xc.NewAmountBlockchainFromUint64(2_459_669_333), // principal - amount
		},
	}

	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			server, close := testtypes.MockJSONRPC(t, txInputResponses)
			defer close()

			asset := xc.NewChainConfig(xc.SUI).WithNet("devnet").WithUrl(server.URL).WithDecimals(9)
			asset.URL = server.URL
			client, err := sui.NewClient(asset)
			require.NoError(t, err)

			stakeArgs, err := builder.NewStakeArgs(asset.Chain, "", builder.OptionStakeAmount(v.amount), builder.OptionPublicKey(make([]byte, 0)))

			require.NoError(t, err)

			unstakeInput, err := client.FetchUnstakingInput(context.Background(), stakeArgs)
			if v.err != "" {
				require.ErrorContains(t, err, v.err)
			} else {
				ui := unstakeInput.(*sui.UnstakingInput)
				require.Equal(t, v.expectedStakesToClose, ui.StakesToUnstake)
				require.Equal(t, v.expectedStakeToSplit, ui.StakeToSplit)
				require.Equal(t, v.expectedRemainder, ui.SplitAmount)
			}
		})
	}
}
