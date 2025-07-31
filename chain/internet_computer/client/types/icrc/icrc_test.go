package icrc_test

import (
	"testing"

	"github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid/idl"
	"github.com/cordialsys/crosschain/chain/internet_computer/client/types/icrc"
	"github.com/test-go/testify/require"
)

func TestIcrcTransferHash(t *testing.T) {
	expected := "418783abf0af99311178438612c6c73980d28b81daecb9daeea20c15cbbcc329"
	fee := idl.NewNat(uint64(10))
	createdAtTime := idl.NewNat(uint64(1_753_915_619_013_944_000))
	transaction := icrc.Transaction{
		Kind:      "transfer",
		Burn:      nil,
		Mint:      nil,
		Approve:   nil,
		Timestamp: idl.NewNat(uint64(1_753_915_619_583_774_080)),
		Transfer: &icrc.Transfer{
			To: icrc.Account{
				Owner: address.MustDecode("uai7x-izst2-xffip-2n2bm-wlyyc-rz6pu-agnjz-2sk2d-67hnl-sxfu5-7ae"),
			},
			From: icrc.Account{
				Owner: address.MustDecode("mglk4-25zez-he5uh-lsy2a-bontn-pfarj-offxd-5teb2-icnpp-scmni-zae"),
			},
			Fee:           &fee,
			Memo:          nil,
			CreatedAtTime: &createdAtTime,
			Amount:        idl.NewNat(uint64(20)),
			Spender:       nil,
		},
	}

	hash, err := transaction.ToFlattened().Hash()
	require.NoError(t, err)
	require.Equal(t, expected, hash)
}
