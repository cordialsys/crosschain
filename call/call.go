package call

type Method string

type CallMethodMeta struct {
	Method         Method
	IsTransaction  bool
	NeedsBroadcast bool
	Valid          bool
}

const EthSendTransaction Method = "eth_sendTransaction"
const EthSignTransaction Method = "eth_signTransaction"
const PersonalSign Method = "personal_sign"
const OfferAccept Method = "offer_accept"
const SettlementComplete Method = "settlement_complete"
const SolanaSignIn Method = "solana:signIn"
const SolanaSignMessage Method = "solana:signMessage"
const SolanaSignTransaction Method = "solana:signTransaction"
const SolanaSignAndSendTransaction Method = "solana:signAndSendTransaction"

var CallMethods = []CallMethodMeta{
	{
		Method:         EthSendTransaction,
		IsTransaction:  true,
		NeedsBroadcast: true,
		Valid:          true,
	},
	{
		Method:         EthSignTransaction,
		IsTransaction:  true,
		NeedsBroadcast: false,
		Valid:          true,
	},
	{
		Method:         PersonalSign,
		IsTransaction:  false,
		NeedsBroadcast: false,
		Valid:          true,
	},
	{
		Method:         OfferAccept,
		IsTransaction:  true,
		NeedsBroadcast: true,
		Valid:          true,
	},
	{
		Method:         SettlementComplete,
		IsTransaction:  true,
		NeedsBroadcast: true,
		Valid:          true,
	},
	{
		Method:         SolanaSignIn,
		IsTransaction:  false,
		NeedsBroadcast: false,
		Valid:          true,
	},
	{
		Method:         SolanaSignMessage,
		IsTransaction:  false,
		NeedsBroadcast: false,
		Valid:          true,
	},
	{
		Method:         SolanaSignTransaction,
		IsTransaction:  true,
		NeedsBroadcast: false,
		Valid:          true,
	},
	{
		Method:         SolanaSignAndSendTransaction,
		IsTransaction:  true,
		NeedsBroadcast: true,
		Valid:          true,
	},
}

// Indicate if it's just an offline message being signed, or a transaction to land on chain.
func (c Method) IsTransaction() bool {
	for _, method := range CallMethods {
		if method.Method == c {
			return method.IsTransaction
		}
	}
	return false
}

// Are we responsible for broadcasting, or is the 3rd party responsible?
func (c Method) NeedsBroadcast() bool {
	for _, method := range CallMethods {
		if method.Method == c {
			return method.NeedsBroadcast
		}
	}
	return false
}

// Is this a valid call method?
func (c Method) Valid() bool {
	for _, method := range CallMethods {
		if method.Method == c {
			return method.Valid
		}
	}
	return false
}
