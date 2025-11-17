package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/kaspa/builder"
	"github.com/cordialsys/crosschain/chain/kaspa/client/rest"
	"github.com/cordialsys/crosschain/chain/kaspa/tx"
	"github.com/cordialsys/crosschain/chain/kaspa/tx_input"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/client/errors"
	txinfo "github.com/cordialsys/crosschain/client/tx_info"
	xctypes "github.com/cordialsys/crosschain/client/types"
	"github.com/cordialsys/crosschain/testutil"
	"github.com/kaspanet/kaspad/util/txmass"
	"github.com/sirupsen/logrus"
)

// Client for Template
type Client struct {
	chain    xc.NativeAsset
	chainCfg *xc.ChainConfig
	client   *rest.Client
	decimals int
}

const (
	defaultMassPerTxByte           = 1
	defaultMassPerScriptPubKeyByte = 10
	defaultMassPerSigOp            = 1000
)

var _ xclient.Client = &Client{}

func derefOrZero[T any](v *T) T {
	if v == nil {
		var zero T
		return zero
	}
	return *v
}

// TODO https://github.com/kaspanet/rusty-kaspa/blob/master/rpc/core/src/api/rpc.rs
func NewClient(cfgI xc.ITask) (*Client, error) {
	chain := cfgI.GetChain()
	clientConfig := chain.ChainClientConfig
	client := rest.NewClient(clientConfig.URL, chain.Chain, chain.DefaultHttpClient())
	return &Client{chain.Chain, chain, client, int(chain.Decimals)}, nil
}

func (c *Client) FetchTransferInput(ctx context.Context, args xcbuilder.TransferArgs) (xc.TxInput, error) {
	utxos, err := c.client.GetUtxos([]string{string(args.GetFrom())})
	if err != nil {
		return nil, err
	}
	txInput := tx_input.NewTxInput()
	txInput.Address = args.GetFrom()
	for _, utxo := range utxos {
		if utxo.UtxoEntry.Amount == nil {
			// skip?
			continue
		}
		txInput.Utxos = append(txInput.Utxos, tx_input.Utxo{
			TransactionId: *utxo.Outpoint.TransactionId,
			Index:         *utxo.Outpoint.Index,
			Amount:        xc.NewAmountBlockchainFromStr(*utxo.UtxoEntry.Amount),
		})
	}

	feeRates, err := c.client.GetFeeEstimate()
	if err != nil {
		return nil, err
	}

	txBuilder, err := builder.NewTxBuilder(c.chainCfg.Base())
	if err != nil {
		return nil, err
	}
	txI, err := txBuilder.Transfer(args, txInput)
	if err != nil {
		return nil, err
	}
	kaspaTx := txI.(*tx.Tx)
	domainTransaction, err := kaspaTx.BuildUnsignedDomainTransaction()
	if err != nil {
		return nil, err
	}

	txMassCalculator := txmass.NewCalculator(defaultMassPerTxByte, defaultMassPerScriptPubKeyByte, defaultMassPerSigOp)
	transactionMass := txMassCalculator.CalculateTransactionMass(domainTransaction)

	txInput.Mass = transactionMass
	if c.chainCfg.ChainGasMultiplier > 0.01 {
		txInput.Mass = uint64(float64(transactionMass) * c.chainCfg.ChainGasMultiplier)
	}
	txInput.FeePerGram = feeRates.GetMostNormalFeeEstimate()
	txInput.MinFee = c.chainCfg.GasBudgetMinimum.ToBlockchain(int32(c.chainCfg.Decimals))

	return txInput, nil
}

func (c *Client) FetchLegacyTxInput(ctx context.Context, from xc.Address, to xc.Address) (xc.TxInput, error) {
	chainCfg := c.chainCfg.Base()
	args, _ := xcbuilder.NewTransferArgs(chainCfg, from, to, xc.NewAmountBlockchainFromUint64(1))
	return c.FetchTransferInput(ctx, args)
}

func (c *Client) SubmitTx(ctx context.Context, txInput xctypes.SubmitTxReq) error {
	serializedSigned, err := txInput.Serialize()
	if err != nil {
		return err
	}
	response, err := c.client.SubmitTransaction(serializedSigned)
	if err != nil {
		if strings.Contains(err.Error(), "larger than max allowed size") {
			return fmt.Errorf("%s: you may be sending a transaction amount that is too small", err.Error())
		}
		return err
	}
	logrus.WithFields(logrus.Fields{
		"txid": *response.TransactionId,
	}).Debug("submitted kaspa tx")
	return nil
}

func (c *Client) FetchLegacyTxInfo(ctx context.Context, txHash xc.TxHash) (txinfo.LegacyTxInfo, error) {
	return txinfo.LegacyTxInfo{}, fmt.Errorf("not implemented")
}

func (c *Client) FetchTxInfo(ctx context.Context, args *txinfo.Args) (txinfo.TxInfo, error) {
	txHash := args.TxHash()
	tx, err := c.client.GetTransaction(string(txHash))
	if apiErr, ok := err.(*rest.ErrorResponse); ok {
		if apiErr.Code == 404 {
			return txinfo.TxInfo{}, errors.TransactionNotFoundf("%s", apiErr.Error())
		}
	}
	if err != nil {
		return txinfo.TxInfo{}, err
	}

	lastestBlueScore, err := c.client.GetVirtualChainBlueScore()
	if err != nil {
		return txinfo.TxInfo{}, err
	}

	movement := txinfo.Movement{
		AssetId: xc.ContractAddress(c.chain),
	}

	for _, input := range derefOrZero(tx.Inputs) {
		movement.From = append(movement.From, txinfo.NewBalanceChange(
			c.chain,
			xc.Address(*input.PreviousOutpointAddress),
			xc.NewAmountBlockchainFromUint64(uint64(*input.PreviousOutpointAmount)),
			nil,
		))
	}

	for _, output := range derefOrZero(tx.Outputs) {
		movement.To = append(movement.To, txinfo.NewBalanceChange(
			c.chain,
			xc.Address(*output.ScriptPublicKeyAddress),
			xc.NewAmountBlockchainFromUint64(uint64(output.Amount)),
			nil,
		))
	}
	testutil.JsonPrint(tx)
	confirmations := uint64(0)
	height := 0
	hashMaybe := ""
	if tx.AcceptingBlockBlueScore != nil && *tx.AcceptingBlockBlueScore != 0 {
		confirmations = uint64(*lastestBlueScore.BlueScore) - uint64(*tx.AcceptingBlockBlueScore)
		height = *tx.AcceptingBlockBlueScore
	}
	if tx.BlockHash != nil && len(*tx.BlockHash) > 0 {
		hashMaybe = (*tx.BlockHash)[0]
	}
	txId := string(txHash)
	if tx.TransactionId != nil {
		txId = *tx.TransactionId
	}

	txInfo := txinfo.NewTxInfo(&txinfo.Block{
		Chain: c.chain,
		// The "blue score" is basically the block height.
		// Although it seems there could be multiple "blocks" at a given height?
		Height: xc.NewAmountBlockchainFromUint64(uint64(height)),
		Hash:   hashMaybe,
		Time: time.Unix(
			int64((time.Duration(*tx.BlockTime) * time.Millisecond).Seconds()), 0),
	},
		c.chainCfg,
		txId,
		confirmations,
		nil,
	)
	txInfo.AddMovement(&movement)

	// The fee is the natural difference of (inputs - outputs)
	txInfo.Fees = txInfo.CalculateFees()

	return *txInfo, nil
}

func (c *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	utxos, err := c.client.GetUtxos([]string{string(args.Address())})
	if err != nil {
		return xc.AmountBlockchain{}, err
	}
	total := xc.AmountBlockchain{}
	for _, utxo := range utxos {
		if utxo.UtxoEntry.Amount != nil {
			amount := xc.NewAmountBlockchainFromStr(*utxo.UtxoEntry.Amount)
			total = total.Add(&amount)
		}
	}
	return total, nil
}

func (c *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	// no tokens on kaspa currently
	if contract != "" && contract != xc.ContractAddress(c.chain) {
		return 0, fmt.Errorf("no tokens on kaspa currently")
	}
	return c.decimals, nil
}
func (c *Client) FetchBlock(ctx context.Context, args *xclient.BlockArgs) (*txinfo.BlockWithTransactions, error) {
	var blocks []*rest.BlockModel
	var err error
	height, ok := args.Height()
	if ok {
		blocks, err = c.client.GetBlocksFromBlockScore(height)
		if err != nil {
			return nil, err
		}
	} else {
		// attempt to scan for the latest block
		// Kaspa chain head is pretty flaky, scanning is needed to be reliable here
		latestBlueScore, err := c.client.GetVirtualChainBlueScore()
		if err != nil {
			return nil, err
		}
		height = uint64(*latestBlueScore.BlueScore)
		for i := range 10 {
			blocks, err = c.client.GetBlocksFromBlockScore(height - uint64(i*10))
			if err != nil {
				return nil, err
			}
			if len(blocks) > 0 {
				break
			}
		}
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("no block found")
	}
	firstBlock := blocks[0]

	res := &txinfo.BlockWithTransactions{
		Block: txinfo.Block{
			Chain:  c.chain,
			Height: xc.NewAmountBlockchainFromUint64(height),
			Hash:   *firstBlock.VerboseData.Hash,
			Time: time.Unix(
				int64((time.Duration(xc.NewAmountBlockchainFromStr(*firstBlock.Header.Timestamp).Uint64()) * time.Millisecond).Seconds()), 0),
		},
	}

	transactions := make(map[string]bool)

	// Just merge all of the blocks for our response
	for _, block := range blocks {
		for _, tx := range derefOrZero(block.Transactions) {
			txId := tx.VerboseData.TransactionId
			// sometimes the same block shows up twice and can result in dup tx's
			if _, ok := transactions[txId]; !ok {
				transactions[txId] = true
				res.TransactionIds = append(res.TransactionIds, txId)
			}
		}
	}

	return res, nil
}
