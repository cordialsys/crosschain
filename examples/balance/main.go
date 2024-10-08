package main

import (
	"context"
	"fmt"

	"github.com/cordialsys/crosschain/config"
	"github.com/cordialsys/crosschain/factory"
)

func getBalanceSui(ctx context.Context, xc *factory.Factory) {
	asset, err := xc.GetAssetConfig("", "SUI")
	if err != nil {
		panic("unsupported asset")
	}
	address := xc.MustAddress(asset, "0x7d20dcdb2bca4f508ea9613994683eb4e76e9c4ed371169677c1be02aaf0b58e")
	client, _ := xc.NewClient(asset)
	balance, err := client.FetchBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Sui machine: %s\n", balance)
	humanBalance, _ := xc.ConvertAmountToHuman(asset, balance)
	fmt.Printf("Sui: %s\n", humanBalance)
}

func getBalanceAptos(ctx context.Context, xc *factory.Factory) {
	asset, err := xc.GetAssetConfig("", "APTOS")
	if err != nil {
		panic("unsupported asset")
	}
	address := xc.MustAddress(asset, "0xa589a80d61ec380c24a5fdda109c3848c082584e6cb725e5ab19b18354b2ab85")
	client, _ := xc.NewClient(asset)
	balance, err := client.FetchBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("APTOS octas: %s\n", balance)
	humanBalance, _ := xc.ConvertAmountToHuman(asset, balance)
	fmt.Printf("APTOS: %s\n", humanBalance)
}

func getBalanceInjective(ctx context.Context, xc *factory.Factory) {
	asset, err := xc.GetAssetConfig("", "INJ")
	if err != nil {
		panic("unsupported asset")
	}
	fmt.Println("rpc-url: ", asset.GetChain().URL)
	address := xc.MustAddress(asset, "inj1akxycslq8cjt0uffw4rjmfm3echchptu52a2dq")
	client, _ := xc.NewClient(asset)
	balance, err := client.FetchBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("uinj: %s\n", balance)
	humanBalance, _ := xc.ConvertAmountToHuman(asset, balance)
	fmt.Printf("inj: %s\n", humanBalance)

	token, err := xc.GetAssetConfig("ETH", "INJ")
	if err != nil {
		panic(err)
	}
	client, _ = xc.NewClient(token)
	balance, err = client.FetchBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	humanBalance, _ = xc.ConvertAmountToHuman(token, balance)
	fmt.Printf("INJ peggy asset: %s\n", humanBalance)
}

func getBalanceEthereum(ctx context.Context, xc *factory.Factory) {
	asset, err := xc.GetAssetConfig("", "ETH")
	if err != nil {
		panic("unsupported asset")
	}
	address := xc.MustAddress(asset, "0x0eC9f48533bb2A03F53F341EF5cc1B057892B10B")
	client, _ := xc.NewClient(asset)
	balance, err := client.FetchBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("gwei: %s\n", balance)
	humanBalance, _ := xc.ConvertAmountToHuman(asset, balance)
	fmt.Printf("ETH: %s\n", humanBalance)

	token, err := xc.GetAssetConfig("USDC", "ETH")
	if err != nil {
		panic(err)
	}
	client, _ = xc.NewClient(token)
	balance, err = client.FetchBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	humanBalance, _ = xc.ConvertAmountToHuman(token, balance)
	fmt.Printf("USDC.ETH: %s\n", humanBalance)
}

func getBalanceBitcoin(ctx context.Context, xc *factory.Factory) {
	asset, err := xc.GetAssetConfig("", "BTC")
	if err != nil {
		panic("unsupported asset")
	}
	address := xc.MustAddress(asset, "tb1q3ayl6ml4054lr2snhm5qx8mv6epulnqhv6k2kk")
	client, _ := xc.NewClient(asset)
	balance, err := client.FetchBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("satoshi: %s\n", balance)
	humanBalance, _ := xc.ConvertAmountToHuman(asset, balance)
	fmt.Printf("BTC: %s\n", humanBalance)
}

func getBalancePolkadot(ctx context.Context, xc *factory.Factory) {
	asset, err := xc.GetAssetConfig("", "DOT")
	if err != nil {
		panic("unsupported asset")
	}
	address := xc.MustAddress(asset, "1598AR2pgoJCWHn3UA2FTemJ74hBWgp7GLyNB4oSkt6vqMno")
	client, _ := xc.NewClient(asset)
	balance, err := client.FetchNativeBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Planck: %s\n", balance)
	humanBalance, _ := xc.ConvertAmountToHuman(asset, balance)
	fmt.Printf("DOT: %s\n", humanBalance)
}

func getBalanceTron(ctx context.Context, xc *factory.Factory) {
	asset, err := xc.GetAssetConfig("", "TRX")
	if err != nil {
		panic("unsupported asset")
	}
	address := xc.MustAddress(asset, "TFzvbsVRAYDtPGc4KdFNUZophTDxfKD3G1")
	client, _ := xc.NewClient(asset)
	balance, err := client.FetchNativeBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("TRX: %s\n", balance)
	humanBalance, _ := xc.ConvertAmountToHuman(asset, balance)
	fmt.Printf("TRX (human): %s\n", humanBalance)

	token, err := xc.GetAssetConfig("USDT", "TRX")
	if err != nil {
		panic("unsupported asset")
	}
	client, _ = xc.NewClient(token)
	balance, err = client.FetchBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("USDT on TRX: %s\n", balance)
	humanBalance, _ = xc.ConvertAmountToHuman(token, balance)
	fmt.Printf("USDT on TRX (human): %s\n", humanBalance)
}

func getBalanceSolana(ctx context.Context, xc *factory.Factory) {
	asset, err := xc.GetAssetConfig("", "SOL")
	if err != nil {
		panic("unsupported asset")
	}
	address := xc.MustAddress(asset, "44UY77prgrQmf4sbD9pyyk3dqRQPMM32QQv4YTHjLReh")
	client, _ := xc.NewClient(asset)
	balance, err := client.FetchBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("lamports: %s\n", balance)
	humanBalance, _ := xc.ConvertAmountToHuman(asset, balance)
	fmt.Printf("SOL: %s\n", humanBalance)

	token, err := xc.GetAssetConfig("USDC", "SOL")
	if err != nil {
		panic(err)
	}
	client, _ = xc.NewClient(token)
	balance, err = client.FetchBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	humanBalance, _ = xc.ConvertAmountToHuman(token, balance)
	fmt.Printf("USDC.SOL: %s\n", humanBalance)
}

func getBalanceLunc(ctx context.Context, xc *factory.Factory) {
	asset, err := xc.GetAssetConfig("", "LUNC")
	if err != nil {
		panic("unsupported asset")
	}
	fmt.Println("rpc-url: ", asset.GetChain().URL)

	address := xc.MustAddress(asset, "terra12tqxwj5vupwdpdcd302emhpn7gg3jypsr6u30m")
	client, _ := xc.NewClient(asset)
	balance, err := client.FetchBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("balance native raw: %s\n", balance.String())

	token, err := xc.GetAssetConfig("ETH", "LUNC")
	if err != nil {
		panic(err)
	}
	client, _ = xc.NewClient(token)
	balance, err = client.FetchBalance(ctx, address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("cw20 balance: %s\n", balance.String())
}

func main() {
	config.ConfigureLogger("debug")
	// initialize crosschain
	xc := factory.NewDefaultFactory()
	ctx := context.Background()
	// getBalanceSui(ctx, xc)
	// getBalanceAptos(ctx, xc)
	getBalanceInjective(ctx, xc)
	// getBalanceEthereum(ctx, xc)
	// getBalanceBitcoin(ctx, xc)
	// getBalancePolkadot(ctx, xc)
	//getBalanceSolana(ctx, xc)
	// getBalanceLunc(ctx, xc)
}
