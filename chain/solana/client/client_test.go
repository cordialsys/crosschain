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
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	testtypes "github.com/cordialsys/crosschain/testutil"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
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
		tokenProgram      string
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
			// Token2022
			asset:    xc.NewChainConfig(""),
			contract: xc.ContractAddress("4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU"),
			resp: []string{
				// valid blockhash
				// `{"context":{"slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"2.0.5","slot":83986105},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","lastValidBlockHeight":308641695}},"id":"6acad392-db4b-4728-9385-2b2f7dd105b1"}`,
				// get-account-info for token account
				`{"jsonrpc":"2.0","result":{"context":{"apiVersion":"1.18.16","slot":274176079},"value":{"data":["","base58"],"executable":false,"lamports":55028723345,"owner":"TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb","rentEpoch":18446744073709551615,"space":0}},"id":1}`,
				// valid owner account
				`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid param: could not find account"},"id":1}`,
				// empty ATA
				`{"context":{"apiVersion":"1.13.3","slot":175636079},"value":null}`,
				// token account
				`{"context":{"apiVersion":"1.14.17","slot":205924180},"value":[{"account":{"data":{"parsed":{"info":{"isNative":false,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"5VCwKtCXgCJ6kit5FybXjvriW3xELsFDhYrPSqtJNmcD","state":"initialized","tokenAmount":{"amount":"55010000","decimals":6,"uiAmount":55.01,"uiAmountString":"55.01"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb","rentEpoch":361},"pubkey":"Hrb916EihPAN4T6xad9aVbrd5PfYmiJpvwLKA9XmgcGV"}]}`,
				// priority fee
				`{"jsonrpc":"2.0","result":[{"prioritizationFee": 50,"slot": 252519673},{"prioritizationFee": 100,"slot": 252519674}],"id":1}`,
				// simulation
				`{"jsonrpc":"2.0","result":{"value": {"unitsConsumed": 30000,"logs": [],"accounts": null},"context": {"slot": 328286226}},"id":1}`,
			},
			blockHash:       "DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK",
			tokenProgram:    solana.Token2022ProgramID.String(),
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
			chainCfg := v.asset.Base()
			args := buildertest.MustNewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1), buildertest.OptionContractDecimals(6))
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
				if v.tokenProgram != "" {
					require.Equal(t, v.tokenProgram, input.(*TxInput).TokenProgram.String())
				}
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
	req, err := xctypes.SubmitTxReqFromTx(xc.SOL, &testtypes.MockXcTx{
		SerializedSignedTx: serialized_tx,
		Signatures:         []xc.TxSignature{{1, 2, 3, 4}},
	})
	require.NoError(t, err)
	err = client.SubmitTx(context.Background(), req)
	require.NoError(t, err)
}
func TestSubmitTxErr(t *testing.T) {

	client, _ := client.NewClient(xc.NewChainConfig(""))
	tx, err := xctypes.SubmitTxReqFromTx(xc.SOL, &tx.Tx{
		SolTx: &solana.Transaction{},
	})
	require.NoError(t, err)
	err = client.SubmitTx(context.Background(), tx)
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
				xclient.BalanceOptionContract(xc.ContractAddress("4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU")),
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
		val  txinfo.LegacyTxInfo
		err  string
	}{
		{
			// 1 SOL
			"5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw",
			[]string{
				`{"blockTime":1650017168,"meta":{"err":null,"fee":5000,"innerInstructions":[],"loadedAddresses":{"readonly":[],"writable":[]},"logMessages":["Program 11111111111111111111111111111111 invoke [1]","Program 11111111111111111111111111111111 success"],"postBalances":[19921026477997237,1869985000,1],"postTokenBalances":[],"preBalances":[19921027478002237,869985000,1],"preTokenBalances":[],"rewards":[],"status":{"Ok":null}},"slot":128184605,"transaction":["Ad9f9FfCzdIyQqsm7dCzCNeEmfKMbUPhhRScrNuIs12xcfF3nkjOIiTMgLm5zkbdgHWDGQaLCOrjSxTcLNBwqwABAAEDeXJtpS2Z1gsH6tc7L28L9gg8yFx3qU401pHXj4vK/sn8iAhjIZAIQGI1+kyPuyqG09p7Z2Lqw5MjsqHYxASkFAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAkyu+8VadWPShFvQQKPdmQ5srpSxowzCLu+orIeRxb2cBAgIAAQwCAAAAAMqaOwAAAAA=","base64"]}`,
				`{"context":{"slot":128184606},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
			},
			txinfo.LegacyTxInfo{
				TxID:            "5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw",
				From:            "9B5XszUGdMaxCZ7uSQhPzdks5ZQSmWxrmzCSvtJ6Ns6g",
				To:              "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
				FeePayer:        "9B5XszUGdMaxCZ7uSQhPzdks5ZQSmWxrmzCSvtJ6Ns6g",
				ToAlt:           "",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(1000000000),
				Fee:             xc.NewAmountBlockchainFromUint64(5000),
				BlockIndex:      128184605,
				BlockTime:       1650017168,
				Confirmations:   1,
				Sources: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address: "9B5XszUGdMaxCZ7uSQhPzdks5ZQSmWxrmzCSvtJ6Ns6g",
						Amount:  xc.NewAmountBlockchainFromUint64(1000000000),
						Event:   txinfo.NewEvent("1", txinfo.MovementVariantNative),
					}},
				Destinations: []*txinfo.LegacyTxInfoEndpoint{
					{Address: "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
						Amount: xc.NewAmountBlockchainFromUint64(1000000000),
						Event:  txinfo.NewEvent("1", txinfo.MovementVariantNative),
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
			txinfo.LegacyTxInfo{
				TxID:            "3XRGeupw3XacNQ4op3TQdWJsX3VvSnzQdjBvQDjGHaTCZs1eJzbuVn67RThFXEBSDBvoCXT5eX7rU1frQLni5AKb",
				From:            "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
				To:              "91t4uSdtBiftqsB24W2fRXFCXjUyc6xY3WMGFedAaTHh",
				ToAlt:           "",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(120000000),
				Fee:             xc.NewAmountBlockchainFromUint64(5000),
				FeePayer:        "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
				BlockIndex:      115310825,
				BlockTime:       1645123751,
				Confirmations:   2,
				Sources: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address: "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
						Amount:  xc.NewAmountBlockchainFromUint64(120000000),
						Event:   txinfo.NewEvent("1", txinfo.MovementVariantNative),
					},
				},
				Destinations: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address: "91t4uSdtBiftqsB24W2fRXFCXjUyc6xY3WMGFedAaTHh",
						Amount:  xc.NewAmountBlockchainFromUint64(120000000),
						Event:   txinfo.NewEvent("1", txinfo.MovementVariantNative),
					},
				},
				Status: 0,
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
			txinfo.LegacyTxInfo{
				TxID:            "5ZrG8iS4RxLXDRQEWkAoddWHzkS1fA1m6ppxaAekgGzskhcFqjkw1ZaFCsLorbhY5V4YUUkjE3SLY2JNLyVanxrM",
				From:            "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
				To:              "6Yg9GttAiHjbHMoiomBuGBDULP7HxQyez45dEiR9CJqw",
				ContractAddress: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
				Amount:          xc.NewAmountBlockchainFromUint64(200000),
				Sources: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
						ContractAddress: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
						Amount:          xc.NewAmountBlockchainFromUint64(200000),
						Event:           txinfo.NewEvent("1", txinfo.MovementVariantNative),
					},
				},
				Destinations: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "6Yg9GttAiHjbHMoiomBuGBDULP7HxQyez45dEiR9CJqw",
						ContractAddress: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDU",
						Amount:          xc.NewAmountBlockchainFromUint64(200000),
						Event:           txinfo.NewEvent("1", txinfo.MovementVariantNative),
					},
				},
				Fee:           xc.NewAmountBlockchainFromUint64(5000),
				FeePayer:      "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
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
			txinfo.LegacyTxInfo{
				TxID:            "66iwZvSCQc1br36ddj7keyLtSXb3yuPzDdMSk3qpkYJUAiiy3thmpzut1WzEWjnubr8oQV19wkhvH3X9j45kPZzx",
				From:            "AC5RDfQFmDS1deWZos921JfqscXdByf8BKHs5ACWjtW2",
				To:              "GtxgnRiSfBzahR9xb7hvYbWq3Uzez7hpCz2BJbCLxKdq",
				ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
				Amount:          xc.NewAmountBlockchainFromUint64(3170652014400000),
				Sources: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "AC5RDfQFmDS1deWZos921JfqscXdByf8BKHs5ACWjtW2",
						ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
						Amount:          xc.NewAmountBlockchainFromUint64(3170652014400000),
						Event:           txinfo.NewEvent("2", txinfo.MovementVariantNative),
					},
				},
				Destinations: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "GtxgnRiSfBzahR9xb7hvYbWq3Uzez7hpCz2BJbCLxKdq",
						ContractAddress: "DezXAZ8z7PnrnRJjz3wXBoRgixCa6xjnB7YaB1pPB263",
						Amount:          xc.NewAmountBlockchainFromUint64(3170652014400000),
						Event:           txinfo.NewEvent("2", txinfo.MovementVariantNative),
					},
				},
				Fee:           xc.NewAmountBlockchainFromUint64(10000),
				FeePayer:      "AC5RDfQFmDS1deWZos921JfqscXdByf8BKHs5ACWjtW2",
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
			txinfo.LegacyTxInfo{
				TxID:            "3XRGeupw3XacNQ4op3TQdWJsX3VvSnzQdjBvQDjGHaTCZs1eJzbuVn67RThFXEBSDBvoCXT5eX7rU1frQLni5AKb",
				From:            "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
				To:              "91t4uSdtBiftqsB24W2fRXFCXjUyc6xY3WMGFedAaTHh",
				ToAlt:           "",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(120000000),
				Fee:             xc.NewAmountBlockchainFromUint64(5000),
				FeePayer:        "Hzn3n914JaSpnxo5mBbmuCDmGL6mxWN9Ac2HzEXFSGtb",
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
			// Multi sol + USDC transfer, with internal/inner instructions
			"2aoWC4uMT9zznC5m3ALE45Rb9YtmD6sNCabSgz4vACC9fKmwjaTooKVsn7mgoJ2Er3pa2xi8rbLcPzogNVwV5KSv",
			[]string{
				// getTransaction
				`{"blockTime":1744942223,"meta":{"computeUnitsConsumed":90548,"err":null,"fee":91404,"innerInstructions":[{"index":2,"instructions":[{"accounts":[0,7,1,8,9,3,13,14,11],"data":"2p2EsBa2hBpHob4XKiovKuRZ","programIdIndex":12,"stackHeight":2},{"accounts":[9,7,3,8],"data":"hZ4Wxjgxc7SnH","programIdIndex":14,"stackHeight":3},{"accounts":[3,4,1],"data":"3bWSwbMixZV1","programIdIndex":14,"stackHeight":2},{"accounts":[1,10],"data":"3Bxs4EVjZpa2tEN3","programIdIndex":11,"stackHeight":2}]}],"loadedAddresses":{"readonly":["11111111111111111111111111111111","BuuP1rJXnVs5GHSPoUxLqeQzV4nBXQ7RFAJ7j4rt6jEk","ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL","TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"],"writable":["EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","EM1GQEerfKsrT6eJbehX4nwV3MdTCdjvhUYVytFAuEVg","BT72p68Jp5eJUpuHThxSoeTRS9p4upTM8969X79GeQRF","3RY3ngufsn1aPSWE46Ga7sX5pZi2KPCvZG5uGS6TFLZJ"]},"logMessages":["Program ComputeBudget111111111111111111111111111111 invoke [1]","Program ComputeBudget111111111111111111111111111111 success","Program ComputeBudget111111111111111111111111111111 invoke [1]","Program ComputeBudget111111111111111111111111111111 success","Program E4CKSsnjU9WXzrBJpNXnFi4gbb1kmLuavwZHd35TLeHs invoke [1]","Program log: Instruction: HandleUserOperation","Program log: metadata nonce: 190","Program BuuP1rJXnVs5GHSPoUxLqeQzV4nBXQ7RFAJ7j4rt6jEk invoke [2]","Program log: Instruction: ReleaseToken","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA invoke [3]","Program log: Instruction: TransferChecked","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA consumed 6200 of 18887 compute units","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA success","Program data: 6OX/iGW9D9wBxvp6877brTo9ZfNqq8l0MbG75MLS9uDkfKYCA0UvXWFx7wABAAAAACy+4kgZ1zDU8Lj/pckqIVbYAxKJsxDYq4kr2VR6mMSn","Program BuuP1rJXnVs5GHSPoUxLqeQzV4nBXQ7RFAJ7j4rt6jEk consumed 25498 of 37443 compute units","Program BuuP1rJXnVs5GHSPoUxLqeQzV4nBXQ7RFAJ7j4rt6jEk success","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA invoke [2]","Program log: Instruction: Transfer","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA consumed 4644 of 10503 compute units","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA success","Program 11111111111111111111111111111111 invoke [2]","Program 11111111111111111111111111111111 success","Program data: WWNObeb8wOTl8wJD2bXLTm7unzZIvmtUR6Qf+JuCmjLYK4xrdIdSvPVlhGU97gRJA9E4yHeEhCdq660NvgAAAAAAAAA=","Program E4CKSsnjU9WXzrBJpNXnFi4gbb1kmLuavwZHd35TLeHs consumed 90248 of 93248 compute units","Program E4CKSsnjU9WXzrBJpNXnFi4gbb1kmLuavwZHd35TLeHs success"],"postBalances":[3746672245,69751950,1169280,2039280,2039280,1,1141440,389086612583,489594522939,2039280,208956596070,1,1141440,731913600,934087680],"postTokenBalances":[{"accountIndex":3,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"0","decimals":6,"uiAmount":null,"uiAmountString":"0"}},{"accountIndex":4,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"3CznQLcJWpyNKrYyq7qCvZsHn19csgB8g5wvisx4us47","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"4608152451","decimals":6,"uiAmount":4608.152451,"uiAmountString":"4608.152451"}},{"accountIndex":9,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"EM1GQEerfKsrT6eJbehX4nwV3MdTCdjvhUYVytFAuEVg","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"19346973797","decimals":6,"uiAmount":19346.973797,"uiAmountString":"19346.973797"}}],"preBalances":[3746763649,72212447,1169280,2039280,2039280,1,1141440,389086612583,489594522939,2039280,208954135573,1,1141440,731913600,934087680],"preTokenBalances":[{"accountIndex":3,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"4591313938","decimals":6,"uiAmount":4591.313938,"uiAmountString":"4591.313938"}},{"accountIndex":4,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"3CznQLcJWpyNKrYyq7qCvZsHn19csgB8g5wvisx4us47","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"0","decimals":6,"uiAmount":null,"uiAmountString":"0"}},{"accountIndex":9,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"EM1GQEerfKsrT6eJbehX4nwV3MdTCdjvhUYVytFAuEVg","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"19363812310","decimals":6,"uiAmount":19363.81231,"uiAmountString":"19363.81231"}}],"rewards":[],"status":{"Ok":null}},"slot":334182963,"transaction":["AU8ql0w1B0kio01+8JPiB74Q+HO7lq0j5PFH8m90GCEGJXimpXVAbA6MHKkZV62E/6T+L3iM8/SmDD8HcMcyuweAAQACBwBhh7VduskWvE6JTY7SlYqyUmDTjKK88V0EUQlzZTqcLL7iSBnXMNTwuP+lySohVtgDEomzENiriSvZVHqYxKcwsk15FBgYLcenkuzDYsoIjcDSZNfwzXm8mdZSRqrYQ8KBzJBVIxHYVbPIV53wT41zsQzpLgedCTskLlurqoFrAgzFuqeAKp8nlkEwrJfPJeoBYQhDwIDJq8Z0p/lbdu0DBkZv5SEXMv/srbpyw5vnvIzlu8X3EmssQ5s6QAAAAMH5eEXH4N6dclT1jQ+o9ERAGroPE7d81/2Phorhn6ue4Dyxuu57L+P52h9fC4kXx13sluxs3x8A/BATMsBJv9QDBQAFAmxtAQAFAAkD7xcOAAAAAAAGEAECAAsAAQMEBwgJCgwNDgugAlYQcC7UnMR79WWEZT3uBEkD0TjId4SEJ2rrrQ0BvgAAAAAAAAABAAAAAKUAAABwx2lBP7WwtACjxatbfTRR05uNupt2doStUC+Y2OeyiAIAAABncNHhAwCNffuwklT8vEoOmkDJnZht+PmPiosz3JjRA0lvSw/sxIbZk3Qh4/1rQgYyU/IV6V7HM6D+WFixrzkQsSu1JaY/OQo0Ic5uLYkytrpQ8kaAVIPiDeBnQi4gFPZ1ABWZ0TnYJz/vqfL4gyA4WWPhS8LbkAcUeR1CEp5LKxxBAAAAAwgJAAQBBQYCCQoLEQDAsA8sQ2tgjwFx7wABAAAAAAoDAgMBCQADg9OqEgEAAAALAgEHDAACAAAAUYslAAAAAADhtAFoAAAAAAGgfYr/iNHgu+9lVUAi8vOn85KVMwvJFF6xJ+NvO5aPIAQGC1YIBAAFAwI=","base64"],"version":0}`,
				// addressLookupTable
				`{"context":{"apiVersion":"2.1.18","slot":334289709},"value":{"data":["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","base64"],"executable":false,"lamports":24443520,"owner":"AddressLookupTab1e1111111111111111111111111","rentEpoch":18446744073709551615,"space":3384}}`,
				// getSlot
				`{"context":{"apiVersion":"2.1.18","slot":334289709},"value":{"blockhash":"2pSMZNzuseAqFXQkfcor1Ts6WvEojJ3uQm7hhe2415j6","lastValidBlockHeight":312525000}}`,
				// getTokenAccount
				`{"context":{"apiVersion":"2.1.18","slot":334289709},"value":{"data":{"parsed":{"info":{"isNative":false,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux","state":"initialized","tokenAmount":{"amount":"0","decimals":6,"uiAmount":0.0,"uiAmountString":"0"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":18446744073709551615,"space":165}}`,
				// getTokenAccount
				`{"context":{"apiVersion":"2.1.18","slot":334289709},"value":{"data":{"parsed":{"info":{"isNative":false,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"3CznQLcJWpyNKrYyq7qCvZsHn19csgB8g5wvisx4us47","state":"initialized","tokenAmount":{"amount":"0","decimals":6,"uiAmount":0.0,"uiAmountString":"0"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":18446744073709551615,"space":165}}`,
			},
			txinfo.LegacyTxInfo{
				TxID:            "2aoWC4uMT9zznC5m3ALE45Rb9YtmD6sNCabSgz4vACC9fKmwjaTooKVsn7mgoJ2Er3pa2xi8rbLcPzogNVwV5KSv",
				From:            "41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux",
				To:              "3RY3ngufsn1aPSWE46Ga7sX5pZi2KPCvZG5uGS6TFLZJ",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(2460497),
				Sources: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux",
						ContractAddress: "",
						Amount:          xc.NewAmountBlockchainFromUint64(2460497),
						Event:           txinfo.NewEvent("3.4", txinfo.MovementVariantNative),
					},
					{
						Address:         "EM1GQEerfKsrT6eJbehX4nwV3MdTCdjvhUYVytFAuEVg",
						ContractAddress: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
						Amount:          xc.NewAmountBlockchainFromUint64(16838513),
						Event:           txinfo.NewEvent("3.2", txinfo.MovementVariantNative),
					},
					{
						Address:         "41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux",
						ContractAddress: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
						Amount:          xc.NewAmountBlockchainFromUint64(4608152451),
						Event:           txinfo.NewEvent("3.3", txinfo.MovementVariantNative),
					},
				},
				Destinations: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "3RY3ngufsn1aPSWE46Ga7sX5pZi2KPCvZG5uGS6TFLZJ",
						ContractAddress: "",
						Amount:          xc.NewAmountBlockchainFromUint64(2460497),
						Event:           txinfo.NewEvent("3.4", txinfo.MovementVariantNative),
					},
					{
						Address:         "41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux",
						ContractAddress: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
						Amount:          xc.NewAmountBlockchainFromUint64(16838513),
						Event:           txinfo.NewEvent("3.2", txinfo.MovementVariantNative),
					},
					{
						Address:         "3CznQLcJWpyNKrYyq7qCvZsHn19csgB8g5wvisx4us47",
						ContractAddress: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
						Amount:          xc.NewAmountBlockchainFromUint64(4608152451),
						Event:           txinfo.NewEvent("3.3", txinfo.MovementVariantNative),
					},
				},
				Fee:           xc.NewAmountBlockchainFromUint64(91404),
				FeePayer:      "12VFrc1dFynPzs4HBRdRoMKNm1jca2S2uhaNKCkdffwy",
				BlockIndex:    334182963,
				BlockTime:     1744942223,
				Confirmations: 106746,
			},
			"",
		},
		{
			// Multi sol + USDC transfer, but one of the getTokenAccount fails
			"2aoWC4uMT9zznC5m3ALE45Rb9YtmD6sNCabSgz4vACC9fKmwjaTooKVsn7mgoJ2Er3pa2xi8rbLcPzogNVwV5KSv",
			[]string{
				// getTransaction
				`{"blockTime":1744942223,"meta":{"computeUnitsConsumed":90548,"err":null,"fee":91404,"innerInstructions":[{"index":2,"instructions":[{"accounts":[0,7,1,8,9,3,13,14,11],"data":"2p2EsBa2hBpHob4XKiovKuRZ","programIdIndex":12,"stackHeight":2},{"accounts":[9,7,3,8],"data":"hZ4Wxjgxc7SnH","programIdIndex":14,"stackHeight":3},{"accounts":[3,4,1],"data":"3bWSwbMixZV1","programIdIndex":14,"stackHeight":2},{"accounts":[1,10],"data":"3Bxs4EVjZpa2tEN3","programIdIndex":11,"stackHeight":2}]}],"loadedAddresses":{"readonly":["11111111111111111111111111111111","BuuP1rJXnVs5GHSPoUxLqeQzV4nBXQ7RFAJ7j4rt6jEk","ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL","TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"],"writable":["EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","EM1GQEerfKsrT6eJbehX4nwV3MdTCdjvhUYVytFAuEVg","BT72p68Jp5eJUpuHThxSoeTRS9p4upTM8969X79GeQRF","3RY3ngufsn1aPSWE46Ga7sX5pZi2KPCvZG5uGS6TFLZJ"]},"logMessages":["Program ComputeBudget111111111111111111111111111111 invoke [1]","Program ComputeBudget111111111111111111111111111111 success","Program ComputeBudget111111111111111111111111111111 invoke [1]","Program ComputeBudget111111111111111111111111111111 success","Program E4CKSsnjU9WXzrBJpNXnFi4gbb1kmLuavwZHd35TLeHs invoke [1]","Program log: Instruction: HandleUserOperation","Program log: metadata nonce: 190","Program BuuP1rJXnVs5GHSPoUxLqeQzV4nBXQ7RFAJ7j4rt6jEk invoke [2]","Program log: Instruction: ReleaseToken","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA invoke [3]","Program log: Instruction: TransferChecked","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA consumed 6200 of 18887 compute units","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA success","Program data: 6OX/iGW9D9wBxvp6877brTo9ZfNqq8l0MbG75MLS9uDkfKYCA0UvXWFx7wABAAAAACy+4kgZ1zDU8Lj/pckqIVbYAxKJsxDYq4kr2VR6mMSn","Program BuuP1rJXnVs5GHSPoUxLqeQzV4nBXQ7RFAJ7j4rt6jEk consumed 25498 of 37443 compute units","Program BuuP1rJXnVs5GHSPoUxLqeQzV4nBXQ7RFAJ7j4rt6jEk success","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA invoke [2]","Program log: Instruction: Transfer","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA consumed 4644 of 10503 compute units","Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA success","Program 11111111111111111111111111111111 invoke [2]","Program 11111111111111111111111111111111 success","Program data: WWNObeb8wOTl8wJD2bXLTm7unzZIvmtUR6Qf+JuCmjLYK4xrdIdSvPVlhGU97gRJA9E4yHeEhCdq660NvgAAAAAAAAA=","Program E4CKSsnjU9WXzrBJpNXnFi4gbb1kmLuavwZHd35TLeHs consumed 90248 of 93248 compute units","Program E4CKSsnjU9WXzrBJpNXnFi4gbb1kmLuavwZHd35TLeHs success"],"postBalances":[3746672245,69751950,1169280,2039280,2039280,1,1141440,389086612583,489594522939,2039280,208956596070,1,1141440,731913600,934087680],"postTokenBalances":[{"accountIndex":3,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"0","decimals":6,"uiAmount":null,"uiAmountString":"0"}},{"accountIndex":4,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"3CznQLcJWpyNKrYyq7qCvZsHn19csgB8g5wvisx4us47","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"4608152451","decimals":6,"uiAmount":4608.152451,"uiAmountString":"4608.152451"}},{"accountIndex":9,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"EM1GQEerfKsrT6eJbehX4nwV3MdTCdjvhUYVytFAuEVg","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"19346973797","decimals":6,"uiAmount":19346.973797,"uiAmountString":"19346.973797"}}],"preBalances":[3746763649,72212447,1169280,2039280,2039280,1,1141440,389086612583,489594522939,2039280,208954135573,1,1141440,731913600,934087680],"preTokenBalances":[{"accountIndex":3,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"4591313938","decimals":6,"uiAmount":4591.313938,"uiAmountString":"4591.313938"}},{"accountIndex":4,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"3CznQLcJWpyNKrYyq7qCvZsHn19csgB8g5wvisx4us47","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"0","decimals":6,"uiAmount":null,"uiAmountString":"0"}},{"accountIndex":9,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"EM1GQEerfKsrT6eJbehX4nwV3MdTCdjvhUYVytFAuEVg","programId":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","uiTokenAmount":{"amount":"19363812310","decimals":6,"uiAmount":19363.81231,"uiAmountString":"19363.81231"}}],"rewards":[],"status":{"Ok":null}},"slot":334182963,"transaction":["AU8ql0w1B0kio01+8JPiB74Q+HO7lq0j5PFH8m90GCEGJXimpXVAbA6MHKkZV62E/6T+L3iM8/SmDD8HcMcyuweAAQACBwBhh7VduskWvE6JTY7SlYqyUmDTjKK88V0EUQlzZTqcLL7iSBnXMNTwuP+lySohVtgDEomzENiriSvZVHqYxKcwsk15FBgYLcenkuzDYsoIjcDSZNfwzXm8mdZSRqrYQ8KBzJBVIxHYVbPIV53wT41zsQzpLgedCTskLlurqoFrAgzFuqeAKp8nlkEwrJfPJeoBYQhDwIDJq8Z0p/lbdu0DBkZv5SEXMv/srbpyw5vnvIzlu8X3EmssQ5s6QAAAAMH5eEXH4N6dclT1jQ+o9ERAGroPE7d81/2Phorhn6ue4Dyxuu57L+P52h9fC4kXx13sluxs3x8A/BATMsBJv9QDBQAFAmxtAQAFAAkD7xcOAAAAAAAGEAECAAsAAQMEBwgJCgwNDgugAlYQcC7UnMR79WWEZT3uBEkD0TjId4SEJ2rrrQ0BvgAAAAAAAAABAAAAAKUAAABwx2lBP7WwtACjxatbfTRR05uNupt2doStUC+Y2OeyiAIAAABncNHhAwCNffuwklT8vEoOmkDJnZht+PmPiosz3JjRA0lvSw/sxIbZk3Qh4/1rQgYyU/IV6V7HM6D+WFixrzkQsSu1JaY/OQo0Ic5uLYkytrpQ8kaAVIPiDeBnQi4gFPZ1ABWZ0TnYJz/vqfL4gyA4WWPhS8LbkAcUeR1CEp5LKxxBAAAAAwgJAAQBBQYCCQoLEQDAsA8sQ2tgjwFx7wABAAAAAAoDAgMBCQADg9OqEgEAAAALAgEHDAACAAAAUYslAAAAAADhtAFoAAAAAAGgfYr/iNHgu+9lVUAi8vOn85KVMwvJFF6xJ+NvO5aPIAQGC1YIBAAFAwI=","base64"],"version":0}`,
				// addressLookupTable
				`{"context":{"apiVersion":"2.1.18","slot":334289709},"value":{"data":["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","base64"],"executable":false,"lamports":24443520,"owner":"AddressLookupTab1e1111111111111111111111111","rentEpoch":18446744073709551615,"space":3384}}`,
				// getSlot
				`{"context":{"apiVersion":"2.1.18","slot":334289709},"value":{"blockhash":"2pSMZNzuseAqFXQkfcor1Ts6WvEojJ3uQm7hhe2415j6","lastValidBlockHeight":312525000}}`,
				// getTokenAccount
				`{"context":{"apiVersion":"2.1.18","slot":334289709},"value":{"data":{"parsed":{"info":{"isNative":false,"mint":"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v","owner":"41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux","state":"initialized","tokenAmount":{"amount":"0","decimals":6,"uiAmount":0.0,"uiAmountString":"0"}},"type":"account"},"program":"spl-token","space":165},"executable":false,"lamports":2039280,"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","rentEpoch":18446744073709551615,"space":165}}`,
				// getTokenAccount (failure)
				`{}`,
			},
			txinfo.LegacyTxInfo{
				TxID:            "2aoWC4uMT9zznC5m3ALE45Rb9YtmD6sNCabSgz4vACC9fKmwjaTooKVsn7mgoJ2Er3pa2xi8rbLcPzogNVwV5KSv",
				From:            "41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux",
				To:              "3RY3ngufsn1aPSWE46Ga7sX5pZi2KPCvZG5uGS6TFLZJ",
				ContractAddress: "",
				Amount:          xc.NewAmountBlockchainFromUint64(2460497),
				Sources: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux",
						ContractAddress: "",
						Amount:          xc.NewAmountBlockchainFromUint64(2460497),
						Event:           txinfo.NewEvent("3.4", txinfo.MovementVariantNative),
					},
					{
						Address:         "EM1GQEerfKsrT6eJbehX4nwV3MdTCdjvhUYVytFAuEVg",
						ContractAddress: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
						Amount:          xc.NewAmountBlockchainFromUint64(16838513),
						Event:           txinfo.NewEvent("3.2", txinfo.MovementVariantNative),
					},
				},
				Destinations: []*txinfo.LegacyTxInfoEndpoint{
					{
						Address:         "3RY3ngufsn1aPSWE46Ga7sX5pZi2KPCvZG5uGS6TFLZJ",
						ContractAddress: "",
						Amount:          xc.NewAmountBlockchainFromUint64(2460497),
						Event:           txinfo.NewEvent("3.4", txinfo.MovementVariantNative),
					},
					{
						Address:         "41fkvu8LhJDqkF325GTz6HuQRvZmCXVtYoYWPZeVCdux",
						ContractAddress: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
						Amount:          xc.NewAmountBlockchainFromUint64(16838513),
						Event:           txinfo.NewEvent("3.2", txinfo.MovementVariantNative),
					},
				},
				Fee:           xc.NewAmountBlockchainFromUint64(91404),
				FeePayer:      "12VFrc1dFynPzs4HBRdRoMKNm1jca2S2uhaNKCkdffwy",
				BlockIndex:    334182963,
				BlockTime:     1744942223,
				Confirmations: 106746,
			},
			"",
		},
		{
			"5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw",
			`{}`,
			txinfo.LegacyTxInfo{},
			"invalid transaction in response",
		},
		{
			"5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw",
			`null`,
			txinfo.LegacyTxInfo{},
			"TransactionNotFound: not found",
		},
		{
			"5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw",
			fmt.Errorf(`{"message": "custom RPC error", "code": 123}`),
			txinfo.LegacyTxInfo{},
			"custom RPC error",
		},
		{
			"",
			"",
			txinfo.LegacyTxInfo{},
			"zero length string",
		},
		{
			"invalid-sig",
			"",
			txinfo.LegacyTxInfo{},
			"invalid base58 digit",
		},
		{
			// 1 SOL
			"5U2YvvKUS6NUrDAJnABHjx2szwLCVmg8LCRK9BDbZwVAbf2q5j8D9Sc9kUoqanoqpn6ZpDguY3rip9W7N7vwCjSw",
			[]string{
				`{"blockTime":1650017168,"meta":{"err":null,"fee":5000,"innerInstructions":[],"loadedAddresses":{"readonly":[],"writable":[]},"logMessages":["Program 11111111111111111111111111111111 invoke [1]","Program 11111111111111111111111111111111 success"],"postBalances":[19921026477997237,1869985000,1],"postTokenBalances":[],"preBalances":[19921027478002237,869985000,1],"preTokenBalances":[],"rewards":[],"status":{"Ok":null}},"slot":128184605,"transaction":["invalid-binary","base64"]}`,
				`{"context":{"slot":128184606},"value":{"blockhash":"DvLEyV2GHk86K5GojpqnRsvhfMF5kdZomKMnhVpvHyqK","feeCalculator":{"lamportsPerSignature":5000}}}`,
			},
			txinfo.LegacyTxInfo{},
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
				require.Equal(t, txinfo.LegacyTxInfo{}, txInfo)
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, txInfo)
				testtypes.JsonPrint(txInfo)
				require.Equal(t, v.val, txInfo)
			}
		})
	}
}
