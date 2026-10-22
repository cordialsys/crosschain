package sui

import (
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"github.com/btcsuite/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/sui/generated/bcs"
	"github.com/cordialsys/go-sui-sdk/v2/types"
)

// addressTransferCoins includes the sender's gas coin when it is spendable SUI.
func (input *TxInput) addressTransferCoins() []*types.Coin {
	coins := append([]*types.Coin(nil), input.Coins...)
	if input.IsNativeTransfer() && input.GasCoinOwner == input.AddressBalance.Sender {
		coins = append(coins, input.AddressBalance.GasCoins...)
	}
	seen := map[string]bool{}
	result := make([]*types.Coin, 0, len(coins))
	for _, coin := range coins {
		if coin != nil && !seen[coin.CoinObjectId.String()] {
			seen[coin.CoinObjectId.String()] = true
			result = append(result, coin)
		}
	}
	return result
}

func coinSum(coins []*types.Coin) (uint64, error) {
	var total uint64
	seen := map[string]bool{}
	for _, coin := range coins {
		if coin == nil {
			return 0, fmt.Errorf("nil coin")
		}
		if seen[coin.CoinObjectId.String()] {
			return 0, fmt.Errorf("duplicate coin %s", coin.CoinObjectId.String())
		}
		seen[coin.CoinObjectId.String()] = true
		v := coin.Balance.Uint64()
		if math.MaxUint64-total < v {
			return 0, fmt.Errorf("coin balance exceeds uint64")
		}
		total += v
	}
	return total, nil
}

type balanceFunding struct {
	addressGas bool
	gasLimit   uint64
	withdrawal uint64
	coins      []*types.Coin
	gasCoins   []*types.Coin
}

func (input *TxInput) balanceFunding(from, feePayer xc.Address, amount xc.AmountBlockchain) (balanceFunding, error) {
	var plan balanceFunding
	a := input.AddressBalance
	if a == nil || a.Sender != from || input.GasCoinOwner != feePayer {
		return plan, fmt.Errorf("address balance owner does not match transaction")
	}
	// Reject oversized/negative amounts before narrowing to Sui's u64.
	if amount.String() != fmt.Sprint(amount.Uint64()) {
		return plan, fmt.Errorf("amount must fit uint64")
	}
	plan.coins = input.addressTransferCoins()
	coinBalance, err := coinSum(plan.coins)
	if err != nil {
		return plan, err
	}
	if math.MaxUint64-coinBalance < a.Balance {
		return plan, fmt.Errorf("total balance exceeds uint64")
	}
	total := coinBalance + a.Balance
	if amount.Uint64() > total {
		return plan, fmt.Errorf("insufficient funds: have %d, need %s", total, amount.String())
	}
	plan.gasCoins = a.GasCoins
	if input.IsNativeTransfer() && from == feePayer {
		plan.gasCoins = plan.coins
	}
	gasCoins, err := coinSum(plan.gasCoins)
	if err != nil {
		return plan, err
	}
	// Gas must be funded entirely by one source. Prefer the source with more
	// capacity; coins and address funds can still be combined for the transfer.
	plan.addressGas = a.GasBalance > gasCoins
	plan.gasLimit = gasCoins
	if plan.addressGas {
		plan.gasLimit = a.GasBalance
	}
	if input.IsNativeTransfer() && from == feePayer {
		if a.Balance != a.GasBalance {
			return plan, fmt.Errorf("inconsistent SUI address balances")
		}
		plan.gasLimit = min(plan.gasLimit, total-amount.Uint64())
	}
	if plan.gasLimit == 0 {
		return plan, fmt.Errorf("insufficient SUI for gas")
	}
	budget := min(input.GasBudget, plan.gasLimit)
	availableCoins := coinBalance
	if !plan.addressGas && input.IsNativeTransfer() && from == feePayer {
		availableCoins -= budget
	}
	if amount.Uint64() > availableCoins {
		plan.withdrawal = amount.Uint64() - availableCoins
	}
	availableAddress := a.Balance
	if plan.addressGas && input.IsNativeTransfer() && from == feePayer {
		availableAddress -= budget
	}
	if plan.withdrawal > availableAddress {
		return plan, fmt.Errorf("insufficient address balance after reserving gas")
	}
	return plan, nil
}

func (txBuilder TxBuilder) newAddressTransactionBase(feePayer, from xc.Address, amount xc.AmountBlockchain, input *TxInput) (transactionBase, error) {
	plan, err := input.balanceFunding(from, feePayer, amount)
	if err != nil {
		return transactionBase{}, err
	}
	sender, err := HexToAddress(string(from))
	if err != nil {
		return transactionBase{}, err
	}
	owner, err := HexToAddress(string(feePayer))
	if err != nil {
		return transactionBase{}, err
	}
	gas := bcs.GasData{Owner: owner, Price: input.GasPrice, Budget: min(input.GasBudget, plan.gasLimit)}
	if gas.Budget == 0 {
		return transactionBase{}, fmt.Errorf("gas budget must be positive")
	}
	epoch := bcs.TransactionExpiration__Epoch(input.CurrentEpoch)
	var expiration bcs.TransactionExpiration = &epoch
	if plan.addressGas {
		chain := base58.Decode(input.AddressBalance.ChainID)
		if len(chain) != 32 {
			return transactionBase{}, fmt.Errorf("chain ID must decode to 32 bytes")
		}
		// A single epoch avoids overlapping validity windows on input refresh.
		minEpoch, maxEpoch := input.CurrentEpoch, input.CurrentEpoch
		expiration = &bcs.TransactionExpiration__ValidDuring{MinEpoch: &minEpoch, MaxEpoch: &maxEpoch, Chain: bcs.Digest(chain), Nonce: input.AddressBalance.Nonce}
	} else {
		for _, coin := range plan.gasCoins {
			obj, err := CoinToObject(coin)
			if err != nil {
				return transactionBase{}, err
			}
			gas.Payment = append(gas.Payment, ObjectRef{Field0: obj.Field0, Field1: obj.Field1, Field2: obj.Field2})
		}
	}
	input.GasBudget = gas.Budget
	return transactionBase{base: bcs.TransactionData__V1{Value: bcs.TransactionDataV1{Sender: sender, GasData: gas, Expiration: expiration}}}, nil
}

func (txBuilder TxBuilder) prepareAddressBalanceCommands(feePayer, from xc.Address, input TxInput, amount xc.AmountBlockchain) (bcs.Argument, []bcs.Command, []bcs.CallArg, error) {
	plan, err := input.balanceFunding(from, feePayer, amount)
	if err != nil {
		return nil, nil, nil, err
	}
	inputs := []bcs.CallArg{}
	commands := []bcs.Command{}
	coins := []bcs.Argument{}
	if !plan.addressGas && input.IsNativeTransfer() && from == feePayer {
		// Sui smashes all payment coins before running the PTB.
		coins = append(coins, &bcs.Argument__GasCoin{})
	} else {
		for _, coin := range plan.coins {
			obj, err := CoinToObject(coin)
			if err != nil {
				return nil, nil, nil, err
			}
			coins = append(coins, ArgumentInput(uint16(len(inputs))))
			inputs = append(inputs, &bcs.CallArg__Object{Value: obj})
		}
	}
	if plan.withdrawal > 0 {
		tag, err := parseCoinType(input.AddressBalance.CoinType)
		if err != nil {
			return nil, nil, nil, err
		}
		withdrawal := ArgumentInput(uint16(len(inputs)))
		inputs = append(inputs, &bcs.CallArg__FundsWithdrawal{Amount: plan.withdrawal, CoinType: tag})
		commands = append(commands, &bcs.Command__MoveCall{Value: bcs.ProgrammableMoveCall{Package: bcs.ObjectID{Value: bcs.AccountAddress{31: 2}}, Module: "coin", Function: "redeem_funds", TypeArguments: []bcs.TypeTag{tag}, Arguments: []bcs.Argument{withdrawal}}})
		coins = append(coins, ArgumentResult(uint16(len(commands)-1)))
	}
	if len(coins) == 0 {
		return nil, nil, nil, fmt.Errorf("no funds to spend")
	}
	if len(coins) > 1 {
		commands = append(commands, &bcs.Command__MergeCoins{Field0: coins[0], Field1: coins[1:]})
	}
	return coins[0], commands, inputs, nil
}

// parseCoinType handles nested Move type parameters as well as simple coin types.
func parseCoinType(value string) (bcs.TypeTag, error) {
	parser := typeTagParser{remaining: strings.TrimSpace(value)}
	tag, err := parser.parse(0)
	if err == nil && strings.TrimSpace(parser.remaining) != "" {
		err = fmt.Errorf("unexpected trailing coin type %q", parser.remaining)
	}
	if err == nil {
		if _, ok := tag.(*bcs.TypeTag__Struct); !ok {
			err = fmt.Errorf("coin type must be a struct")
		}
	}
	return tag, err
}

type typeTagParser struct{ remaining string }

func (p *typeTagParser) parse(depth int) (bcs.TypeTag, error) {
	if depth > 32 {
		return nil, fmt.Errorf("coin type nesting exceeds 32")
	}
	p.remaining = strings.TrimSpace(p.remaining)
	i := strings.IndexAny(p.remaining, "<>, ")
	if i < 0 {
		i = len(p.remaining)
	}
	name := p.remaining[:i]
	p.remaining = strings.TrimSpace(p.remaining[i:])
	var args []bcs.TypeTag
	if strings.HasPrefix(p.remaining, "<") {
		p.remaining = p.remaining[1:]
		for {
			arg, err := p.parse(depth + 1)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if strings.HasPrefix(p.remaining, ",") {
				p.remaining = p.remaining[1:]
				continue
			}
			if !strings.HasPrefix(p.remaining, ">") {
				return nil, fmt.Errorf("unterminated coin type")
			}
			p.remaining = strings.TrimSpace(p.remaining[1:])
			break
		}
	}
	if name == "vector" && len(args) == 1 {
		return &bcs.TypeTag__Vector{Value: args[0]}, nil
	}
	parts := strings.Split(name, "::")
	if len(parts) == 3 {
		address := strings.TrimPrefix(parts[0], "0x")
		if address == parts[0] || len(address) == 0 || len(address) > 64 {
			return nil, fmt.Errorf("invalid coin package")
		}
		raw, err := hex.DecodeString(strings.Repeat("0", 64-len(address)) + address)
		if err != nil {
			return nil, err
		}
		var addr bcs.AccountAddress
		copy(addr[:], raw)
		for _, id := range parts[1:] {
			if id == "" {
				return nil, fmt.Errorf("empty Move identifier")
			}
			for i, c := range id {
				if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
					return nil, fmt.Errorf("invalid Move identifier %q", id)
				}
			}
		}
		return &bcs.TypeTag__Struct{Value: bcs.StructTag{Address: addr, Module: bcs.Identifier(parts[1]), Name: bcs.Identifier(parts[2]), TypeArgs: args}}, nil
	}
	if len(args) == 0 {
		switch name {
		case "bool":
			return &bcs.TypeTag__Bool{}, nil
		case "u8":
			return &bcs.TypeTag__U8{}, nil
		case "u16":
			return &bcs.TypeTag__U16{}, nil
		case "u32":
			return &bcs.TypeTag__U32{}, nil
		case "u64":
			return &bcs.TypeTag__U64{}, nil
		case "u128":
			return &bcs.TypeTag__U128{}, nil
		case "u256":
			return &bcs.TypeTag__U256{}, nil
		case "address":
			return &bcs.TypeTag__Address{}, nil
		case "signer":
			return &bcs.TypeTag__Signer{}, nil
		}
	}
	return nil, fmt.Errorf("invalid Move type %q", name)
}
