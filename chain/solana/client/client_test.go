package client_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	xcsolana "github.com/cordialsys/crosschain/chain/solana"
	"github.com/cordialsys/crosschain/chain/solana/client"
	"github.com/cordialsys/crosschain/chain/solana/tx"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/chain/solana/types"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	testtypes "github.com/cordialsys/crosschain/testutil/types"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestNewClient(t *testing.T) {
	client, err := client.NewClient(xc.NewChainConfig(""))
	require.NotNil(t, client)
	require.NoError(t, err)
}

func TestFindAssociatedTokenAddress(t *testing.T) {

	ata, err := types.FindAssociatedTokenAddress("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb", "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU", solana.TokenProgramID)
	require.NoError(t, err)
	require.Equal(t, "DvSgNMRxVSMBpLp4hZeBrmQo8ZRFne72actTZ3PYE3AA", ata)

	// backwards compat with no token owner being used
	ata, err = types.FindAssociatedTokenAddress("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb", "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU", solana.PublicKey{})
	require.NoError(t, err)
	require.Equal(t, "DvSgNMRxVSMBpLp4hZeBrmQo8ZRFne72actTZ3PYE3AA", ata)

	ata, err = types.FindAssociatedTokenAddress("CMNyyCXkAQ5cfFS2zQEg6YPzd8fpHvMFbbbmUfjoPp1s", "BRfq2tdBXycyPA9PWeGFPA61327VNQMkZBE7rTAcvYDr", solana.Token2022ProgramID)
	require.NoError(t, err)
	require.Equal(t, "5LJSMaVdHFzaDG6wPtRSL1RULtKWgrRubXcbeARsLLru", ata)

	ata, err = types.FindAssociatedTokenAddress("", "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU", solana.TokenProgramID)
	require.ErrorContains(t, err, "zero length string")
	require.Equal(t, "", ata)

	ata, err = types.FindAssociatedTokenAddress("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb", "xxx", solana.TokenProgramID)
	require.ErrorContains(t, err, "invalid length")
	require.Equal(t, "", ata)
}

func TestErrors(t *testing.T) {
	require.Equal(t, errors.TransactionTimedOut, xcsolana.CheckError(fmt.Errorf("Transaction simulation failed: Blockhash not found")))
}

/*
curl https://api.devnet.solana.com -X POST -H "Content-Type: application/json" -d '

	{
	  "jsonrpc": "2.0",
	  "id": 1,
	  "method": "getAccountInfo",
	  "params": [
	    "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb", {"encoding": "base64"}
	  ]
	}

'
*/
func TestFetchTxInput(t *testing.T) {

	vectors := []struct {
		asset             *xc.ChainConfig
		contract          xc.ContractAddress
		resp              interface{}
		blockHash         string
		toIsATA           bool
		shouldCreateATA   bool
		tokenAccountCount int
		err               string
		forceError        int
	}{
		{
			asset: xc.NewChainConfig(""),
			// valid blockhash
			resp: []string{
				// valid blockhash
				`{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				// simulation
				`{"jsonrpc":"2.0","result":{"value": {"unitsConsumed": 150,"logs": [],"accounts": null},"context": {"slot": 328286226}},"id":1}`,
			},
			blockHash:       "DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK",
			toIsATA:         false,
			shouldCreateATA: false,
			err:             "",
		},
		{
			asset:    xc.NewChainConfig(""),
			contract: xc.ContractAddress("4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"),
			resp: []string{
				// valid blockhash
				// `{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"2.0.5","slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","lastValidBlockHeight":308641695}},"id":"6acad392-db4b-4728-9385-2b2f7dd105b1"}`,
				// get-account-info for token account
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"1.18.16","slot":274176079},"value":{"data":["","base58"],"executable":false,"lamports":55028723345,"owner":"11111111111111111111111111111111","rentEpoch":18446744073709551615,"space":0}},"id":1}`,
				// valid owner account
				`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid param: could not find account"},"id":1}`,
				// valid ATA
				`{"context":{"apiVersion":"1.13.3","slot":175635873},"value":{"data":["O0Qss5EhV/E6kz0BNCgtAytf/s0Botvxt3kGCN8ALqctdvBIL2OuEzV5LBqS2x3308rEBwESq+xcukVQUYDkgpg6AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","base64"],"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":0}}`,
				// token account
				`{"context":{"apiVersion":"1.14.17","slot":205924180},"value":[{"account":{"data":{"parsed":{"info":{"isNative":false,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"5VCwKtCXgCJ6kit5FybXjvriW3xELsFDhYrPSqtJNmcD","state":"initialized","tokenAmount":{"amount":"55010000","decimals":6,"uiAmount":55.01,"uiAmountString":"55.01"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":361},"pubkey":"Hrb916EihPAN4T6xad9aVbrd5PfYmiJpvwLKA9XmgcGV"}]}`,
				// priority fee
				`{"jsonrpc":"2.0","result":[{"prioritizationFee": 50,"slot": 252519673},{"prioritizationFee": 100,"slot": 252519674}],"id":1}`,
				// simulation
				`{"jsonrpc":"2.0","result":{"value": {"unitsConsumed": 30000,"logs": [],"accounts": null},"context": {"slot": 328286226}},"id":1}`,
			},
			blockHash:       "DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK",
			toIsATA:         false,
			shouldCreateATA: false,
			err:             "",
		},
		{
			asset:    xc.NewChainConfig(""),
			contract: xc.ContractAddress("4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"),
			resp: []string{
				// valid blockhash
				// `{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"2.0.5","slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","lastValidBlockHeight":308641695}},"id":"6acad392-db4b-4728-9385-2b2f7dd105b1"}`,
				// get-account-info for token account
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"1.18.16","slot":274176079},"value":{"data":["","base58"],"executable":false,"lamports":55028723345,"owner":"11111111111111111111111111111111","rentEpoch":18446744073709551615,"space":0}},"id":1}`,
				// valid owner account
				`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid param: could not find account"},"id":1}`,
				// empty ATA
				`{"context":{"apiVersion":"1.13.3","slot":175636079},"value":null}`,
				// token account
				`{"context":{"apiVersion":"1.14.17","slot":205924180},"value":[{"account":{"data":{"parsed":{"info":{"isNative":false,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"5VCwKtCXgCJ6kit5FybXjvriW3xELsFDhYrPSqtJNmcD","state":"initialized","tokenAmount":{"amount":"55010000","decimals":6,"uiAmount":55.01,"uiAmountString":"55.01"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":361},"pubkey":"Hrb916EihPAN4T6xad9aVbrd5PfYmiJpvwLKA9XmgcGV"}]}`,
				// priority fee
				`{"jsonrpc":"2.0","result":[{"prioritizationFee": 50,"slot": 252519673},{"prioritizationFee": 100,"slot": 252519674}],"id":1}`,
				// simulation
				`{"jsonrpc":"2.0","result":{"value": {"unitsConsumed": 30000,"logs": [],"accounts": null},"context": {"slot": 328286226}},"id":1}`,
			},
			blockHash:       "DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK",
			toIsATA:         false,
			shouldCreateATA: true,
			err:             "",
		},
		{
			asset:    xc.NewChainConfig(""),
			contract: xc.ContractAddress("4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"),
			resp: []string{
				// valid blockhash
				// `{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"2.0.5","slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","lastValidBlockHeight":308641695}},"id":"6acad392-db4b-4728-9385-2b2f7dd105b1"}`,
				// get-account-info for token account
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"1.18.16","slot":274176079},"value":{"data":["","base58"],"executable":false,"lamports":55028723345,"owner":"11111111111111111111111111111111","rentEpoch":18446744073709551615,"space":0}},"id":1}`,
				// valid ATA
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"1.14.20","slot":206509845},"value":{"amount":"10000","decimals":6,"uiAmount":0.01,"uiAmountString":"0.01"}},"id":1}`,
				// valid ATA
				`{"context":{"apiVersion":"1.13.3","slot":175635873},"value":{"data":["O0Qss5EhV/E6kz0BNCgtAytf/s0Botvxt3kGCN8ALqctdvBIL2OuEzV5LBqS2x3308rEBwESq+xcukVQUYDkgpg6AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","base64"],"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":0}}`,

				// token account
				`{"context":{"apiVersion":"1.14.17","slot":205924180},"value":[{"account":{"data":{"parsed":{"info":{"isNative":false,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"5VCwKtCXgCJ6kit5FybXjvriW3xELsFDhYrPSqtJNmcD","state":"initialized","tokenAmount":{"amount":"55010000","decimals":6,"uiAmount":55.01,"uiAmountString":"55.01"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":361},"pubkey":"Hrb916EihPAN4T6xad9aVbrd5PfYmiJpvwLKA9XmgcGV"}]}`,
				// priority fee
				`{"jsonrpc":"2.0","result":[{"prioritizationFee": 50,"slot": 252519673},{"prioritizationFee": 100,"slot": 252519674}],"id":1}`,
				// simulation
				`{"jsonrpc":"2.0","result":{"value": {"unitsConsumed": 30000,"logs": [],"accounts": null},"context": {"slot": 328286226}},"id":1}`,
			},
			blockHash:       "DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK",
			toIsATA:         true,
			shouldCreateATA: false,
			err:             "",
		},
		{
			asset:    xc.NewChainConfig(""),
			contract: xc.ContractAddress("4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"),
			resp: []string{
				// valid blockhash
				// `{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"2.0.5","slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","lastValidBlockHeight":308641695}},"id":"6acad392-db4b-4728-9385-2b2f7dd105b1"}`,
				// get-account-info for token account
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"1.18.16","slot":274176079},"value":{"data":["","base58"],"executable":false,"lamports":55028723345,"owner":"11111111111111111111111111111111","rentEpoch":18446744073709551615,"space":0}},"id":1}`,
				// empty ATA
				`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid param: could not find account"},"id":1}`,
				// empty ATA
				`{"context":{"apiVersion":"1.13.3","slot":175636079},"value":null}`,
				// token account
				`{"context":{"apiVersion":"1.14.17","slot":205924180},"value":[{"account":{"data":{"parsed":{"info":{"isNative":false,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"5VCwKtCXgCJ6kit5FybXjvriW3xELsFDhYrPSqtJNmcD","state":"initialized","tokenAmount":{"amount":"55010000","decimals":6,"uiAmount":55.01,"uiAmountString":"55.01"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":361},"pubkey":"Hrb916EihPAN4T6xad9aVbrd5PfYmiJpvwLKA9XmgcGV"}]}`,
				// 0 priority fee
				`{"jsonrpc":"2.0","result":[{"prioritizationFee": 0,"slot": 252519673},{"prioritizationFee": 0,"slot": 252519674}],"id":1}`,
				// simulation
				`{"jsonrpc":"2.0","result":{"value": {"unitsConsumed": 30000,"logs": [],"accounts": null},"context": {"slot": 328286226}},"id":1}`,
			},
			blockHash:       "DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK",
			toIsATA:         false,
			shouldCreateATA: true,
			err:             "",
		},
		{
			asset:    xc.NewChainConfig(""),
			contract: xc.ContractAddress("4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"),
			resp: []string{
				// valid blockhash
				// `{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"2.0.5","slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","lastValidBlockHeight":308641695}},"id":"6acad392-db4b-4728-9385-2b2f7dd105b1"}`,
				// get-account-info for token account
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"1.18.16","slot":274176079},"value":{"data":["","base58"],"executable":false,"lamports":55028723345,"owner":"11111111111111111111111111111111","rentEpoch":18446744073709551615,"space":0}},"id":1}`,
				// empty ATA
				`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid param: could not find account"},"id":1}`,
				// empty ATA
				`{"context":{"apiVersion":"1.13.3","slot":175636079},"value":null}`,
				// multiple token accounts
				`{"context":{"apiVersion":"1.14.20","slot":205932194},"value":[{"account":{"data":{"parsed":{"info":{"isNative":false,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"MYiaXnRnaRCinBxK1usPhLeVA1Bfae4aepdT1pcPeNx","state":"initialized","tokenAmount":{"amount":"5000","decimals":6,"uiAmount":0.005,"uiAmountString":"0.005"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":0},"pubkey":"4j6aPPP22iB7q4NZjfdNBQHd6dvEnfM5PH6XxdzfURph"},{"account":{"data":{"parsed":{"info":{"isNative":false,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"MYiaXnRnaRCinBxK1usPhLeVA1Bfae4aepdT1pcPeNx","state":"initialized","tokenAmount":{"amount":"3000","decimals":6,"uiAmount":0.003,"uiAmountString":"0.003"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":0},"pubkey":"HmmCAv8mBn6piJBbAeHfMDajNzg8H8boKv7gQRijST9J"},{"account":{"data":{"parsed":{"info":{"isNative":false,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"MYiaXnRnaRCinBxK1usPhLeVA1Bfae4aepdT1pcPeNx","state":"initialized","tokenAmount":{"amount":"6000","decimals":6,"uiAmount":0.006,"uiAmountString":"0.006"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":0},"pubkey":"4Nd1Ufsc3gBARS3iHus5RT65kX6LDvJPHsuVPEwxpWwD"}]}`,
				// priority fee
				`{"jsonrpc":"2.0","result":[{"prioritizationFee": 50,"slot": 252519673},{"prioritizationFee": 100,"slot": 252519674}],"id":1}`,
				// simulation
				`{"jsonrpc":"2.0","result":{"value": {"unitsConsumed": 30000,"logs": [],"accounts": null},"context": {"slot": 328286226}},"id":1}`,
			},
			blockHash:         "DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK",
			toIsATA:           false,
			shouldCreateATA:   true,
			tokenAccountCount: 3,
			err:               "",
		},
		{
			asset: xc.NewChainConfig(""),
			resp: []string{
				// invalid blockhash
				`{"context":{"slot":83986105},"value":{"blockhash":"error","feeCalculator":{"lamportsPerSignature":5000}}}`,
			},
			blockHash:       "",
			toIsATA:         false,
			shouldCreateATA: false,
			err:             "rpc.GetLatestBlockhashResult",
		},
		{
			asset:    xc.NewChainConfig(""),
			contract: xc.ContractAddress("invalid-contract"),
			resp: []string{
				// valid blockhash
				// `{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"2.0.5","slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","lastValidBlockHeight":308641695}},"id":"6acad392-db4b-4728-9385-2b2f7dd105b1"}`,
				// get-account-info for token account
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"1.18.16","slot":274176079},"value":{"data":["","base58"],"executable":false,"lamports":55028723345,"owner":"11111111111111111111111111111111","rentEpoch":18446744073709551615,"space":0}},"id":1}`,
				// valid owner account -> error getting token balance
				`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid param: could not find account"},"id":1}`,
				// priority fee
				`{"jsonrpc":"2.0","result":[{"prioritizationFee": 50,"slot": 252519673},{"prioritizationFee": 100,"slot": 252519674}],"id":1}`,
				// simulation
				`{"jsonrpc":"2.0","result":{"value": {"unitsConsumed": 30000,"logs": [],"accounts": null},"context": {"slot": 328286226}},"id":1}`,
			},
			blockHash:       "DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK",
			toIsATA:         false,
			shouldCreateATA: true,
			err:             "decode: invalid base58 digit",
		},
		{
			asset:           xc.NewChainConfig(""),
			resp:            `null`,
			blockHash:       "",
			toIsATA:         false,
			shouldCreateATA: false,
			err:             "error fetching latest blockhash",
		},
		{
			asset:           xc.NewChainConfig(""),
			resp:            `{}`,
			blockHash:       "",
			toIsATA:         false,
			shouldCreateATA: false,
			err:             "error fetching latest blockhash",
		},
		{
			asset:           xc.NewChainConfig(""),
			resp:            fmt.Errorf(`{"message": "custom RPC error", "code": 123}`),
			blockHash:       "",
			toIsATA:         false,
			shouldCreateATA: false,
			err:             "custom RPC error",
		},
	}

	for i, v := range vectors {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			server, close := testtypes.MockJSONRPC(t, v.resp)
			defer close()
			fmt.Println("ASSET", v.asset)
			server.ForceError = v.forceError
			v.asset.URL = server.URL

			client, _ := client.NewClient(v.asset)
			from := xc.Address("4ixwJt7DDGUV3xxi3mvZuEjLn4kDC39ogknnHQ4Crv5a")
			to := xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb")
			args := buildertest.MustNewTransferArgs(from, to, xc.NewAmountBlockchainFromUint64(1), buildertest.OptionContractDecimals(6))
			if v.contract != "" {
				args.SetContract(v.contract)
			}
			input, err := client.FetchTransferInput(context.Background(), args)

			if v.err != "" {
				require.Nil(t, input)
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				require.Equal(t, v.toIsATA, input.(*TxInput).ToIsATA, "ToIsATA")
				require.Equal(t, v.shouldCreateATA, input.(*TxInput).ShouldCreateATA, "ShouldCreateATA")
				require.Equal(t, v.blockHash, input.(*TxInput).RecentBlockHash.String())
				if v.tokenAccountCount > 0 {
					require.Len(t, input.(*TxInput).SourceTokenAccounts, v.tokenAccountCount)
					// token accounts must be sorted descending
					for i := 1; i < v.tokenAccountCount; i++ {
						prior := input.(*TxInput).SourceTokenAccounts[i-1].Balance.Uint64()
						current := input.(*TxInput).SourceTokenAccounts[i].Balance.Uint64()
						require.LessOrEqual(t, current, prior)
					}
				}
			}
		})
	}
}

func TestSubmitTxSuccess(t *testing.T) {
	txbin := "01df5ff457c2cdd23242ab26edd0b308d78499f28c6d43e185149cacdb88b35db171f1779e48ce2224cc80b9b9ce46dd80758319068b08eae34b14dc2cd070ab000100010379726da52d99d60b07ead73b2f6f0bf6083cc85c77a94e34d691d78f8bcafec9fc880863219008406235fa4c8fbb2a86d3da7b6762eac39323b2a1d8c404a4140000000000000000000000000000000000000000000000000000000000000000932bbef1569d58f4a116f41028f766439b2ba52c68c3308bbbea2b21e4716f6701020200010c0200000000ca9a3b00000000"
	bytes, _ := hex.DecodeString(txbin)
	solTx, _ := solana.TransactionFromDecoder(bin.NewBinDecoder(bytes))
	tx := &tx.Tx{
		SolTx: solTx,
	}
	serialized_tx, err := tx.Serialize()
	require.NoError(t, err)

	server, close := testtypes.MockJSONRPC(t, fmt.Sprintf("\"%s\"", tx.Hash()))
	defer close()
	client, _ := client.NewClient(xc.NewChainConfig(xc.SOL).WithUrl(server.URL))
	err = client.SubmitTx(context.Background(), &testtypes.MockXcTx{
		SerializedSignedTx: serialized_tx,
		Signatures:         []xc.TxSignature{{1, 2, 3, 4}},
	})
	require.NoError(t, err)
}
func TestSubmitTxErr(t *testing.T) {

	client, _ := client.NewClient(xc.NewChainConfig(""))
	tx := &tx.Tx{
		SolTx:       &solana.Transaction{},
		ParsedSolTx: &rpc.ParsedTransaction{},
	}
	err := client.SubmitTx(context.Background(), tx)
	require.ErrorContains(t, err, "unsupported protocol scheme")
}

func TestAccountBalance(t *testing.T) {

	vectors := []struct {
		resp interface{}
		val  string
		err  string
	}{
		{
			`{"value": 123}`,
			"123",
			"",
		},
		{
			`null`,
			"0",
			"",
		},
		{
			`{}`,
			"0",
			"",
		},
		{
			fmt.Errorf(`{"message": "custom RPC error", "code": 123}`),
			"",
			"custom RPC error",
		},
	}

	for _, v := range vectors {
		server, close := testtypes.MockJSONRPC(t, v.resp)
		defer close()

		client, _ := client.NewClient(xc.NewChainConfig(xc.SOL).WithUrl(server.URL))
		from := xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb")
		balance, err := client.FetchNativeBalance(context.Background(), from)

		if v.err != "" {
			require.Equal(t, "0", balance.String())
			require.ErrorContains(t, err, v.err)
		} else {
			require.Nil(t, err)
			require.NotNil(t, balance)
			require.Equal(t, v.val, balance.String())
		}
	}
}

func TestTokenBalance(t *testing.T) {

	vectors := []struct {
		resp interface{}
		val  string
		err  string
	}{
		{
			`{"context":{"apiVersion":"1.14.20","slot":205924046},"value":[{"account":{"data":{"parsed":{"info":
				{"isNative":false,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"5VCwKtCXgCJ6kit5FybXjvriW3xELsFDhYrPSqtJNmcD","state":"initialized",
				"tokenAmount":{"amount":"9864","decimals":2,"uiAmount":98.64,"uiAmountString":"98.64"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":361},"pubkey":"Hrb916EihPAN4T6xad9aVbrd5PfYmiJpvwLKA9XmgcGV"}]}`,
			"9864",
			"",
		},
		{
			`null`,
			"0",
			"failed to get balance",
		},
		{
			`{}`,
			"0",
			"failed to get balance",
		},
		{
			fmt.Errorf(`{"message": "custom RPC error", "code": 123}`),
			"",
			"custom RPC error",
		},
	}

	for i, v := range vectors {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			server, close := testtypes.MockJSONRPC(t, v.resp)
			defer close()

			client, _ := client.NewClient(xc.NewChainConfig("").WithUrl(server.URL))
			from := xc.Address("Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb")
			args := xclient.NewBalanceArgs(
				from,
				xclient.OptionContract(xc.ContractAddress("4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU")),
			)
			balance, err := client.FetchBalance(context.Background(), args)

			if v.err != "" {
				require.Equal(t, "0", balance.String())
				require.ErrorContains(t, err, v.err)
			} else {
				require.Nil(t, err)
				require.NotNil(t, balance)
				require.Equal(t, v.val, balance.String())
			}
		})
	}
}

func TestFetchTxInfo(t *testing.T) {

	vectors := []struct {
		tx   string
		resp interface{}
		val  xclient.LegacyTxInfo
		err  string
	}{
		{
			// 1 SOL
			"5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw",
			[]string{
				`{"blockTime":1650017168,"meta":{"err":null,"fee":5000,"innerInstructions":[],"loadedAddresses":{"readonly":[],"writable":[]},"logMessages":["Program 11111111111111111111111111111111 invoke [1]","Program 11111111111111111111111111111111 success"],"postBalances":[19921026477997237,1869985000,1],"postTokenBalances":[],"preBalances":[19921027478002237,869985000,1],"preTokenBalances":[],"rewards":[],"status":{"Ok":null}},"slot":128184605,"transaction":["Ad9f9FfCzdIyQqsm7dCzCNeEmfKMbUPhhRScrNuIs12xcfF3nkjOIiTMgLm5zkbdgHWDGQaLCOrjSxTcLNBwqwABAAEDeXJtpS2Z1gsH6tc7L28L9gg8yFx3qU401pHXj4vK/sn8iAhjIZAIQGI1+kyPuyqG09p7Z2Lqw5MjsqHYxASkFAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAkyu+8VadWPShFvQQKPdmQ5srpSxowzCLu+orIeRxb2cBAgIAAQwCAAAAAMqaOwAAAAA=","base64"]}`,
				`{"context":{"slot":128184606},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
			},
			xclient.LegacyTxInfo{
				TxID:            "5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw",
				From:            "9B5XszUGdMaxCZ7uSQhPzdks5ZQSmWxrmzCSvtJ6Ns6g",
				To:              "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
				ToAlt:           "",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(1000000000),
				Fee:             xc.NewAmountBlockchainFromUint64(5000),
				BlockIndex:      128184605,
				BlockTime:       1650017168,
				Confirmations:   1,
				Sources: []*xclient.LegacyTxInfoEndpoint{
					{
						Address: "9B5XszUGdMaxCZ7uSQhPzdks5ZQSmWxrmzCSvtJ6Ns6g",
						Amount:  xc.NewAmountBlockchainFromUint64(1000000000),
						Event:   xclient.NewEvent("1", xclient.MovementVariantNative),
					}},
				Destinations: []*xclient.LegacyTxInfoEndpoint{
					{Address: "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
						Amount: xc.NewAmountBlockchainFromUint64(1000000000),
						Event:  xclient.NewEvent("1", xclient.MovementVariantNative),
					}},
			},
			"",
		},
		{
			// 0.12 SOL
			"3XRGeupw3XacNQ4op3TQdWJsX3VvSnzQdjBvQDjGHaTCZs1eJzbuVn67RThFXEBSDBvoCXT5eX7rU1frQLni5AKb",
			[]string{
				`{"blockTime":1645123751,"meta":{"err":null,"fee":5000,"innerInstructions":[],"loadedAddresses":{"readonly":[],"writable":[]},"logMessages":["Program 11111111111111111111111111111111 invoke [1]","Program 11111111111111111111111111111111 success"],"postBalances":[879990000,1420000000,1],"postTokenBalances":[],"preBalances":[999995000,1300000000,1],"preTokenBalances":[],"rewards":[],"status":{"Ok":null}},"slot":115310825,"transaction":["AX5EBZa5UnMbHNgzEDz8dn1mcrTjLwLsLC3Ph3tMgQshAb2hEkbkkUQleXVJqmcTYmxnnw3jIXOjfR3lGvw8pQoBAAED/IgIYyGQCEBiNfpMj7sqhtPae2di6sOTI7Kh2MQEpBR3FzzGpO7sbgIIhX1XFeQKpFBxBTrVYewdaBjV/jf96AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAZ3rYIt4WDe4pwTzQI6YOAbSxt/Orf5UkTzqKqXN1KMoBAgIAAQwCAAAAAA4nBwAAAAA=","base64"]}`,
				`{"context":{"slot":115310827},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
			},
			xclient.LegacyTxInfo{
				TxID:            "3XRGeupw3XacNQ4op3TQdWJsX3VvSnzQdjBvQDjGHaTCZs1eJzbuVn67RThFXEBSDBvoCXT5eX7rU1frQLni5AKb",
				From:            "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
				To:              "91t4uSdtBiftqsB24W2fRXFCXjUyc6xY3WMGFedAaTHh",
				ToAlt:           "",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(120000000),
				Fee:             xc.NewAmountBlockchainFromUint64(5000),
				BlockIndex:      115310825,
				BlockTime:       1645123751,
				Confirmations:   2,
				Sources: []*xclient.LegacyTxInfoEndpoint{
					{
						Address: "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
						Amount:  xc.NewAmountBlockchainFromUint64(120000000),
						Event:   xclient.NewEvent("1", xclient.MovementVariantNative),
					},
				},
				Destinations: []*xclient.LegacyTxInfoEndpoint{
					{
						Address: "91t4uSdtBiftqsB24W2fRXFCXjUyc6xY3WMGFedAaTHh",
						Amount:  xc.NewAmountBlockchainFromUint64(120000000),
						Event:   xclient.NewEvent("1", xclient.MovementVariantNative),
					},
				},
				Status: 0,
			},
			"",
		},
		{
			// 0.001 USDC
			"ZJaJTB5oLfPrzEsFE2cEa94KdNb6SGvqMgaLdtqoYFnaqo4zAncVPjkpDqPbVPv85S68zNcaTyYobDcPJuRfhrX",
			[]string{
				`{"blockTime":1645120351,"meta":{"err":null,"fee":5000,"innerInstructions":[{"index":0,"instructions":[{"accounts":[0,1],"data":"3Bxs4h24hBtQy9rw","programIdIndex":5},{"accounts":[1],"data":"9krTDU2LzCSUJuVZ","programIdIndex":5},{"accounts":[1],"data":"SYXsBSQy3GeifSEQSGvTbrPNposbSAiSoh1YA85wcvGKSnYg","programIdIndex":5},{"accounts":[1,4,3,7],"data":"2","programIdIndex":6}]}],"loadedAddresses":{"readonly":[],"writable":[]},"logMessages":["Program ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL invoke [1]","Program log: Transfer 2039280 lamports to the associated token account","Program 11111111111111111111111111111111 invoke [2]","Program 11111111111111111111111111111111 success","Program log: Allocate space for the associated token account","Program 11111111111111111111111111111111 invoke [2]","Program 11111111111111111111111111111111 success","Program log: Assign the associated token account to the SPL Token program","Program 11111111111111111111111111111111 invoke [2]","Program 11111111111111111111111111111111 success","Program log: Initialize the associated token account","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA invoke [2]","Program log: Instruction: InitializeAccount","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA consumed 3297 of 169352 compute units","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA success","Program ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL consumed 34626 of 200000 compute units","Program ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL success","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA invoke [1]","Program log: Instruction: TransferChecked","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA consumed 3414 of 200000 compute units","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA success"],"postBalances":[4995907578480,2039280,2002039280,0,1461600,1,953185920,1009200,898174080],"postTokenBalances":[{"accountIndex":1,"mint":"4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU","owner":"Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb","uiTokenAmount":{"amount":"1000000","decimals":6,"uiAmount":1.0,"uiAmountString":"1"}},{"accountIndex":2,"mint":"4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU","owner":"HzcTrHjkEhjFTHEsC6Dsv8DXCh21WgujD4s5M15Sm94g","uiTokenAmount":{"amount":"9437986064320000","decimals":6,"uiAmount":9437986064.32,"uiAmountString":"9437986064.32"}}],"preBalances":[4995909622760,0,2002039280,0,1461600,1,953185920,1009200,898174080],"preTokenBalances":[{"accountIndex":2,"mint":"4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU","owner":"HzcTrHjkEhjFTHEsC6Dsv8DXCh21WgujD4s5M15Sm94g","uiTokenAmount":{"amount":"9437986065320000","decimals":6,"uiAmount":9437986065.32,"uiAmountString":"9437986065.32"}}],"rewards":[],"status":{"Ok":null}},"slot":115302132,"transaction":["AWHQR1wAwmzbcOtoAVvDi4tkCQD/n6Pnks/038BYBd5o3oh+QZDKHR0Onl9j+AFGp5wziV4cS96gDbzm4RYebwsBAAYJ/H0x63TSPiBNuaFOZG+ZK2YJNAoqn9Gpp2i1PA5g++W//Q+h80WCx4fF9949OkDj+1D/UUOKp6XPufludArJk2as/hRuCozLqVIK567QNivd/o4dGEy7JdaJah9qp8KU/IgIYyGQCEBiNfpMj7sqhtPae2di6sOTI7Kh2MQEpBQ7RCyzkSFX8TqTPQE0KC0DK1/+zQGi2/G3eQYI3wAupwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABt324ddloZPZy+FGzut5rBy0he1fWzeROoz1hX7/AKkGp9UXGSxcUSGMyUw9SvF/WNruCJuh/UTj29mKAAAAAIyXJY9OJInxuz0QKRSODYMLWhOZ2v8QhASOe9jb6fhZNT+YJRqOG5qSK+OHTJJvIkfUNScIwvHTlo4R2xjWx7cCCAcAAQMEBQYHAAYFAgQBAAAKDEBCDwAAAAAABg==","base64"]}`,
				`{"context":{"slot":115302135},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				// `{"context":{"apiVersion":"1.13.2","slot":169710435},"value":{"data":["O0Qss5EhV/E6kz0BNCgtAytf/s0Botvxt3kGCN8ALqf8iAhjIZAIQGI1+kyPuyqG09p7Z2Lqw5MjsqHYxASkFAA1DAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","base64"],"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":371}}`,
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"1.14.17","slot":205923735},"value":{"data":{"parsed":{"info":{"isNative":false,"mint":"4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU","owner":"DvSgNMRxVSMBpLp4hZeBrmQo8ZRFne72actTZ3PYE3AA","state":"initialized","tokenAmount":{"amount":"100","decimals":6,"uiAmount":0.001,"uiAmountString":"0.001"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":0}},"id":1}`,
			},
			xclient.LegacyTxInfo{
				TxID:            "ZJaJTB5oLfPrzEsFE2cEa94KdNb6SGvqMgaLdtqoYFnaqo4zAncVPjkpDqPbVPv85S68zNcaTyYobDcPJuRfhrX",
				From:            "HzcTrHjkEhjFTHEsC6Dsv8DXCh21WgujD4s5M15Sm94g",
				To:              "DvSgNMRxVSMBpLp4hZeBrmQo8ZRFne72actTZ3PYE3AA",
				ContractAddress: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
				Amount:          xc.NewAmountBlockchainFromUint64(1000000),
				Fee:             xc.NewAmountBlockchainFromUint64(5000),
				Sources: []*xclient.LegacyTxInfoEndpoint{{
					Address:         "HzcTrHjkEhjFTHEsC6Dsv8DXCh21WgujD4s5M15Sm94g",
					ContractAddress: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
					Amount:          xc.NewAmountBlockchainFromUint64(1000000),
					Event:           xclient.NewEvent("2", xclient.MovementVariantNative),
				},
				},
				Destinations: []*xclient.LegacyTxInfoEndpoint{
					{
						Address:         "DvSgNMRxVSMBpLp4hZeBrmQo8ZRFne72actTZ3PYE3AA",
						ContractAddress: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
						Amount:          xc.NewAmountBlockchainFromUint64(1000000),
						Event:           xclient.NewEvent("2", xclient.MovementVariantNative),
					},
				},
				BlockIndex:    115302132,
				BlockTime:     1645120351,
				Confirmations: 3,
			},
			"",
		},
		{
			// 0.0002 USDC
			"5ZrG8iS4RxLXDRQEWkAoddWHzkS1fA1m6ppxaAekgGzskhcFqjkw1ZaFCsLorbhY5V4YUUkjE3SLY2JNLyVanxrM",
			[]string{
				`{"blockTime":1645121566,"meta":{"err":null,"fee":5000,"innerInstructions":[],"loadedAddresses":{"readonly":[],"writable":[]},"logMessages":["Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA invoke [1]","Program log: Instruction: TransferChecked","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA consumed 3285 of 200000 compute units","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA success"],"postBalances":[999995000,2039280,2039280,1461600,953185920],"postTokenBalances":[{"accountIndex":1,"mint":"4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU","owner":"91t4uSdtBiftqsB24W2fRXFCXjUyc6xY3WMGFedAaTHh","uiTokenAmount":{"amount":"1200000","decimals":6,"uiAmount":1.2,"uiAmountString":"1.2"}},{"accountIndex":2,"mint":"4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU","owner":"Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb","uiTokenAmount":{"amount":"800000","decimals":6,"uiAmount":0.8,"uiAmountString":"0.8"}}],"preBalances":[1000000000,2039280,2039280,1461600,953185920],"preTokenBalances":[{"accountIndex":1,"mint":"4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU","owner":"91t4uSdtBiftqsB24W2fRXFCXjUyc6xY3WMGFedAaTHh","uiTokenAmount":{"amount":"1000000","decimals":6,"uiAmount":1.0,"uiAmountString":"1"}},{"accountIndex":2,"mint":"4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU","owner":"Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb","uiTokenAmount":{"amount":"1000000","decimals":6,"uiAmount":1.0,"uiAmountString":"1"}}],"rewards":[],"status":{"Ok":null}},"slot":115305244,"transaction":["AeRlYJzP6QA39jotdCCkfUXkwSfdikREPsky500UJkzv9WjwqWJRE1AVBY7gaDVCoHUXzBdRP2HqHa+yk6AOpAIBAAIF/IgIYyGQCEBiNfpMj7sqhtPae2di6sOTI7Kh2MQEpBRSZ7LN9mT5H4I0p8JRr5XuoV6s6jPU4jO/AY/AlB3Scr/9D6HzRYLHh8X33j06QOP7UP9RQ4qnpc+5+W50CsmTO0Qss5EhV/E6kz0BNCgtAytf/s0Botvxt3kGCN8ALqcG3fbh12Whk9nL4UbO63msHLSF7V9bN5E6jPWFfv8AqSRIX/y7kCeqnCwBTRUU6oea3zqDrhoNbdKX6z3IVPmkAQQEAgMBAAoMQA0DAAAAAAAG","base64"]}`,
				`{"context":{"slot":115305248},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"1.14.17","slot":205923735},"value":{"data":{"parsed":{"info":{"isNative":false,"mint":"4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU","owner":"6Yg9GttAiHjbHMoiomBuGBDULP7HxQyez45dEiR9CJqw","state":"initialized","tokenAmount":{"amount":"20","decimals":6,"uiAmount":0.0002,"uiAmountString":"0.0002"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":0}},"id":1}`,
			},
			xclient.LegacyTxInfo{
				TxID:            "5ZrG8iS4RxLXDRQEWkAoddWHzkS1fA1m6ppxaAekgGzskhcFqjkw1ZaFCsLorbhY5V4YUUkjE3SLY2JNLyVanxrM",
				From:            "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
				To:              "6Yg9GttAiHjbHMoiomBuGBDULP7HxQyez45dEiR9CJqw",
				ContractAddress: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
				Amount:          xc.NewAmountBlockchainFromUint64(200000),
				Sources: []*xclient.LegacyTxInfoEndpoint{
					{
						Address:         "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
						ContractAddress: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
						Amount:          xc.NewAmountBlockchainFromUint64(200000),
						Event:           xclient.NewEvent("1", xclient.MovementVariantNative),
					},
				},
				Destinations: []*xclient.LegacyTxInfoEndpoint{
					{
						Address:         "6Yg9GttAiHjbHMoiomBuGBDULP7HxQyez45dEiR9CJqw",
						ContractAddress: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
						Amount:          xc.NewAmountBlockchainFromUint64(200000),
						Event:           xclient.NewEvent("1", xclient.MovementVariantNative),
					},
				},
				Fee:           xc.NewAmountBlockchainFromUint64(5000),
				BlockIndex:    115305244,
				BlockTime:     1645121566,
				Confirmations: 4,
			},
			"",
		},
		{
			// 3,170,652,014,400,000 BONK
			// sent using Transfer instead of TransferChecked
			"66iwZvSCQc1br36ddj7keyLtSXb3yuPzDdMSk3qpkYJUAiiy3thmpzut1WzEWjnubr8oQV19wkhvH3X9j45kPZzx",
			[]string{
				`{"blockTime":1690153132,"meta":{"computeUnitsConsumed":4906,"err":null,"fee":10000,"innerInstructions":[],"loadedAddresses":{"readonly":[],"writable":[]},"logMessages":["Program 11111111111111111111111111111111 invoke [1]","Program 11111111111111111111111111111111 success","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA invoke [1]","Program log: Instruction: Transfer","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA consumed 4906 of 400000 compute units","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA success"],"postBalances":[403171402509284,9718964400,1447680,2039280,2039280,42706560,1,934087680],"postTokenBalances":[{"accountIndex":3,"mint":"DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263","owner":"AC5RDfQFmDS1deWZos921JfqscXdByf8BKHs5ACWjtW2","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"382080963725226836","decimals":5,"uiAmount":3820809637252.268,"uiAmountString":"3820809637252.26836"}},{"accountIndex":4,"mint":"DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263","owner":"GtxgnRiSfBzahR9xb7hvYbWq3Uzez7hpCz2BJbCLxKdq","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"32212708779800000","decimals":5,"uiAmount":322127087798.0,"uiAmountString":"322127087798"}}],"preBalances":[403171402519284,9718964400,1447680,2039280,2039280,42706560,1,934087680],"preTokenBalances":[{"accountIndex":3,"mint":"DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263","owner":"AC5RDfQFmDS1deWZos921JfqscXdByf8BKHs5ACWjtW2","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"385251615739626836","decimals":5,"uiAmount":3852516157396.268,"uiAmountString":"3852516157396.26836"}},{"accountIndex":4,"mint":"DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263","owner":"GtxgnRiSfBzahR9xb7hvYbWq3Uzez7hpCz2BJbCLxKdq","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"29042056765400000","decimals":5,"uiAmount":290420567654.0,"uiAmountString":"290420567654"}}],"rewards":[],"status":{"Ok":null}},"slot":207121234,"transaction":["Av8FOF0kjNnWUXPbpt+FtgqWelW97InTZA43ZjGG2gVYvDZ6ALXfFEQp+zM418XAIXTTNrp1giZtBWiHzZOrsQF8quddFkkixTxvDCAK/CKNgohm8krjk7lXBQA/PNkqmIkQLCq3OsSld1RbykqJ991PIiorRyOEBYtSIR7oVWsFAgEDCIiPkPF7lPXeQRe3/M9ndHf3KzQ6duqul8PtMmScMvQ/goOWqOOKvTeqsU2DOUEzLCfh/G9JKAsSMoSArVGCj+76bP6e8td9znud99Ag6O6gVor611aun3c/s7g/1Sxpn//C/fFM4vo5abxlD/cXTW6BkM+8hiAYrsm3j3zaALp0vOgWQmOt+noP+6lFK2FHxOyzyOgmDBUTAFeq96ZycnMGp9UXGSxWjuCKhF9z0peIzwNcMUWyGrNE2AYuqUAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABt324ddloZPZy+FGzut5rBy0he1fWzeROoz1hX7/AKki68scur9rwzLR19+wDicGs0RvD9ugv7rUitFS1TMSfwIGAwIFAQQEAAAABwUDBAAAAQkDAFIG87BDCwA=","base64"]}`,
				`{"context":{"apiVersion":"1.14.20","slot":207847505},"value":{"blockhash":"9xWSgdL1GkwydD5uHX2WwKFSC9U2mqZiSXe4LSCa9ciR","feeCalculator":{"lamportsPerSignature":5000}}}`,
				`{"context":{"apiVersion":"1.14.20","slot":207847505},"value":{"data":{"parsed":{"info":{"isNative":false,"mint":"DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263","owner":"GtxgnRiSfBzahR9xb7hvYbWq3Uzez7hpCz2BJbCLxKdq","state":"initialized","tokenAmount":{"amount":"31087540303300000","decimals":5,"uiAmount":310875403033.0,"uiAmountString":"310875403033"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":0}}`,
			},
			xclient.LegacyTxInfo{
				TxID:            "66iwZvSCQc1br36ddj7keyLtSXb3yuPzDdMSk3qpkYJUAiiy3thmpzut1WzEWjnubr8oQV19wkhvH3X9j45kPZzx",
				From:            "AC5RDfQFmDS1deWZos921JfqscXdByf8BKHs5ACWjtW2",
				To:              "GtxgnRiSfBzahR9xb7hvYbWq3Uzez7hpCz2BJbCLxKdq",
				ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
				Amount:          xc.NewAmountBlockchainFromUint64(3170652014400000),
				Sources: []*xclient.LegacyTxInfoEndpoint{
					{
						Address:         "AC5RDfQFmDS1deWZos921JfqscXdByf8BKHs5ACWjtW2",
						ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
						Amount:          xc.NewAmountBlockchainFromUint64(3170652014400000),
						Event:           xclient.NewEvent("2", xclient.MovementVariantNative),
					},
				},
				Destinations: []*xclient.LegacyTxInfoEndpoint{
					{
						Address:         "GtxgnRiSfBzahR9xb7hvYbWq3Uzez7hpCz2BJbCLxKdq",
						ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
						Amount:          xc.NewAmountBlockchainFromUint64(3170652014400000),
						Event:           xclient.NewEvent("2", xclient.MovementVariantNative),
					},
				},
				Fee:           xc.NewAmountBlockchainFromUint64(10000),
				BlockIndex:    207121234,
				BlockTime:     1690153132,
				Confirmations: 726271,
			},
			"",
		},
		{
			// Failed transaction sending 0.12 SOL
			"3XRGeupw3XacNQ4op3TQdWJsX3VvSnzQdjBvQDjGHaTCZs1eJzbuVn67RThFXEBSDBvoCXT5eX7rU1frQLni5AKb",
			[]string{
				`{"blockTime":1645123751,"meta":{"err":{"InsufficientFundsForRent":{"account_index":1}},"fee":5000,"innerInstructions":[],"loadedAddresses":{"readonly":[],"writable":[]},"logMessages":["Program 11111111111111111111111111111111 invoke [1]","Program 11111111111111111111111111111111 success"],"postBalances":[879990000,1420000000,1],"postTokenBalances":[],"preBalances":[999995000,1300000000,1],"preTokenBalances":[],"rewards":[],"status":{"Ok":null}},"slot":115310825,"transaction":["AX5EBZa5UnMbHNgzEDz8dn1mcrTjLwLsLC3Ph3tMgQshAb2hEkbkkUQleXVJqmcTYmxnnw3jIXOjfR3lGvw8pQoBAAED/IgIYyGQCEBiNfpMj7sqhtPae2di6sOTI7Kh2MQEpBR3FzzGpO7sbgIIhX1XFeQKpFBxBTrVYewdaBjV/jf96AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAZ3rYIt4WDe4pwTzQI6YOAbSxt/Orf5UkTzqKqXN1KMoBAgIAAQwCAAAAAA4nBwAAAAA=","base64"]}`,
				`{"context":{"slot":115310827},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
			},
			xclient.LegacyTxInfo{
				TxID:            "3XRGeupw3XacNQ4op3TQdWJsX3VvSnzQdjBvQDjGHaTCZs1eJzbuVn67RThFXEBSDBvoCXT5eX7rU1frQLni5AKb",
				From:            "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
				To:              "91t4uSdtBiftqsB24W2fRXFCXjUyc6xY3WMGFedAaTHh",
				ToAlt:           "",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(120000000),
				Fee:             xc.NewAmountBlockchainFromUint64(5000),
				BlockIndex:      115310825,
				BlockTime:       1645123751,
				Confirmations:   2,
				Sources:         nil,
				Destinations:    nil,
				Status:          0,
				Error:           "{\"InsufficientFundsForRent\":{\"account_index\":1}}",
			},
			"",
		},
		{
			"5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw",
			`{}`,
			xclient.LegacyTxInfo{},
			"invalid transaction in response",
		},
		{
			"5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw",
			`null`,
			xclient.LegacyTxInfo{},
			"TransactionNotFound: not found",
		},
		{
			"5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw",
			fmt.Errorf(`{"message": "custom RPC error", "code": 123}`),
			xclient.LegacyTxInfo{},
			"custom RPC error",
		},
		{
			"",
			"",
			xclient.LegacyTxInfo{},
			"zero length string",
		},
		{
			"invalid-sig",
			"",
			xclient.LegacyTxInfo{},
			"invalid base58 digit",
		},
		{
			// 1 SOL
			"5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw",
			[]string{
				`{"blockTime":1650017168,"meta":{"err":null,"fee":5000,"innerInstructions":[],"loadedAddresses":{"readonly":[],"writable":[]},"logMessages":["Program 11111111111111111111111111111111 invoke [1]","Program 11111111111111111111111111111111 success"],"postBalances":[19921026477997237,1869985000,1],"postTokenBalances":[],"preBalances":[19921027478002237,869985000,1],"preTokenBalances":[],"rewards":[],"status":{"Ok":null}},"slot":128184605,"transaction":["invalid-binary","base64"]}`,
				`{"context":{"slot":128184606},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
			},
			xclient.LegacyTxInfo{},
			"illegal base64 data",
		},
	}

	for i, v := range vectors {
		t.Run(fmt.Sprintf("test_case_%d", i), func(t *testing.T) {
			fmt.Println("test case ", i)
			server, close := testtypes.MockJSONRPC(t, v.resp)
			defer close()

			client, _ := client.NewClient(xc.NewChainConfig(xc.SOL).WithUrl(server.URL))
			txInfo, err := client.FetchLegacyTxInfo(context.Background(), xc.TxHash(v.tx))

			if v.err != "" {
				require.Equal(t, xclient.LegacyTxInfo{}, txInfo)
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, txInfo)
				require.Equal(t, v.val, txInfo)
			}
		})
	}
}
