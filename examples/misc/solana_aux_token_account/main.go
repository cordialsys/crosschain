package main

// This creates a new token account and sends tokens to it

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"

	"github.com/cordialsys/crosschain/factory/signer"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

func sendToAuxAccount(mint string, from string, to string, amount uint64, seed string) error {
	solClient := rpc.New("https://api.mainnet-beta.solana.com")
	fromOwnerAddress := from
	toOwnerAddress := to
	tokenAddress := mint

	key := signer.ReadPrivateKeyEnv()
	if key == "" {
		return fmt.Errorf("must set env %s to base58 private key", signer.EnvPrivateKey)
	}

	signer, err := solana.WalletFromPrivateKeyBase58(key)
	if err != nil {
		return err
	}

	out, err := solClient.GetFees(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return err
	}
	feeLamports := out.Value.FeeCalculator.LamportsPerSignature
	rentFree, err := solClient.GetMinimumBalanceForRentExemption(context.Background(), 165, rpc.CommitmentFinalized)
	if err != nil {
		return err
	}
	fmt.Println("fees ", feeLamports, rentFree)

	fromOwner, err := solana.PublicKeyFromBase58(string(fromOwnerAddress))
	if err != nil {
		return err
	}
	toOwner, err := solana.PublicKeyFromBase58(string(toOwnerAddress))
	if err != nil {
		return err
	}
	tokenAccount, err := solana.PublicKeyFromBase58(string(tokenAddress))
	if err != nil {
		return err
	}
	// fromAtaAccount, err := solana.PublicKeyFromBase58(string(fromAtaAddress))
	// if err != nil {
	// 	return err
	// }
	fromAtaAccount, _, err := solana.FindAssociatedTokenAddress(fromOwner, tokenAccount)
	if err != nil {
		return err
	}

	// wallet := solana.NewWallet()

	newAccount, err := solana.CreateWithSeed(fromOwner, seed, solana.TokenProgramID)
	if err != nil {
		return err
	}
	createAccountBuilder := system.NewCreateAccountWithSeedInstructionBuilder()
	// createAccountBuilder.SetBase(solana.TokenProgramID)
	createAccountBuilder.SetBase(fromOwner)
	createAccountBuilder.SetFundingAccount(fromOwner)
	createAccountBuilder.SetLamports(rentFree)
	createAccountBuilder.SetSpace(165)
	createAccountBuilder.SetCreatedAccount(newAccount)
	createAccountBuilder.SetBaseAccount(fromOwner)
	createAccountBuilder.SetSeed(seed)
	createAccountBuilder.SetOwner(solana.TokenProgramID)
	err = createAccountBuilder.Validate()
	if err != nil {
		return err
	}
	fmt.Println(createAccountBuilder.Build().ProgramID().String())
	_ = toOwner

	initTokenAccount := token.NewInitializeAccount2Instruction(
		toOwner,
		newAccount,
		tokenAccount,
		// fromOwner,
		solana.SysVarRentPubkey,
	)
	err = initTokenAccount.Validate()
	if err != nil {
		return err
	}

	transferToken := token.NewTransferCheckedInstruction(
		amount,
		6,
		fromAtaAccount,
		tokenAccount,
		newAccount,
		fromOwner,
		[]solana.PublicKey{
			fromOwner,
		},
	)
	err = transferToken.Validate()
	if err != nil {
		return err
	}

	instructions := []solana.Instruction{
		createAccountBuilder.Build(),
		initTokenAccount.Build(),
		transferToken.Build(),
	}
	recentHash, err := solClient.GetRecentBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return err
	}
	tx, err := solana.NewTransaction(
		instructions,
		recentHash.Value.Blockhash,
		solana.TransactionPayer(fromOwner),
	)
	if err != nil {
		return err
	}
	toSign, err := tx.Message.MarshalBinary()
	if err != nil {
		return err
	}
	sig, err := signer.PrivateKey.Sign(toSign)
	if err != nil {
		return err
	}
	tx.Signatures = []solana.Signature{sig}
	txData, err := tx.MarshalBinary()
	if err != nil {
		return fmt.Errorf("send transaction: encode transaction: %w", err)
	}
	fmt.Println("sending tx...")
	res, err := solClient.SendEncodedTransactionWithOpts(
		context.Background(),
		base64.StdEncoding.EncodeToString(txData),
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentFinalized,
		},
	)
	fmt.Println("signature: ", res.String())
	return err
	// .MarshalWithEncoder(
	// 0,
	// 165 bytes
	// 165,
	// toOwner,
	// fromOwner,
	// wallet.PublicKey(),
	// )
}
func main() {
	if len(os.Args) != 6 {
		fmt.Printf(`usage:
	%s <mint> <from> <to> <amount-chain> <seed-for-aux-account>\n`, os.Args[0])
	}
	mint := os.Args[1]
	from := os.Args[2]
	to := os.Args[3]
	amt := os.Args[4]
	seed := os.Args[5]

	amount, err := strconv.Atoi(amt)
	if err != nil {
		panic(err)
	}

	err = sendToAuxAccount(mint, from, to, uint64(amount), seed)
	if err != nil {
		panic("failed: " + err.Error())
	}
	fmt.Println("done")
}
