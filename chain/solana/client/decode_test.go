package client_test

import (
	"context"
	"testing"

	client "github.com/cordialsys/crosschain/chain/solana/client"
	soltx "github.com/cordialsys/crosschain/chain/solana/tx"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/stretchr/testify/require"
)

func TestResolveTokenAccountOwnerPrefersCachedOwner(t *testing.T) {
	account := solana.MustPublicKeyFromBase58("HLnpSz9h2S4hiLQ43rnSD9XkcUThA7B8hQMKmDaiTLcC")
	resolver := client.NewSolanaTokenAccountResolver(nil)
	resolver.Store(account, client.SolanaTokenAccount{Owner: "DdrKe4CYcQ72d1yEME7MMKhVhbQAERn6wWxrP9C5u2av"})

	got := client.ResolveTokenAccountOwner(context.Background(), resolver, account)
	want := "DdrKe4CYcQ72d1yEME7MMKhVhbQAERn6wWxrP9C5u2av"
	require.Equal(t, want, got)
}

func TestPreloadTemporaryTokenAccountsTracksAssociatedTokenCreate(t *testing.T) {
	// Solana DeFi transactions often create temporary tokens that will not
	// resolve using RPC, as they only exist for the instant of the transaction.
	// So now we parse initialize-account instructions to preload the token accounts resolutions.
	payer := solana.MustPublicKeyFromBase58("DdrKe4CYcQ72d1yEME7MMKhVhbQAERn6wWxrP9C5u2av")
	owner := solana.MustPublicKeyFromBase58("HLnpSz9h2S4hiLQ43rnSD9XkcUThA7B8hQMKmDaiTLcC")
	mint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	ata, _, err := solana.FindAssociatedTokenAddress(owner, mint)
	require.NoError(t, err)

	resolver := client.NewSolanaTokenAccountResolver(nil)

	client.PreloadTemporaryTokenAccounts(resolver, []soltx.ResolvedInstruction{
		{
			ProgramID: associatedtokenaccount.ProgramID,
			Accounts: []*solana.AccountMeta{
				solana.Meta(payer),
				solana.Meta(ata),
				solana.Meta(owner),
				solana.Meta(mint),
			},
		},
	})

	resolved, err := resolver.Resolve(context.Background(), ata)
	require.NoError(t, err)
	require.Equal(t, owner.String(), resolved.Owner)
	require.Equal(t, mint.String(), resolved.Mint)
}
