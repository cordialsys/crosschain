package client

import (
	"context"
	"encoding/binary"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	solanacall "github.com/cordialsys/crosschain/chain/solana/call"
	"github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

func (client *Client) FetchCallInput(ctx context.Context, call xc.TxCall, args builder.CallArgs) (xc.CallTxInput, error) {
	var nonceAccountMaybe *solana.PublicKey
	nonceAccount, ok := args.GetNonceAccount()
	if ok {
		nonceAccountPub, err := solana.PublicKeyFromBase58(nonceAccount)
		if err != nil {
			return nil, fmt.Errorf("invalid nonce account: %s: %v", nonceAccount, err)
		}
		nonceAccountMaybe = &nonceAccountPub
	}

	if solCall, ok := call.(*solanacall.TxCall); ok {
		input, usesDurableNonce, err := client.fetchCallDurableNonceInput(ctx, solCall.SolTx, nonceAccountMaybe)
		if err != nil {
			return nil, err
		}
		if usesDurableNonce {
			return input, nil
		}
	}

	fromAddr := call.SigningAddresses()[0]
	txInput, err := client.FetchBaseInput(ctx, fromAddr, "", xc.NewAmountBlockchainFromUint64(0), nonceAccountMaybe)
	if err != nil {
		return nil, err
	}
	return &tx_input.CallInput{TxInput: *txInput}, nil
}

// A supplied nonce transaction may have an external authority. Validate the
// authority in its advance instruction instead of assuming it is our signer.
// The bool reports whether the first instruction advances a durable nonce.
func (client *Client) fetchCallDurableNonceInput(ctx context.Context, solTx *solana.Transaction, requestedNonce *solana.PublicKey) (*tx_input.CallInput, bool, error) {
	if len(solTx.Message.Instructions) == 0 {
		return nil, false, nil
	}
	instruction := solTx.Message.Instructions[0]
	program, err := solTx.Message.ResolveProgramIDIndex(instruction.ProgramIDIndex)
	if err != nil {
		return nil, false, err
	}
	if program != solana.SystemProgramID || len(instruction.Data) != 4 || binary.LittleEndian.Uint32(instruction.Data) != system.Instruction_AdvanceNonceAccount {
		return nil, false, nil
	}
	if len(instruction.Accounts) < 3 {
		return nil, true, fmt.Errorf("advance nonce instruction requires nonce, recent blockhashes, and authority accounts")
	}
	nonceAccount, err := solTx.Message.Account(instruction.Accounts[0])
	if err != nil {
		return nil, true, fmt.Errorf("invalid nonce account: %w", err)
	}
	authority, err := solTx.Message.Account(instruction.Accounts[2])
	if err != nil {
		return nil, true, fmt.Errorf("invalid nonce authority: %w", err)
	}
	if instruction.Accounts[2] >= uint16(solTx.Message.Header.NumRequiredSignatures) {
		return nil, true, fmt.Errorf("nonce authority %s is not a transaction signer", authority)
	}
	if requestedNonce != nil && *requestedNonce != nonceAccount {
		return nil, true, fmt.Errorf("nonce account %s does not match advance nonce instruction account %s", requestedNonce, nonceAccount)
	}

	// This transaction already advances an existing account; a failed lookup
	// must not fall back to marking the nonce account for creation.
	state, err := client.FetchNonceAccount(ctx, nonceAccount)
	if err != nil {
		return nil, true, fmt.Errorf("could not fetch durable nonce: %w", err)
	}
	if state.AuthorizedPubkey != authority {
		return nil, true, fmt.Errorf("nonce account %s is authorized by %s, which does not match instruction authority %s", nonceAccount, state.AuthorizedPubkey, authority)
	}
	input := tx_input.NewTxInput()
	input.RecentBlockHash = solTx.Message.RecentBlockhash
	input.BaseFee = xc.NewAmountBlockchainFromUint64(baseFeeLamports)
	input.FeePayerBaseFee = xc.NewAmountBlockchainFromUint64(baseFeeLamports)
	input.DurableNonceAccount = nonceAccount
	input.DurableNonceAuthority = authority
	input.DurableNonce = state.Nonce
	return &tx_input.CallInput{TxInput: *input}, true, nil
}
