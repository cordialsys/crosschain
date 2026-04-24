package client

import (
	"context"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	soltx "github.com/cordialsys/crosschain/chain/solana/tx"
	"github.com/cordialsys/crosschain/chain/solana/types"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

type SolanaTokenAccount struct {
	Owner string
	Mint  string
}

type SolanaTokenAccountResolver struct {
	client *rpc.Client
	cache  map[string]SolanaTokenAccount
}

func NewSolanaTokenAccountResolver(client *rpc.Client) *SolanaTokenAccountResolver {
	return &SolanaTokenAccountResolver{
		client: client,
		cache:  map[string]SolanaTokenAccount{},
	}
}

func (r *SolanaTokenAccountResolver) Resolve(ctx context.Context, account solana.PublicKey) (SolanaTokenAccount, error) {
	if r == nil {
		return SolanaTokenAccount{}, fmt.Errorf("token account resolver is unavailable")
	}
	key := account.String()
	if cached, ok := r.cache[key]; ok {
		return cached, nil
	}
	if r.client == nil {
		return SolanaTokenAccount{}, fmt.Errorf("token account resolver is unavailable")
	}
	info, err := r.client.GetAccountInfoWithOpts(ctx, account, &rpc.GetAccountInfoOpts{
		Commitment: rpc.CommitmentFinalized,
		Encoding:   "jsonParsed",
	})
	if err != nil {
		return SolanaTokenAccount{}, err
	}
	accountInfo, err := types.ParseRpcData(info.Value.Data)
	if err != nil {
		return SolanaTokenAccount{}, err
	}
	resolved := SolanaTokenAccount{
		Owner: accountInfo.Parsed.Info.Owner,
		Mint:  accountInfo.Parsed.Info.Mint,
	}
	r.cache[key] = resolved
	return resolved, nil
}

func (r *SolanaTokenAccountResolver) Store(account solana.PublicKey, resolved SolanaTokenAccount) {
	if r == nil {
		return
	}
	if r.cache == nil {
		r.cache = map[string]SolanaTokenAccount{}
	}
	r.cache[account.String()] = resolved
}

func BuildTransfersFromDecoder(
	ctx context.Context,
	solClient *rpc.Client,
	decoder *soltx.Decoder,
	accountResolver *SolanaTokenAccountResolver,
	chain xc.NativeAsset,
) ([]*txinfo.LegacyTxInfoEndpoint, []*txinfo.LegacyTxInfoEndpoint, error) {
	if decoder == nil {
		return nil, nil, nil
	}
	if accountResolver == nil {
		accountResolver = NewSolanaTokenAccountResolver(solClient)
	}
	PreloadTemporaryTokenAccounts(accountResolver, decoder.GetResolvedInstructions())

	sources := []*txinfo.LegacyTxInfoEndpoint{}
	dests := []*txinfo.LegacyTxInfoEndpoint{}

	for _, instr := range decoder.GetSystemTransfers() {
		appendLegacyTransfer(
			&sources,
			&dests,
			xc.Address(instr.Instruction.GetFundingAccount().PublicKey.String()),
			xc.Address(ResolveTokenAccountOwner(ctx, accountResolver, instr.Instruction.GetRecipientAccount().PublicKey)),
			xc.NewAmountBlockchainFromUint64(*instr.Instruction.Lamports),
			"",
			txinfo.NewEvent(instr.ID, txinfo.MovementVariantNative),
		)
	}
	for _, instr := range decoder.GetVoteWithdraws() {
		appendLegacyTransfer(
			&sources,
			&dests,
			xc.Address(instr.Instruction.GetWithdrawAuthorityAccount().PublicKey.String()),
			xc.Address(ResolveTokenAccountOwner(ctx, accountResolver, instr.Instruction.GetRecipientAccount().PublicKey)),
			xc.NewAmountBlockchainFromUint64(*instr.Instruction.Lamports),
			"",
			txinfo.NewEvent(instr.ID, txinfo.MovementVariantNative),
		)
	}
	for _, instr := range decoder.GetStakeWithdraws() {
		appendLegacyTransfer(
			&sources,
			&dests,
			xc.Address(instr.Instruction.GetStakeAccount().PublicKey.String()),
			xc.Address(ResolveTokenAccountOwner(ctx, accountResolver, instr.Instruction.GetRecipientAccount().PublicKey)),
			xc.NewAmountBlockchainFromUint64(*instr.Instruction.Lamports),
			"",
			txinfo.NewEvent(instr.ID, txinfo.MovementVariantNative),
		)
	}
	for _, instr := range decoder.GetTokenTransferCheckeds() {
		appendLegacyTransfer(
			&sources,
			&dests,
			xc.Address(ResolveTokenAccountOwner(ctx, accountResolver, instr.Instruction.GetSourceAccount().PublicKey)),
			xc.Address(ResolveTokenAccountOwner(ctx, accountResolver, instr.Instruction.GetDestinationAccount().PublicKey)),
			xc.NewAmountBlockchainFromUint64(*instr.Instruction.Amount),
			xc.ContractAddress(instr.Instruction.GetMintAccount().PublicKey.String()),
			txinfo.NewEvent(instr.ID, txinfo.MovementVariantNative),
		)
	}
	for _, instr := range decoder.GetTokenTransferCheckedWithFee() {
		appendLegacyTransfer(
			&sources,
			&dests,
			xc.Address(ResolveTokenAccountOwner(ctx, accountResolver, instr.Instruction.GetSourceAccount().PublicKey)),
			xc.Address(ResolveTokenAccountOwner(ctx, accountResolver, instr.Instruction.GetDestinationAccount().PublicKey)),
			xc.NewAmountBlockchainFromUint64(*instr.Instruction.Amount),
			xc.ContractAddress(instr.Instruction.GetMintAccount().PublicKey.String()),
			txinfo.NewEvent(instr.ID, txinfo.MovementVariantNative),
		)
	}
	for _, instr := range decoder.GetTokenTransfers() {
		destResolved, destErr := accountResolver.Resolve(ctx, instr.Instruction.GetDestinationAccount().PublicKey)
		sourceResolved, sourceErr := accountResolver.Resolve(ctx, instr.Instruction.GetSourceAccount().PublicKey)
		if (destErr != nil || destResolved.Mint == "") && (sourceErr != nil || sourceResolved.Mint == "") {
			continue
		}

		from := instr.Instruction.GetOwnerAccount().PublicKey.String()
		if sourceErr == nil && sourceResolved.Owner != "" {
			from = sourceResolved.Owner
		}

		to := instr.Instruction.GetDestinationAccount().PublicKey.String()
		if destErr == nil && destResolved.Owner != "" {
			to = destResolved.Owner
		}

		contract := destResolved.Mint
		if contract == "" {
			contract = sourceResolved.Mint
		}

		appendLegacyTransfer(
			&sources,
			&dests,
			xc.Address(from),
			xc.Address(to),
			xc.NewAmountBlockchainFromUint64(*instr.Instruction.Amount),
			xc.ContractAddress(contract),
			txinfo.NewEvent(instr.ID, txinfo.MovementVariantNative),
		)
	}
	for _, instr := range decoder.GetTokenMintTo() {
		to := instr.Instruction.GetDestinationAccount().PublicKey.String()
		contract := xc.ContractAddress(instr.Instruction.GetAuthorityAccount().PublicKey.String())
		if resolved, err := accountResolver.Resolve(ctx, instr.Instruction.GetDestinationAccount().PublicKey); err == nil {
			if resolved.Owner != "" {
				to = resolved.Owner
			}
			if resolved.Mint != "" {
				contract = xc.ContractAddress(resolved.Mint)
			}
		}
		appendLegacyTransfer(
			&sources,
			&dests,
			xc.Address(instr.Instruction.GetAuthorityAccount().PublicKey.String()),
			xc.Address(to),
			xc.NewAmountBlockchainFromUint64(*instr.Instruction.Amount),
			contract,
			txinfo.NewEvent(instr.ID, txinfo.MovementVariantNative),
		)
	}
	for _, instr := range decoder.GetTokenMintToChecked() {
		to := instr.Instruction.GetDestinationAccount().PublicKey.String()
		contract := xc.ContractAddress(instr.Instruction.GetAuthorityAccount().PublicKey.String())
		if resolved, err := accountResolver.Resolve(ctx, instr.Instruction.GetDestinationAccount().PublicKey); err == nil {
			if resolved.Owner != "" {
				to = resolved.Owner
			}
			if resolved.Mint != "" {
				contract = xc.ContractAddress(resolved.Mint)
			}
		}
		appendLegacyTransfer(
			&sources,
			&dests,
			xc.Address(instr.Instruction.GetAuthorityAccount().PublicKey.String()),
			xc.Address(to),
			xc.NewAmountBlockchainFromUint64(*instr.Instruction.Amount),
			contract,
			txinfo.NewEvent(instr.ID, txinfo.MovementVariantNative),
		)
	}
	if solClient != nil {
		for _, instr := range decoder.GetCloseTokenAccounts() {
			lamports, err := solClient.GetMinimumBalanceForRentExemption(ctx, 165, rpc.CommitmentFinalized)
			if err != nil {
				continue
			}
			appendLegacyTransfer(
				&sources,
				&dests,
				xc.Address(instr.Instruction.GetOwnerAccount().PublicKey.String()),
				xc.Address(instr.Instruction.GetDestinationAccount().PublicKey.String()),
				xc.NewAmountBlockchainFromUint64(lamports),
				"",
				txinfo.NewEvent(instr.ID, txinfo.MovementVariantNative),
			)
		}
	}
	for _, instr := range decoder.GetCustomSystemIntentTransfers() {
		appendLegacyTransfer(
			&sources,
			&dests,
			xc.Address(instr.Instruction.GetFromAccount().PublicKey.String()),
			xc.Address(ResolveTokenAccountOwner(ctx, accountResolver, instr.Instruction.GetToAccount().PublicKey)),
			xc.NewAmountBlockchainFromUint64(*instr.Instruction.Lamports),
			"",
			txinfo.NewEvent(instr.ID, txinfo.MovementVariantNative),
		)
	}

	return sources, dests, nil
}

func appendLegacyTransfer(
	sources *[]*txinfo.LegacyTxInfoEndpoint,
	dests *[]*txinfo.LegacyTxInfoEndpoint,
	from xc.Address,
	to xc.Address,
	amount xc.AmountBlockchain,
	contract xc.ContractAddress,
	event *txinfo.Event,
) {
	*sources = append(*sources, &txinfo.LegacyTxInfoEndpoint{
		Address:         from,
		Amount:          amount,
		ContractAddress: contract,
		Event:           event,
	})
	*dests = append(*dests, &txinfo.LegacyTxInfoEndpoint{
		Address:         to,
		Amount:          amount,
		ContractAddress: contract,
		Event:           event,
	})
}

func PreloadTemporaryTokenAccounts(resolver *SolanaTokenAccountResolver, instructions []soltx.ResolvedInstruction) {
	if resolver == nil {
		return
	}
	for _, instruction := range instructions {
		switch {
		case instruction.ProgramID.Equals(associatedtokenaccount.ProgramID):
			if len(instruction.Accounts) < 4 {
				continue
			}
			resolver.cache[instruction.Accounts[1].PublicKey.String()] = SolanaTokenAccount{
				Owner: instruction.Accounts[2].PublicKey.String(),
				Mint:  instruction.Accounts[3].PublicKey.String(),
			}
		case instruction.ProgramID.Equals(solana.TokenProgramID), instruction.ProgramID.Equals(solana.Token2022ProgramID):
			decoded, err := token.DecodeInstruction(instruction.Accounts, instruction.Data)
			if err != nil || decoded == nil {
				continue
			}
			switch inst := decoded.Impl.(type) {
			case *token.InitializeAccount:
				cacheTokenAccount(resolver, inst.GetAccount(), inst.GetOwnerAccount(), inst.GetMintAccount())
			case *token.InitializeAccount2:
				cacheTokenAccountWithOwnerValue(resolver, inst.GetAccount(), inst.Owner, inst.GetMintAccount())
			case *token.InitializeAccount3:
				cacheTokenAccountWithOwnerValue(resolver, inst.GetAccount(), inst.Owner, inst.GetMintAccount())
			}
		}
	}
}

func cacheTokenAccount(resolver *SolanaTokenAccountResolver, account, owner, mint *solana.AccountMeta) {
	if resolver == nil || account == nil || owner == nil || mint == nil {
		return
	}
	resolver.cache[account.PublicKey.String()] = SolanaTokenAccount{
		Owner: owner.PublicKey.String(),
		Mint:  mint.PublicKey.String(),
	}
}

func cacheTokenAccountWithOwnerValue(resolver *SolanaTokenAccountResolver, account *solana.AccountMeta, owner *solana.PublicKey, mint *solana.AccountMeta) {
	if resolver == nil || account == nil || owner == nil || mint == nil {
		return
	}
	resolver.cache[account.PublicKey.String()] = SolanaTokenAccount{
		Owner: owner.String(),
		Mint:  mint.PublicKey.String(),
	}
}

func ResolveTokenAccountOwner(ctx context.Context, resolver *SolanaTokenAccountResolver, account solana.PublicKey) string {
	fallback := account.String()
	if resolver == nil {
		return fallback
	}
	resolved, err := resolver.Resolve(ctx, account)
	if err != nil || resolved.Owner == "" {
		return fallback
	}
	return resolved.Owner
}
