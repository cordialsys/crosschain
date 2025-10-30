package httpclient

import (
	xc "github.com/cordialsys/crosschain"
)

const (
	METHOD_CREATE_TRANSACTION   = "createtransaction"
	METHOD_FREEZE_BALANCEV2     = "freezebalancev2"
	METHOD_UNFREEZE_BALANCEV2   = "unfreezebalancev2"
	METHOD_WITHDRAW_BALANCE     = "withdrawbalance"
	METHOD_WITHDRAW_UNFREEZE    = "withdrawexpireunfreeze"
	METHOD_VOTE_WITNESS_ACCOUNT = "votewitnessaccount"
	RESOURCE_BANDWIDTH          = "BANDWIDTH"
)

type CreateInputParams interface {
	ToMap() map[string]any
	Method() string
}

type CreateTransactionParams struct {
	From   xc.Address
	To     xc.Address
	Amount xc.AmountBlockchain
}

var _ CreateInputParams = CreateTransactionParams{}

func (t CreateTransactionParams) ToMap() map[string]any {
	return map[string]any{
		"owner_address": string(t.From),
		"to_address":    string(t.To),
		"amount":        t.Amount.Uint64(),
		"visible":       true,
	}
}

func (t CreateTransactionParams) Method() string {
	return METHOD_CREATE_TRANSACTION
}

type FreezeBalanceV2Params struct {
	Owner         xc.Address
	FrozenBalance xc.AmountBlockchain
}

var _ CreateInputParams = FreezeBalanceV2Params{}

func (f FreezeBalanceV2Params) ToMap() map[string]any {
	return map[string]any{
		"owner_address":  string(f.Owner),
		"frozen_balance": f.FrozenBalance.Uint64(),
		"resource":       RESOURCE_BANDWIDTH,
		"visible":        true,
	}
}

func (t FreezeBalanceV2Params) Method() string {
	return METHOD_FREEZE_BALANCEV2
}

var _ CreateInputParams = UnfreezeBalanceV2Params{}

type UnfreezeBalanceV2Params struct {
	Owner           xc.Address
	UnfreezeBalance xc.AmountBlockchain
}

func (u UnfreezeBalanceV2Params) ToMap() map[string]any {
	return map[string]any{
		"owner_address":    u.Owner,
		"unfreeze_balance": u.UnfreezeBalance.Uint64(),
		"resource":         RESOURCE_BANDWIDTH,
		"visible":          true,
	}
}

func (t UnfreezeBalanceV2Params) Method() string {
	return METHOD_UNFREEZE_BALANCEV2
}

type VoteWitnessAccountParams struct {
	Owner xc.Address
	Votes []*Vote
}

var _ CreateInputParams = VoteWitnessAccountParams{}

func (v VoteWitnessAccountParams) ToMap() map[string]any {
	votes := make([]map[string]any, 0)
	for _, v := range v.Votes {
		votes = append(votes, map[string]any{
			"vote_address": v.VoteAddress,
			"vote_count":   v.VoteCount,
		})
	}

	return map[string]any{
		"owner_address": v.Owner,
		"votes":         votes,
		"visible":       true,
	}
}

func (t VoteWitnessAccountParams) Method() string {
	return METHOD_VOTE_WITNESS_ACCOUNT
}

type WithdrawExpiredUnfreezeParams struct {
	Owner xc.Address
}

var _ CreateInputParams = WithdrawExpiredUnfreezeParams{}

func (w WithdrawExpiredUnfreezeParams) ToMap() map[string]any {
	return map[string]any{
		"owner_address": w.Owner,
		"visible":       true,
	}
}

func (w WithdrawExpiredUnfreezeParams) Method() string {
	return METHOD_WITHDRAW_UNFREEZE
}

type WithdrawBalanceParams struct {
	Owner xc.Address
}

var _ CreateInputParams = WithdrawExpiredUnfreezeParams{}

func (w WithdrawBalanceParams) ToMap() map[string]any {
	return map[string]any{
		"owner_address": w.Owner,
		"visible":       true,
	}
}

func (w WithdrawBalanceParams) Method() string {
	return METHOD_WITHDRAW_BALANCE
}
