//go:build !not_ci

package ci

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"testing"
	"time"

	xcclient "github.com/cordialsys/crosschain/client"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

var (
	chain         string
	rpc           string
	network       string
	algorithm     string
	contract      string
	decimalsStr   string
	feePayer      bool
	decimalsInput *int
)

func init() {
	flag.StringVar(&chain, "chain", "", "Used Blockchain chain")
	flag.StringVar(&rpc, "rpc", "", "RPC endpoint")
	flag.StringVar(&network, "network", "", "Bitcoin network, if relevant")
	flag.StringVar(&algorithm, "algorithm", "", "Used to override signature algorithm. Bitcoin only")
	flag.StringVar(&contract, "contract", "", "Contract address for token")
	flag.StringVar(&decimalsStr, "decimals", "", "Decimals used for token")
	flag.BoolVar(&feePayer, "fee-payer", false, "Use fee payer for transactions")

	logrus.SetLevel(logrus.DebugLevel)
}

func validateCLIInputs(t *testing.T) {
	if chain == "" {
		t.Fatal("--chain is required")
	}
	if rpc == "" {
		t.Fatal("--rpc is required")
	}
	if decimalsStr != "" {
		asInt, err := strconv.Atoi(decimalsStr)
		if err != nil {
			panic(err)
		}
		decimalsInput = &asInt
	}
}

func fundWallet(t *testing.T, chainConfig *xc.ChainConfig, walletAddress xc.Address, amount string, contractMaybe string, decimals int32) {
	require.NotNil(t, chainConfig)

	amountHuman, err := xc.NewAmountHumanReadableFromStr(amount)
	require.NoError(t, err)
	amountBlockchain := amountHuman.ToBlockchain(int32(decimals))

	fmt.Printf("funding wallet %s with %s %s\n", walletAddress, amountBlockchain, contractMaybe)

	// The RPC host is the same as the faucet host
	parsedURL, err := url.Parse(chainConfig.URL)
	if err != nil {
		panic(err)
	}

	host := parsedURL.Hostname()
	require.NotEmpty(t, host)

	assetId := string(chainConfig.Chain)
	if contractMaybe != "" {
		assetId = contractMaybe
	}

	faucetUrl := fmt.Sprintf("http://%s:10001/chains/%s/assets/%s", host, chainConfig.Chain, assetId)
	require.NoError(t, err)

	err = getTestTokensFromFaucet(faucetUrl, walletAddress, amountBlockchain)
	require.NoError(t, err)

}

func asJson(data any) string {
	bz, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(bz)
}

func getTestTokensFromFaucet(faucetUrl string, walletAddress xc.Address, amount xc.AmountBlockchain) error {
	requestBody := map[string]interface{}{
		"amount":  amount.String(),
		"address": walletAddress,
	}
	requestBodyBz, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("error creating request body: %v", err)
	}
	fmt.Println("POST ", faucetUrl)
	fmt.Println(asJson(requestBody))

	req, err := http.NewRequest("POST", faucetUrl, bytes.NewBuffer(requestBodyBz))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("Response: %s\n", string(body))
	return nil
}

func deriveAddress(t *testing.T, xcFactory *factory.Factory, chainConfig *xc.ChainConfig, privateKey string) xc.Address {
	signer, err := xcFactory.NewSigner(chainConfig.Base(), privateKey)
	require.NoError(t, err)

	publicKey, err := signer.PublicKey()
	require.NoError(t, err)

	addressBuilder, err := xcFactory.NewAddressBuilder(chainConfig.Base())
	require.NoError(t, err)

	from, err := addressBuilder.GetAddressFromPublicKey(publicKey)
	require.NoError(t, err)

	return from
}

func fetchCombinedBalance(t *testing.T, client xcclient.Client, balanceArgs ...*xcclient.BalanceArgs) xc.AmountBlockchain {
	combinedBalance := xc.NewAmountBlockchainFromUint64(0)
	for _, balanceArg := range balanceArgs {
		balance, err := client.FetchBalance(context.Background(), balanceArg)
		require.NoError(t, err, fmt.Sprintf("Failed to fetch balance for %s", balanceArg.Address()))
		combinedBalance = combinedBalance.Add(&balance)
	}
	return combinedBalance
}

// Because we haven't been successful with getting the faucets on devnet nodes
// to be syncronous, we instead tolerate some delay in the test
func awaitBalance(t *testing.T, client xcclient.Client, expectedBalance xc.AmountBlockchain, decimals int32, balanceArgs ...*xcclient.BalanceArgs) {
	combinedBalance := xc.NewAmountBlockchainFromUint64(0)
	for attempts := range 30 {
		combinedBalance = xc.NewAmountBlockchainFromUint64(0)
		for _, balanceArg := range balanceArgs {
			balance, err := client.FetchBalance(context.Background(), balanceArg)
			require.NoError(t, err, fmt.Sprintf("Failed to fetch balance for %s on attempt %d", balanceArg.Address(), attempts))
			combinedBalance = combinedBalance.Add(&balance)
		}
		if combinedBalance.Cmp(&expectedBalance) == 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	fmt.Println("Wallet Balance after transaction:", combinedBalance.ToHuman(decimals).String())
	require.Equal(t,
		expectedBalance.ToHuman(decimals).String(),
		combinedBalance.ToHuman(decimals).String(),
		"Failed to get balance over after 30 attempts",
	)
}

// Fetch the tx, waiting for it to be confirmed and the balance to change
func awaitTx(t *testing.T, client xcclient.Client, txHash xc.TxHash, initialBalance xc.AmountBlockchain, balanceArgs ...*xcclient.BalanceArgs) xcclient.TxInfo {
	start := time.Now()
	timeout := time.Minute * 1
	for {
		if time.Since(start) > timeout {
			require.Fail(t, fmt.Sprintf("Timed out waiting %v for transactions", time.Since(start)))
		}
		time.Sleep(1 * time.Second)
		info, err := client.FetchTxInfo(context.Background(), txHash)
		if err != nil {
			fmt.Printf("could not find tx yet, trying again (%v)...\n", err)
			continue
		}
		if info.Confirmations < 1 {
			fmt.Printf("waiting for 1 confirmation...\n")
			continue
		}
		finalWalletBalance := fetchCombinedBalance(t, client, balanceArgs...)
		if finalWalletBalance.String() == initialBalance.String() {
			fmt.Printf("waiting for change in balance...\n")
			continue
		}

		fmt.Println(asJson(info))
		return info
	}
}

// We poll until we the "full" expected balance change, as sometimes
// the balance can partially update (e.g. deducts network fee first...).
func verifyBalanceChanges(t *testing.T, client xcclient.Client, txInfo xcclient.TxInfo, assetId string, initialBalance xc.AmountBlockchain, balanceArgs ...*xcclient.BalanceArgs) {
	var finalWalletBalance xc.AmountBlockchain
	var remainder xc.AmountBlockchain
	addresses := []xc.Address{}
	for _, balanceArg := range balanceArgs {
		addresses = append(addresses, balanceArg.Address())
	}

	// We poll until we the "full" expected balance change, as sometimes
	// the balance can partially update (e.g. deducts network fee first...).
	for range 50 {
		finalWalletBalance = fetchCombinedBalance(t, client, balanceArgs...)
		fmt.Printf("Balance of %v after transaction: %v\n", balanceArgs, finalWalletBalance)

		remainder = initialBalance
		for _, movement := range txInfo.Movements {
			if movement.AssetId != xc.ContractAddress(assetId) {
				// skip movements not matching the asset we transferred
				continue
			}
			for _, from := range movement.From {
				if slices.Contains(addresses, from.AddressId) {
					// subtract
					remainder = remainder.Sub(&from.Balance)
				}
			}
			for _, to := range movement.To {
				if slices.Contains(addresses, to.AddressId) {
					// add
					remainder = remainder.Add(&to.Balance)
				}
			}
		}
		if finalWalletBalance.String() == remainder.String() {
			break
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}

	require.Equal(t, finalWalletBalance.String(), remainder.String())
	require.Less(t, finalWalletBalance.Uint64(), initialBalance.Uint64())
}
