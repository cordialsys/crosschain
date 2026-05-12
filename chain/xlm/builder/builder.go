package builder

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math"
	"time"

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
		TimeBounds: xlm.NewTimeout(time.Unix(txInput.Timestamp, 0), txInput.TransactionActiveTime),
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
		} else if !isToken && txInput.AccountMerge && args.InclusiveFeeSpendingEnabled() {
			// Sweep transfer: merge the source account into the destination.
			// AccountMerge only operates on the native asset, so we require
			// !isToken here in addition to the AccountMerge flag itself — that
			// way a stray flag on a token transfer cannot accidentally drain
			// the sender's XLM balance. AccountMerge releases the network
			// reserve and transfers the entire remaining XLM balance, so the
			// on-chain effect matches a true "send all". Guarded with
			// InclusiveFeeSpendingEnabled so callers that did not opt in to
			// inclusive-fee never see their amount silently replaced.
			destinationMuxedAccount, err := common.MuxedAccountFromAddress(to)
			if err != nil {
				return &xlmtx.Tx{}, fmt.Errorf("invalid `to` address for merge: %w", err)
			}
			mergeBody, err := xdr.NewOperationBody(xdr.OperationTypeAccountMerge, destinationMuxedAccount)
			if err != nil {
				return &xlmtx.Tx{}, fmt.Errorf("failed to create account-merge operation: %w", err)
			}
			operations = append(operations, xdr.Operation{
				SourceAccount: &opSourceAccount,
				Body:          mergeBody,
			})
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

	if len(operations) == 0 {
		return &xlmtx.Tx{}, fmt.Errorf("xlm transfer produced no operations: amount must be greater than 0 unless creating a trustline")
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
// Auth and the operation are constructed natively. Simulated transaction data
// is used when present so the footprint matches Soroban RPC exactly.
func (builder TxBuilder) buildSACTransfer(args xcbuilder.TransferArgs, txInput *TxInput, from xc.Address, to xc.Address) (*xdr.OperationBody, *xdr.SorobanTransactionData, error) {
	// Derive the SAC contract address from the asset. Missing contract means native XLM.
	contract, _ := args.GetContract()
	xdrAsset, err := common.CreateAssetFromContract(contract)
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
	if len(txInput.SorobanAuthorizationEntries) > 0 {
		subInvocations, ok, err := sorobanSubInvocationsFromInput(txInput, contractFn)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, fmt.Errorf("simulated soroban authorization entries did not match SAC transfer invocation")
		}
		authEntry.RootInvocation.SubInvocations = subInvocations
	}

	invokeOp := xdr.InvokeHostFunctionOp{
		HostFunction: hostFn,
		Auth:         []xdr.SorobanAuthorizationEntry{authEntry},
	}

	opBody, err := xdr.NewOperationBody(xdr.OperationTypeInvokeHostFunction, invokeOp)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create invoke host function operation: %w", err)
	}

	if txInput.SorobanTransactionData != "" {
		sorobanData, err := sorobanTransactionDataFromInput(txInput)
		if err != nil {
			return nil, nil, err
		}
		return &opBody, sorobanData, nil
	}

	// Construct the ledger footprint natively.
	// SAC transfer touches the SAC contract instance, sender balance, and
	// receiver contract balance. Native account balances use Account entries;
	// issued asset account balances use TrustLine entries.
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

	var senderBalanceKey xdr.LedgerKey
	if xdrAsset.IsNative() {
		senderBalanceKey, err = accountLedgerKey(from)
		if err != nil {
			return nil, nil, err
		}
	} else {
		senderBalanceKey = xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeTrustline,
			TrustLine: &xdr.LedgerKeyTrustLine{
				AccountId: fromAccountId,
				Asset:     xdrAsset.ToTrustLineAsset(),
			},
		}
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

	readWriteKeys := []xdr.LedgerKey{senderBalanceKey, receiverBalanceKey}

	sorobanData := xdr.SorobanTransactionData{
		Resources: xdr.SorobanResources{
			Footprint: xdr.LedgerFootprint{
				ReadOnly:  []xdr.LedgerKey{sacInstanceKey},
				ReadWrite: readWriteKeys,
			},
			Instructions:  xdr.Uint32(txInput.SorobanInstructions),
			DiskReadBytes: xdr.Uint32(txInput.SorobanDiskReadBytes),
			WriteBytes:    xdr.Uint32(txInput.SorobanWriteBytes),
		},
		ResourceFee: xdr.Int64(txInput.SorobanResourceFee),
	}

	return &opBody, &sorobanData, nil
}

func sorobanTransactionDataFromInput(txInput *TxInput) (*xdr.SorobanTransactionData, error) {
	dataBz, err := base64.StdEncoding.DecodeString(txInput.SorobanTransactionData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode soroban transaction data: %w", err)
	}

	var sorobanData xdr.SorobanTransactionData
	if err := sorobanData.UnmarshalBinary(dataBz); err != nil {
		return nil, fmt.Errorf("failed to unmarshal soroban transaction data: %w", err)
	}

	if txInput.SorobanInstructions > 0 {
		sorobanData.Resources.Instructions = xdr.Uint32(txInput.SorobanInstructions)
	}
	if txInput.SorobanDiskReadBytes > 0 {
		sorobanData.Resources.DiskReadBytes = xdr.Uint32(txInput.SorobanDiskReadBytes)
	}
	if txInput.SorobanWriteBytes > 0 {
		sorobanData.Resources.WriteBytes = xdr.Uint32(txInput.SorobanWriteBytes)
	}
	if txInput.SorobanResourceFee > 0 {
		sorobanData.ResourceFee = xdr.Int64(txInput.SorobanResourceFee)
	}

	return &sorobanData, nil
}

func sorobanSubInvocationsFromInput(txInput *TxInput, rootFunction xdr.SorobanAuthorizedFunction) ([]xdr.SorobanAuthorizedInvocation, bool, error) {
	for _, encoded := range txInput.SorobanAuthorizationEntries {
		dataBz, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, false, fmt.Errorf("failed to decode soroban authorization entry: %w", err)
		}

		var entry xdr.SorobanAuthorizationEntry
		if err := entry.UnmarshalBinary(dataBz); err != nil {
			return nil, false, fmt.Errorf("failed to unmarshal soroban authorization entry: %w", err)
		}

		matches, err := sorobanAuthorizedFunctionEqual(entry.RootInvocation.Function, rootFunction)
		if err != nil {
			return nil, false, err
		}
		if !matches {
			continue
		}
		return entry.RootInvocation.SubInvocations, true, nil
	}
	return nil, false, nil
}

func sorobanAuthorizedFunctionEqual(a xdr.SorobanAuthorizedFunction, b xdr.SorobanAuthorizedFunction) (bool, error) {
	aBz, err := a.MarshalBinary()
	if err != nil {
		return false, fmt.Errorf("failed to marshal soroban authorized function: %w", err)
	}
	bBz, err := b.MarshalBinary()
	if err != nil {
		return false, fmt.Errorf("failed to marshal soroban authorized function: %w", err)
	}
	return bytes.Equal(aBz, bBz), nil
}

func accountLedgerKey(address xc.Address) (xdr.LedgerKey, error) {
	accountID, err := xdr.AddressToAccountId(string(address))
	if err != nil {
		return xdr.LedgerKey{}, fmt.Errorf("failed to parse footprint account %s: %w", address, err)
	}
	return xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeAccount,
		Account: &xdr.LedgerKeyAccount{
			AccountId: accountID,
		},
	}, nil
}

func (txBuilder TxBuilder) SupportsMemo() xc.MemoSupport {
	// XLM supports memo
	return xc.MemoSupportString
}
