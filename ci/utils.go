package ci

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/client"
	xclient "github.com/cordialsys/crosschain/client"
	"github.com/cordialsys/crosschain/cmd/xc/commands"
	"github.com/cordialsys/crosschain/factory"
)

var (
	chain string
	rpc   string
)

func init() {
	flag.StringVar(&chain, "chain", "", "Used Blockchain chain")
	flag.StringVar(&rpc, "rpc", "", "RPC endpoint")
}

func validateRequiredFlags(t *testing.T, value, errorMsg string) {
	if value == "" {
		t.Fatal(errorMsg)
	}
}

func fundWallet(chainConfig *xc.ChainConfig, walletAddress string) (string, error) {
	foundAmount, err := getChainSpecificFoundAmount(chainConfig)
	if err != nil {
		return "", err
	}

	host, err := getHost(chainConfig.URL)
	if err != nil {
		return "", err
	}

	faucetUrl, err := buildFaucetURL(host, chainConfig)
	if err != nil {
		return "", err
	}

	err = getTestTokensFromFaucet(faucetUrl, walletAddress, foundAmount)
	if err != nil {
		return "", err
	}

	return foundAmount, nil
}

func getChainSpecificFoundAmount(chainConfig *xc.ChainConfig) (string, error) {
	amountHuman, err := xc.NewAmountHumanReadableFromStr("1")
	if err != nil {
		return "", err
	}

	decimals := chainConfig.GetDecimals()
	amountBlockchain := amountHuman.ToBlockchain(decimals)

	return amountBlockchain.String(), nil
}

func getHost(inputURL string) (string, error) {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	host := parsedURL.Hostname()
	return host, nil
}

func buildFaucetURL(host string, chainConfig *xc.ChainConfig) (string, error) {
	if host == "" || chainConfig == nil {
		return "", fmt.Errorf("baseURL, chainConfig must be non-empty")
	}

	return fmt.Sprintf("http://%s:10001/chains/%s/assets/%s", host, chainConfig.Chain, chainConfig.Chain), nil
}

func getTestTokensFromFaucet(faucetUrl string, walletAddress string, amount string) error {
	requestBody, err := json.Marshal(map[string]string{
		"amount":  amount,
		"address": walletAddress,
	})
	if err != nil {
		return fmt.Errorf("error creating request body: %v", err)
	}

	req, err := http.NewRequest("POST", faucetUrl, bytes.NewBuffer(requestBody))
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("Response: %s\n", string(body))
	return nil
}

func getTxTransfer(xcFactory *factory.Factory, chainConfig *xc.ChainConfig, client xclient.Client, fromPrivateKey string, toPrivateKey string) (string, error) {
	toWalletAddress, err := commands.DeriveAddress(xcFactory, chainConfig, toPrivateKey)
	if err != nil {
		return "", err
	}

	amountToTransfer := "0.001"
	timeout := time.Duration(60000000000)
	decimals := chainConfig.GetDecimals()

	txTransfer, err := commands.RetrieveTxTransfer(xcFactory, chainConfig, "", "", timeout, toWalletAddress, amountToTransfer, decimals, fromPrivateKey, client)
	if err != nil {
		return "", err
	}

	return string(txTransfer), nil
}

func computeBalanceAfterTransfer(initialBanalceStr string, parsedTx *client.TxInfo) (string, error) {
	initialBallance, err := xc.NewAmountHumanReadableFromStr(initialBanalceStr)
	if err != nil {
		return "", err
	}

	transferredAmount, err := xc.NewAmountHumanReadableFromStr(parsedTx.Movements[0].From[0].Balance.String())
	if err != nil {
		return "", err
	}

	transactionFee, err := xc.NewAmountHumanReadableFromStr(parsedTx.Fees[0].Balance.String())
	if err != nil {
		return "", err
	}

	return initialBallance.Decimal().Sub(transferredAmount.Decimal().Add(transactionFee.Decimal())).String(), nil
}

func parseTxTransaction(data string) (*client.TxInfo, error) {
	var tx client.TxInfo

	err := json.Unmarshal([]byte(data), &tx)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON: %v", err)
	}

	return &tx, nil
}
