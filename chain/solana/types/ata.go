package types

import "github.com/gagliardetto/solana-go"

// FindAssociatedTokenAddress returns the associated token account (ATA) for a given account and token
func FindAssociatedTokenAddress(addr string, contract string, tokenProgram solana.PublicKey) (string, error) {
	address, err := solana.PublicKeyFromBase58(addr)
	if err != nil {
		return "", err
	}
	mint, err := solana.PublicKeyFromBase58(contract)
	if err != nil {
		return "", err
	}
	if len(tokenProgram) == 0 || tokenProgram.IsZero() {
		tokenProgram = solana.TokenProgramID
	}
	associatedAddr, _, err := solana.FindProgramAddress(
		[][]byte{
			address[:],
			tokenProgram[:],
			mint[:],
		},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	if err != nil {
		return "", err
	}
	return associatedAddr.String(), nil
}
