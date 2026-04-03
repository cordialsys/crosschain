package builder

import (
	"fmt"
	"math"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/xlm"
	common "github.com/cordialsys/crosschain/chain/xlm/common"
	xlmtx "github.com/cordialsys/crosschain/chain/xlm/tx"
	xlminput "github.com/cordialsys/crosschain/chain/xlm/tx_input"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type TxBuilder struct {
	Asset *xc.ChainBaseConfig
}

var _ xcbuilder.FullTransferBuilder = &TxBuilder{}
var _ xcbuilder.BuilderSupportsFeePayer = &TxBuilder{}

func (builder TxBuilder) SupportsFeePayer() xcbuilder.FeePayerType {
	return xcbuilder.FeePayerWithConflicts
}

type TxInput = xlminput.TxInput
type Tx = xlmtx.Tx

func NewTxBuilder(asset *xc.ChainBaseConfig) (*TxBuilder, error) {
	return &TxBuilder{
		Asset: asset,
	}, nil
}

func (builder TxBuilder) Transfer(args xcbuilder.TransferArgs, input xc.TxInput) (xc.Tx, error) {
	from := args.GetFrom()
	to := args.GetTo()
	amount := args.GetAmount()

	txInput := input.(*TxInput)

	// Determine the transaction source account (who pays fees and provides sequence)
	txSourceAddress := from
	txSequence := txInput.GetXlmSequence()
	feePayer, hasFeePayer := args.GetFeePayer()
	if hasFeePayer {
		txSourceAddress = feePayer
		txSequence = int64(txInput.FeePayerSequence)
	}

	txSourceAccount, err := common.MuxedAccountFromAddress(txSourceAddress)
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("invalid transaction source address: %w", err)
	}

	// The operation source is always the sender
	opSourceAccount, err := common.MuxedAccountFromAddress(from)
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("invalid `from` address: %w", err)
	}

	preconditions := xlm.Preconditions{
		TimeBounds: xlm.NewTimeout(txInput.TransactionActiveTime),
	}

	xdrMemo := xdr.Memo{}
	if memo, ok := args.GetMemo(); ok {
		xdrMemo, err = xdr.NewMemo(xdr.MemoTypeMemoText, memo)
		if err != nil {
			return &xlmtx.Tx{}, fmt.Errorf("failed to create memo: %w", err)
		}
	}

	txe := xdr.TransactionV1Envelope{
		Tx: xdr.Transaction{
			SourceAccount: txSourceAccount,
			// We can skip fee * operation_count multiplication because the transfer is a single
			// `Payment` operation
			Fee:    xdr.Uint32(txInput.MaxFee),
			SeqNum: xdr.SequenceNumber(txSequence),
			Cond:   preconditions.BuildXDR(),
			Memo:   xdrMemo,
		},
	}

	if memo, ok := args.GetMemo(); ok {
		var xdrMemo xdr.Memo
		xdrMemo, err = xdr.NewMemo(xdr.MemoTypeMemoText, memo)
		if err != nil {
			return &xlmtx.Tx{}, fmt.Errorf("failed to create memo: %w", err)
		}
		txe.Tx.Memo = xdrMemo
	}

	xdrAmount := xdr.Int64(amount.Int().Int64())

	var operations []xdr.Operation

	// If the sender needs a trustline for the token, prepend a ChangeTrust operation.
	if txInput.NeedsCreateTrustline {
		if contract, ok := args.GetContract(); ok {
			contractDetails, err := common.GetAssetAndIssuerFromContract(string(contract))
			if err != nil {
				return &xlmtx.Tx{}, fmt.Errorf("failed to get contract details for trustline: %w", err)
			}
			changeTrustAsset, err := common.CreateChangeTrustAsset(contractDetails)
			if err != nil {
				return &xlmtx.Tx{}, fmt.Errorf("failed to create change trust asset: %w", err)
			}
			changeTrustBody, err := xdr.NewOperationBody(xdr.OperationTypeChangeTrust, xdr.ChangeTrustOp{
				Line:  changeTrustAsset,
				Limit: xdr.Int64(math.MaxInt64),
			})
			if err != nil {
				return &xlmtx.Tx{}, fmt.Errorf("failed to create change trust operation: %w", err)
			}
			operations = append(operations, xdr.Operation{
				SourceAccount: &opSourceAccount,
				Body:          changeTrustBody,
			})
		}
	}

	if common.IsContractAddress(to) {
		// Soroban SAC invocation for contract (C...) address destinations
		sacOp, sorobanData, err := builder.buildSACTransfer(args, txInput, from, to)
		if err != nil {
			return &xlmtx.Tx{}, fmt.Errorf("failed to build SAC transfer: %w", err)
		}
		operations = append(operations, xdr.Operation{
			SourceAccount: &opSourceAccount,
			Body:          *sacOp,
		})
		// Attach Soroban data to the transaction
		txe.Tx.Ext, err = xdr.NewTransactionExt(1, *sorobanData)
		if err != nil {
			return &xlmtx.Tx{}, fmt.Errorf("failed to set soroban transaction data: %w", err)
		}
		// Soroban fee = base inclusion fee + resource fee
		txe.Tx.Fee = xdr.Uint32(txInput.MaxFee) + xdr.Uint32(txInput.SorobanResourceFee)
	} else {
		_, isToken := args.GetContract()
		if !txInput.DestinationFunded && !isToken {
			// Use CreateAccount for new/unfunded native XLM destinations
			destinationAccountId, err := xdr.AddressToAccountId(string(to))
			if err != nil {
				return &xlmtx.Tx{}, fmt.Errorf("invalid `to` address: %w", err)
			}
			xdrCreateAccount := xdr.CreateAccountOp{
				Destination:     destinationAccountId,
				StartingBalance: xdrAmount,
			}
			xdrOperationBody, err := xdr.NewOperationBody(xdr.OperationTypeCreateAccount, xdrCreateAccount)
			if err != nil {
				return &xlmtx.Tx{}, fmt.Errorf("failed to create operation body: %w", err)
			}
			if xdrAmount > 0 {
				operations = append(operations, xdr.Operation{
					SourceAccount: &opSourceAccount,
					Body:          xdrOperationBody,
				})
			}
		} else {
			// Use Payment for funded accounts or token transfers
			destinationMuxedAccount, err := common.MuxedAccountFromAddress(to)
			if err != nil {
				return &xlmtx.Tx{}, fmt.Errorf("invalid `to` address: %w", err)
			}
			var xdrAsset xdr.Asset
			if contract, ok := args.GetContract(); ok {
				contractDetails, err := common.GetAssetAndIssuerFromContract(string(contract))
				if err != nil {
					return &xlmtx.Tx{}, fmt.Errorf("failed to get contract details: %w", err)
				}
				xdrAsset, err = common.CreateAssetFromContractDetails(contractDetails)
				if err != nil {
					return &xlmtx.Tx{}, fmt.Errorf("failed to create token details: %w", err)
				}
			} else {
				xdrAsset.Type = xdr.AssetTypeAssetTypeNative
			}

			xdrPayment := xdr.PaymentOp{
				Destination: destinationMuxedAccount,
				Amount:      xdrAmount,
				Asset:       xdrAsset,
			}
			xdrOperationBody, err := xdr.NewOperationBody(xdr.OperationTypePayment, xdrPayment)
			if err != nil {
				return &xlmtx.Tx{}, fmt.Errorf("failed to create operation body: %w", err)
			}
			// Skip the payment operation when amount is 0 (trustline-only transaction)
			if xdrAmount > 0 {
				operations = append(operations, xdr.Operation{
					SourceAccount: &opSourceAccount,
					Body:          xdrOperationBody,
				})
			}
		}
	}

	txe.Tx.Operations = operations

	xlmTx, err := xdr.NewTransactionEnvelope(xdr.EnvelopeTypeEnvelopeTypeTx, txe)
	if err != nil {
		return &xlmtx.Tx{}, fmt.Errorf("failed to create transaction envelope: %w", err)
	}

	tx := &xlmtx.Tx{
		TxEnvelope:        &xlmTx,
		NetworkPassphrase: txInput.Passphrase,
	}
	if hasFeePayer {
		tx.FeePayer = feePayer
	}
	return tx, nil
}

// buildSACTransfer builds an InvokeHostFunction operation that calls the SAC transfer function.
// The SAC transfer signature is: transfer(from: Address, to: Address, amount: i128)
// All Soroban data (auth, footprint, resources) is constructed natively — no simulation RPC needed.
func (builder TxBuilder) buildSACTransfer(args xcbuilder.TransferArgs, txInput *TxInput, from xc.Address, to xc.Address) (*xdr.OperationBody, *xdr.SorobanTransactionData, error) {
	// Derive the SAC contract address from the token asset
	contract, ok := args.GetContract()
	if !ok {
		return nil, nil, fmt.Errorf("contract is required for SAC transfers")
	}
	contractDetails, err := common.GetAssetAndIssuerFromContract(string(contract))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse contract: %w", err)
	}
	xdrAsset, err := common.CreateAssetFromContractDetails(contractDetails)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create asset: %w", err)
	}
	sacId, err := xdrAsset.ContractID(txInput.Passphrase)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive SAC contract ID: %w", err)
	}
	contractId := xdr.ContractId(sacId)
	contractAddr := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractId,
	}

	// Build the SAC transfer arguments: from, to, amount
	fromScVal, err := common.ScValAddress(string(from))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode from address: %w", err)
	}
	toScVal, err := common.ScValAddress(string(to))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encode to address: %w", err)
	}
	amountScVal := common.ScValI128(args.GetAmount().Int().Int64())

	invokeArgs := xdr.InvokeContractArgs{
		ContractAddress: contractAddr,
		FunctionName:    "transfer",
		Args:            []xdr.ScVal{fromScVal, toScVal, amountScVal},
	}

	hostFn, err := xdr.NewHostFunction(xdr.HostFunctionTypeHostFunctionTypeInvokeContract, invokeArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create host function: %w", err)
	}

	// Construct auth entry natively.
	// For SAC transfer where sender = transaction source, credentials are SOURCE_ACCOUNT
	// and the invocation is the transfer call with no sub-invocations.
	contractFn, err := xdr.NewSorobanAuthorizedFunction(
		xdr.SorobanAuthorizedFunctionTypeSorobanAuthorizedFunctionTypeContractFn,
		invokeArgs,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create authorized function: %w", err)
	}
	authEntry := xdr.SorobanAuthorizationEntry{
		Credentials: xdr.SorobanCredentials{
			Type: xdr.SorobanCredentialsTypeSorobanCredentialsSourceAccount,
		},
		RootInvocation: xdr.SorobanAuthorizedInvocation{
			Function:       contractFn,
			SubInvocations: nil,
		},
	}

	invokeOp := xdr.InvokeHostFunctionOp{
		HostFunction: hostFn,
		Auth:         []xdr.SorobanAuthorizationEntry{authEntry},
	}

	opBody, err := xdr.NewOperationBody(xdr.OperationTypeInvokeHostFunction, invokeOp)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create invoke host function operation: %w", err)
	}

	// Construct the ledger footprint natively.
	// SAC transfer touches:
	//   ReadOnly: SAC contract instance
	//   ReadWrite: sender's trustline (G-addr balance), receiver's contract data (C-addr balance)
	fromAccountId, err := xdr.AddressToAccountId(string(from))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse from account: %w", err)
	}
	toContractAddr, err := common.ScAddressFromString(string(to))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse to contract address: %w", err)
	}

	// SAC contract instance key
	sacInstanceKey := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.LedgerKeyContractData{
			Contract:   contractAddr,
			Key:        xdr.ScVal{Type: xdr.ScValTypeScvLedgerKeyContractInstance},
			Durability: xdr.ContractDataDurabilityPersistent,
		},
	}

	// Sender's classic trustline (SAC uses TrustLine entries for G-address balances)
	senderTrustlineKey := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeTrustline,
		TrustLine: &xdr.LedgerKeyTrustLine{
			AccountId: fromAccountId,
			Asset:     xdrAsset.ToTrustLineAsset(),
		},
	}

	// Receiver's contract balance (SAC uses ContractData for C-address balances)
	// Key format: Vec[Symbol("Balance"), Address(to)]
	balanceSym := common.ScValSymbol("Balance")
	toAddrScVal := xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &toContractAddr}
	balanceVec := xdr.ScVec{balanceSym, toAddrScVal}
	balanceVecPtr := &balanceVec
	receiverBalanceKey := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.LedgerKeyContractData{
			Contract: contractAddr,
			Key: xdr.ScVal{
				Type: xdr.ScValTypeScvVec,
				Vec:  &balanceVecPtr,
			},
			Durability: xdr.ContractDataDurabilityPersistent,
		},
	}

	sorobanData := xdr.SorobanTransactionData{
		Resources: xdr.SorobanResources{
			Footprint: xdr.LedgerFootprint{
				ReadOnly:  []xdr.LedgerKey{sacInstanceKey},
				ReadWrite: []xdr.LedgerKey{senderTrustlineKey, receiverBalanceKey},
			},
			Instructions:  xdr.Uint32(txInput.SorobanInstructions),
			DiskReadBytes: xdr.Uint32(txInput.SorobanDiskReadBytes),
			WriteBytes:    xdr.Uint32(txInput.SorobanWriteBytes),
		},
		ResourceFee: xdr.Int64(txInput.SorobanResourceFee),
	}

	return &opBody, &sorobanData, nil
}

func (txBuilder TxBuilder) SupportsMemo() xc.MemoSupport {
	// XLM supports memo
	return xc.MemoSupportString
}
