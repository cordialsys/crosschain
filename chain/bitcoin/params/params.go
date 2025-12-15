package params

import (
	"errors"

	"github.com/btcsuite/btcd/chaincfg"
	xc "github.com/cordialsys/crosschain"
)

func GetParams(cfg *xc.ChainBaseConfig) (chaincfg.Params, error) {
	switch xc.NativeAsset(cfg.Chain) {
	case xc.BTC, xc.BCH:
		return BtcNetworks.GetParams(cfg.Network), nil
	case xc.DOGE:
		return DogeNetworks.GetParams(cfg.Network), nil
	case xc.LTC:
		return LtcNetworks.GetParams(cfg.Network), nil
	case xc.ZEC:
		return ZecNetworks.GetParams(cfg.Network), nil
	case xc.DASH:
		return DashNetworks.GetParams(cfg.Network), nil
	case xc.FLUX:
		return FluxNetworks.GetParams(cfg.Network), nil
	}
	return chaincfg.Params{}, errors.New("unsupported bitcoin chain: " + string(cfg.Chain))
}

// UTXO chains have mainnet, testnet, and regtest/devnet network types built in.
type Network string

const Mainnet Network = "mainnet"
const Testnet Network = "testnet"
const Regtest Network = "regtest"

type NetworkTriple struct {
	Mainnet chaincfg.Params
	Testnet chaincfg.Params
	Regtest chaincfg.Params
}

func init() {
	_ = chaincfg.Register(&BtcNetworks.Mainnet)
	_ = chaincfg.Register(&BtcNetworks.Testnet)
	_ = chaincfg.Register(&BtcNetworks.Regtest)

	_ = chaincfg.Register(&DogeNetworks.Mainnet)
	_ = chaincfg.Register(&DogeNetworks.Testnet)
	_ = chaincfg.Register(&DogeNetworks.Regtest)

	_ = chaincfg.Register(&LtcNetworks.Mainnet)
	_ = chaincfg.Register(&LtcNetworks.Testnet)
	_ = chaincfg.Register(&LtcNetworks.Regtest)

	_ = chaincfg.Register(&ZecNetworks.Mainnet)
	_ = chaincfg.Register(&ZecNetworks.Testnet)
}

func (n *NetworkTriple) GetParams(network string) chaincfg.Params {
	switch Network(network) {
	case Mainnet:
		return n.Mainnet
	case Testnet:
		return n.Testnet
	case Regtest:
		return n.Regtest
	default:
		return n.Regtest
	}
}

var BtcNetworks *NetworkTriple = &NetworkTriple{
	Mainnet: chaincfg.MainNetParams,
	// testnet4 is the upgrade to testnet3
	// https://github.com/bitcoin/bitcoin/blob/45719390a1434ad7377a5ed05dcd73028130cf2d/src/kernel/chainparams.cpp
	Testnet: chaincfg.Params{
		Name: "testnet",
		Net:  0x283f161c,

		PubKeyHashAddrID: 111,
		ScriptHashAddrID: 196,
		PrivateKeyID:     239,
		HDPublicKeyID:    [4]byte{0x04, 0x35, 0x87, 0xCF},
		HDPrivateKeyID:   [4]byte{0x04, 0x35, 0x83, 0x94},
		Bech32HRPSegwit:  "tb",
	},
	Regtest: chaincfg.RegressionNetParams,
}

var DogeNetworks *NetworkTriple = &NetworkTriple{
	Mainnet: chaincfg.Params{
		Name: "mainnet",
		Net:  0xc0c0c0c0,

		// Address encoding magics
		PubKeyHashAddrID: 30,
		ScriptHashAddrID: 22,
		PrivateKeyID:     158,

		// BIP32 hierarchical deterministic extended key magics
		HDPrivateKeyID: [4]byte{0x02, 0xfa, 0xc3, 0x98}, // starts with xprv
		HDPublicKeyID:  [4]byte{0x02, 0xfa, 0xca, 0xfd}, // starts with xpub

		// Human-readable part for Bech32 encoded segwit addresses, as defined in
		// BIP 173. Dogecoin does not actually support this, but we do not want to
		// collide with real addresses, so we specify it.
		Bech32HRPSegwit: "doge",
	},
	Testnet: chaincfg.Params{
		Name: "testnet",
		Net:  0xfcc1b7dc,

		// Address encoding magics
		PubKeyHashAddrID: 113,
		ScriptHashAddrID: 196,
		PrivateKeyID:     241,

		// BIP32 hierarchical deterministic extended key magics
		HDPrivateKeyID: [4]byte{0x04, 0x35, 0x83, 0x94}, // starts with xprv
		HDPublicKeyID:  [4]byte{0x04, 0x35, 0x87, 0xcf}, // starts with xpub

		// Human-readable part for Bech32 encoded segwit addresses, as defined in
		// BIP 173. Dogecoin does not actually support this, but we do not want to
		// collide with real addresses, so we specify it.
		Bech32HRPSegwit: "doget",
	},
	Regtest: chaincfg.Params{
		Name: "regtest",

		// Dogecoin has 0xdab5bffa as RegTest (same as Bitcoin's RegTest).
		// Setting it to an arbitrary value (leet_hex(dogecoin)), so that we can
		// register the regtest network.
		Net: 0xfabfb5da,

		// Address encoding magics
		PubKeyHashAddrID: 111,
		ScriptHashAddrID: 196,
		PrivateKeyID:     239,

		// BIP32 hierarchical deterministic extended key magics
		HDPrivateKeyID: [4]byte{0x04, 0x35, 0x83, 0x94}, // starts with xprv
		HDPublicKeyID:  [4]byte{0x04, 0x35, 0x87, 0xcf}, // starts with xpub

		// Human-readable part for Bech32 encoded segwit addresses, as defined in
		// BIP 173. Dogecoin does not actually support this, but we do not want to
		// collide with real addresses, so we specify it.
		Bech32HRPSegwit: "dogert",
	},
}

var LtcNetworks *NetworkTriple = &NetworkTriple{
	Mainnet: chaincfg.Params{
		Name: "mainnet",
		Net:  0xfbc0b6db,

		// Address encoding magics
		PubKeyHashAddrID: 48,
		ScriptHashAddrID: 50,
		PrivateKeyID:     176,

		// BIP32 hierarchical deterministic extended key magics
		HDPrivateKeyID: [4]byte{0x04, 0x88, 0xAD, 0xE4}, // starts with xprv
		HDPublicKeyID:  [4]byte{0x04, 0x88, 0xB2, 0x1E}, // starts with xpub

		// Human-readable part for Bech32 encoded segwit addresses, as defined in
		// BIP 173. Dogecoin does not actually support this, but we do not want to
		// collide with real addresses, so we specify it.
		Bech32HRPSegwit: "ltc",
	},
	Testnet: chaincfg.Params{
		Name: "testnet",
		Net:  0xfdd2c8f1,

		// Address encoding magics
		PubKeyHashAddrID: 111,
		ScriptHashAddrID: 196,
		PrivateKeyID:     239,

		// BIP32 hierarchical deterministic extended key magics
		HDPrivateKeyID: [4]byte{0x04, 0x35, 0x83, 0x94}, // starts with xprv
		HDPublicKeyID:  [4]byte{0x04, 0x35, 0x87, 0xCF}, // starts with xpub

		// Human-readable part for Bech32 encoded segwit addresses, as defined in
		// BIP 173. Dogecoin does not actually support this, but we do not want to
		// collide with real addresses, so we specify it.
		Bech32HRPSegwit: "tltc",
	},
	Regtest: chaincfg.Params{
		Name: "regtest",

		// Dogecoin has 0xdab5bffa as RegTest (same as Bitcoin's RegTest).
		// Setting it to an arbitrary value (leet_hex(dogecoin)), so that we can
		// register the regtest network.
		Net: 0xfabfb5da,

		// Address encoding magics
		PubKeyHashAddrID: 111,
		ScriptHashAddrID: 196,
		PrivateKeyID:     239,

		// BIP32 hierarchical deterministic extended key magics
		HDPrivateKeyID: [4]byte{0x04, 0x35, 0x83, 0x94}, // starts with xprv
		HDPublicKeyID:  [4]byte{0x04, 0x35, 0x87, 0xcf}, // starts with xpub

		// Human-readable part for Bech32 encoded segwit addresses, as defined in
		// BIP 173. Dogecoin does not actually support this, but we do not want to
		// collide with real addresses, so we specify it.
		Bech32HRPSegwit: "rltc",
	},
}

var ZecNetworks *NetworkTriple = &NetworkTriple{
	Mainnet: chaincfg.Params{
		Name: "mainnet",
		// Net:  0x6427E924,
		Net: 0xd9b4bef9,
		// https://zips.z.cash/protocol/protocol.pdf
		PubKeyHashAddrID: 0x1C,
		ScriptHashAddrID: 0xB8,
		PrivateKeyID:     0x80,
		HDPublicKeyID:    [4]byte{0x00, 0x00, 0x00, 0x00},
		HDPrivateKeyID:   [4]byte{0x00, 0x00, 0x00, 0x00},

		// not used for zcash
		Bech32HRPSegwit: "",
	},
	Testnet: chaincfg.Params{
		Name: "testnet",
		Net:  0x0709110B,
		// https://zips.z.cash/protocol/protocol.pdf
		PubKeyHashAddrID: 0x1D,
		ScriptHashAddrID: 0x25,
		PrivateKeyID:     0xEF,
		HDPublicKeyID:    [4]byte{0x00, 0x00, 0x00, 0x00},
		HDPrivateKeyID:   [4]byte{0x00, 0x00, 0x00, 0x00},

		// not used for zcash
		Bech32HRPSegwit: "",
	},
}

var DashNetworks *NetworkTriple = &NetworkTriple{
	Mainnet: chaincfg.Params{
		Name: "mainnet",
		Net:  0xbf0c6bbd,

		// Address encoding magics
		PubKeyHashAddrID:        0x4c, // 76 - starts with X
		ScriptHashAddrID:        0x10, // 16 - starts with 7
		PrivateKeyID:            0xcc, // 204 - WIF starts with X
		WitnessPubKeyHashAddrID: 0x00, // Not used
		WitnessScriptHashAddrID: 0x00, // Not used

		// BIP32 hierarchical deterministic extended key magics
		HDPrivateKeyID: [4]byte{0x04, 0x88, 0xad, 0xe4}, // xprv
		HDPublicKeyID:  [4]byte{0x04, 0x88, 0xb2, 0x1e}, // xpub

		// BIP44 coin type
		HDCoinType: 5,
	},
	Testnet: chaincfg.Params{
		Name: "testnet3",
		Net:  0xffcae2ce,

		// Address encoding magics
		PubKeyHashAddrID:        0x8c, // 140 - starts with y
		ScriptHashAddrID:        0x13, // 19 - starts with 8 or 9
		PrivateKeyID:            0xef, // 239 - WIF starts with c
		WitnessPubKeyHashAddrID: 0x00, // Not used
		WitnessScriptHashAddrID: 0x00, // Not used

		// BIP32 hierarchical deterministic extended key magics
		HDPrivateKeyID: [4]byte{0x04, 0x35, 0x83, 0x94}, // tprv
		HDPublicKeyID:  [4]byte{0x04, 0x35, 0x87, 0xcf}, // tpub

		// BIP44 coin type (testnet uses coin type 1)
		HDCoinType: 1,
	},
}

var FluxNetworks *NetworkTriple = &NetworkTriple{
	Mainnet: chaincfg.Params{
		Name: "mainnet",
		Net:  0x6427a1f5,

		// Address encoding magics
		PubKeyHashAddrID:        0x1C, // 7352 - starts with t1
		ScriptHashAddrID:        0xB8, // 7357 - starts with t3
		PrivateKeyID:            0x80, // 128 - WIF starts with 5/K/L
		WitnessPubKeyHashAddrID: 0x00, // Not used
		WitnessScriptHashAddrID: 0x00, // Not used

		// BIP32 hierarchical deterministic extended key magics
		HDPrivateKeyID: [4]byte{0x04, 0x88, 0xAD, 0xE4}, // xprv
		HDPublicKeyID:  [4]byte{0x04, 0x88, 0xB2, 0x1E}, // xpub

		// BIP44 coin type
		HDCoinType: 19167,
	},
	Testnet: chaincfg.Params{
		Name: "testnet3",
		Net:  0xbff91afa,

		// Address encoding magics
		PubKeyHashAddrID:        0x1D, // 7461 - starts with tm
		ScriptHashAddrID:        0x25, // 7354 - starts with t2
		PrivateKeyID:            0xEF, // 239 - WIF starts with c
		WitnessPubKeyHashAddrID: 0x00, // Not used
		WitnessScriptHashAddrID: 0x00, // Not used

		// BIP32 hierarchical deterministic extended key magics
		HDPrivateKeyID: [4]byte{0x04, 0x35, 0x83, 0x94}, // tprv
		HDPublicKeyID:  [4]byte{0x04, 0x35, 0x87, 0xCF}, // tpub

		// BIP44 coin type (testnet uses coin type 1)
		HDCoinType: 1,
	},
}
