package client

import (
	"math/big"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestParseTempoFeePayment(t *testing.T) {
	token := common.HexToAddress("0x20c00000000000000000000014f22ca97301eb73")
	payer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	receiver := common.HexToAddress("0x2222222222222222222222222222222222222222")
	feeManager := common.HexToAddress(tempoFeeManagerAddress)

	receipt := &types.Receipt{
		Logs: []*types.Log{
			transferLog(6, token, payer, receiver, big.NewInt(99)),
			transferLog(7, token, payer, feeManager, big.NewInt(1234)),
		},
	}

	payment, ok := parseTempoFeePayment(receipt)
	require.True(t, ok)
	require.Equal(t, xc.Address(payer.String()), payment.Payer)
	require.Equal(t, xc.ContractAddress(token.String()), payment.Contract)
	require.Equal(t, xc.NewAmountBlockchainFromUint64(1234), payment.Amount)
	require.Contains(t, payment.EventIds, "7")
	require.NotContains(t, payment.EventIds, "6")
}

func TestParseTempoFeePaymentSumsMatchingTransfers(t *testing.T) {
	token := common.HexToAddress("0x20c00000000000000000000014f22ca97301eb73")
	payer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	feeManager := common.HexToAddress(tempoFeeManagerAddress)

	receipt := &types.Receipt{
		Logs: []*types.Log{
			transferLog(7, token, payer, feeManager, big.NewInt(1234)),
			transferLog(8, token, payer, feeManager, big.NewInt(56)),
		},
	}

	payment, ok := parseTempoFeePayment(receipt)
	require.True(t, ok)
	require.Equal(t, xc.NewAmountBlockchainFromUint64(1290), payment.Amount)
	require.Contains(t, payment.EventIds, "7")
	require.Contains(t, payment.EventIds, "8")
}

func TestRemoveTempoFeePayment(t *testing.T) {
	payment := &tempoFeePayment{
		EventIds: map[string]struct{}{
			"7": {},
		},
	}
	movements := tx.SourcesAndDests{
		Sources: []*txinfo.LegacyTxInfoEndpoint{
			{Address: "0x1111111111111111111111111111111111111111", Event: txinfo.NewEventFromIndex(7, txinfo.MovementVariantToken)},
			{Address: "0x2222222222222222222222222222222222222222", Event: txinfo.NewEventFromIndex(8, txinfo.MovementVariantToken)},
		},
		Destinations: []*txinfo.LegacyTxInfoEndpoint{
			{Address: "0xfeec000000000000000000000000000000000000", Event: txinfo.NewEventFromIndex(7, txinfo.MovementVariantToken)},
			{Address: "0x3333333333333333333333333333333333333333", Event: txinfo.NewEventFromIndex(8, txinfo.MovementVariantToken)},
		},
	}

	filtered := removeTempoFeePayment(movements, payment)
	require.Len(t, filtered.Sources, 1)
	require.Len(t, filtered.Destinations, 1)
	require.Equal(t, xc.Address("0x2222222222222222222222222222222222222222"), filtered.Sources[0].Address)
	require.Equal(t, xc.Address("0x3333333333333333333333333333333333333333"), filtered.Destinations[0].Address)
}

func transferLog(index uint, token common.Address, from common.Address, to common.Address, amount *big.Int) *types.Log {
	return &types.Log{
		Address: token,
		Topics: []common.Hash{
			ERC20.Events["Transfer"].ID,
			common.BytesToHash(common.LeftPadBytes(from.Bytes(), 32)),
			common.BytesToHash(common.LeftPadBytes(to.Bytes(), 32)),
		},
		Data:  common.LeftPadBytes(amount.Bytes(), 32),
		Index: index,
	}
}
