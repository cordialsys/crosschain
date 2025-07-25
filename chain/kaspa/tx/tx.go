package tx

import (
	"encoding/hex"
	"errors"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/kaspa/tx/txscript"
	"github.com/cordialsys/crosschain/chain/kaspa/tx_input"
	"github.com/kaspanet/kaspad/domain/consensus/model/externalapi"
	"github.com/kaspanet/kaspad/domain/consensus/utils/consensushashing"
	"github.com/kaspanet/kaspad/domain/consensus/utils/constants"
	"github.com/kaspanet/kaspad/domain/consensus/utils/subnetworks"
	"github.com/kaspanet/kaspad/domain/consensus/utils/transactionid"

	// "github.com/kaspanet/kaspad/domain/consensus/utils/txscript"
	"github.com/kaspanet/kaspad/domain/consensus/utils/utxo"
	"github.com/kaspanet/kaspad/util"
)

type Tx struct {
	args                    xcbuilder.TransferArgs
	input                   *tx_input.TxInput
	chainPrefix             util.Bech32Prefix
	signedDomainTransaction *externalapi.DomainTransaction
	signatures              []xc.TxSignature
}

func NewTx(args xcbuilder.TransferArgs, input *tx_input.TxInput, chainPrefix uint64) *Tx {
	return &Tx{
		args,
		input,
		util.Bech32Prefix(chainPrefix),
		nil,
		nil,
	}
}

var _ xc.Tx = &Tx{}

const hashType = consensushashing.SigHashAll

// Hash returns the tx hash or id
func (tx Tx) Hash() xc.TxHash {
	domainId := consensushashing.TransactionID(tx.signedDomainTransaction)
	return xc.TxHash(hex.EncodeToString(domainId.ByteSlice()))
}

func (tx Tx) BuildUnsignedDomainTransaction() (*externalapi.DomainTransaction, error) {
	txInput := tx.input
	var totalInput uint64
	var inputs []*externalapi.DomainTransactionInput
	prefix := tx.chainPrefix
	for _, inputUtxo := range txInput.Utxos {
		txIdBytes, err := hex.DecodeString(inputUtxo.TransactionId)
		if err != nil {
			return nil, err
		}
		transactionID, err := transactionid.FromBytes(txIdBytes)
		if err != nil {
			return nil, err
		}
		outpoint := externalapi.DomainOutpoint{
			TransactionID: *transactionID,
			Index:         uint32(inputUtxo.Index),
		}
		fromAddress, err := util.DecodeAddress(string(tx.args.GetFrom()), prefix)
		if err != nil {
			return nil, err
		}
		scriptPublicKey, err := txscript.PayToAddrScript(fromAddress)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, &externalapi.DomainTransactionInput{
			PreviousOutpoint: outpoint,
			SigOpCount:       1,
			UTXOEntry: utxo.NewUTXOEntry(
				inputUtxo.Amount.Uint64(),
				scriptPublicKey,
				false,
				0,
			),
		})
		totalInput += inputUtxo.Amount.Uint64()
	}

	var outputs []*externalapi.DomainTransactionOutput
	toAddress, err := util.DecodeAddress(string(tx.args.GetTo()), prefix)
	if err != nil {
		return nil, err
	}
	scriptPublicKey, err := txscript.PayToAddrScript(toAddress)
	if err != nil {
		return nil, err
	}
	toAmount := tx.args.GetAmount().Uint64()
	outputs = append(outputs, &externalapi.DomainTransactionOutput{
		Value:           toAmount,
		ScriptPublicKey: scriptPublicKey,
	})

	// fee limit produces the exact fee estimate
	feeBigInt, _ := txInput.GetFeeLimit()
	fee := feeBigInt.Uint64()
	// handle remainder/change
	if totalInput > toAmount+fee {
		change := totalInput - (toAmount + fee)
		changeAddress, err := util.DecodeAddress(string(tx.args.GetFrom()), prefix)
		if err != nil {
			return nil, err
		}
		changeScriptPublicKey, err := txscript.PayToAddrScript(changeAddress)
		if err != nil {
			return nil, err
		}
		changeOutput := &externalapi.DomainTransactionOutput{
			Value:           change,
			ScriptPublicKey: changeScriptPublicKey,
		}
		outputs = append(outputs, changeOutput)
	}

	domainTransaction := &externalapi.DomainTransaction{
		Version:      constants.MaxTransactionVersion,
		Inputs:       inputs,
		Outputs:      outputs,
		LockTime:     0,
		SubnetworkID: subnetworks.SubnetworkIDNative,
		Gas:          0,
		Payload:      nil,
	}
	return domainTransaction, nil
}

// Sighashes returns the tx payload to sign, aka sighash
func (tx Tx) Sighashes() ([]*xc.SignatureRequest, error) {
	domainTransaction, err := tx.BuildUnsignedDomainTransaction()
	if err != nil {
		return nil, err
	}
	// make sign requests for each input
	signRequests := make([]*xc.SignatureRequest, len(domainTransaction.Inputs))
	for i := range domainTransaction.Inputs {
		hash, err := consensushashing.CalculateSignatureHashSchnorr(
			domainTransaction,
			i,
			hashType,
			&consensushashing.SighashReusedValues{},
		)
		if err != nil {
			return nil, err
		}
		signRequests[i] = &xc.SignatureRequest{
			Payload: hash.ByteSlice(),
		}
	}
	return signRequests, nil
}

// Add signature script for each transaction input to spend it
func (tx *Tx) SetSignatures(signatures ...*xc.SignatureResponse) error {
	domainTransaction, err := tx.BuildUnsignedDomainTransaction()
	if err != nil {
		return err
	}
	if len(signatures) != len(domainTransaction.Inputs) {
		return fmt.Errorf("expected %d signatures for kaspa tx, got %d", len(domainTransaction.Inputs), len(signatures))
	}
	tx.signatures = make([]xc.TxSignature, len(signatures))
	for i, input := range domainTransaction.Inputs {
		signature := signatures[i].Signature
		signature = append(signature, byte(hashType))
		signatureScript, err := txscript.NewScriptBuilder().AddData(signature).Script()
		if err != nil {
			return fmt.Errorf("could not add kaspa signature for input %d: %w", i, err)
		}
		input.SignatureScript = signatureScript
		tx.signatures[i] = signatures[i].Signature
	}
	tx.signedDomainTransaction = domainTransaction

	return nil
}

func (tx Tx) Serialize() ([]byte, error) {
	if tx.signedDomainTransaction == nil {
		return nil, errors.New("kaspa transaction not signed")
	}
	return serialize(tx.signedDomainTransaction)
}
