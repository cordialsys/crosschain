package client

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cordialsys/crosschain/chain/monero/crypto"
	"github.com/cordialsys/crosschain/chain/monero/tx_input"
	"github.com/sirupsen/logrus"
)

// LWSClient communicates with a monero-lws (Light Wallet Server) instance.
// It provides indexed output queries so we don't need to scan the blockchain.
type LWSClient struct {
	url    string
	http   *http.Client
	// The main (standard) address registered with the LWS
	address string
	// The private view key (hex) for authentication
	viewKey string
}

// NewLWSClient creates a new LWS client from an indexer URL.
func NewLWSClient(url string) *LWSClient {
	return &LWSClient{
		url: url,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetCredentials sets the address and view key for LWS API authentication.
func (l *LWSClient) SetCredentials(address, viewKeyHex string) {
	l.address = address
	l.viewKey = viewKeyHex
}

// post makes an HTTP POST request to the LWS endpoint.
func (l *LWSClient) post(ctx context.Context, endpoint string, body interface{}) (json.RawMessage, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", l.url+"/"+endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LWS request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("LWS %s returned %d: %s", endpoint, resp.StatusCode, string(respBody))
	}

	return json.RawMessage(respBody), nil
}

// Login registers the address with the LWS if needed.
func (l *LWSClient) Login(ctx context.Context) error {
	result, err := l.post(ctx, "login", map[string]interface{}{
		"address":           l.address,
		"view_key":          l.viewKey,
		"create_account":    true,
		"generated_locally": true,
	})
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	var resp struct {
		NewAddress bool   `json:"new_address"`
		StartHeight uint64 `json:"start_height"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return fmt.Errorf("parse login response: %w", err)
	}

	if resp.NewAddress {
		logrus.WithField("start_height", resp.StartHeight).Info("registered new address with LWS")
	}
	return nil
}

// LWSOutput represents an output from the get_unspent_outs response.
type LWSOutput struct {
	Amount      json.Number `json:"amount"`
	Index       uint16      `json:"index"`
	GlobalIndex json.Number `json:"global_index"`
	TxHash      string      `json:"tx_hash"`
	TxPubKey    string      `json:"tx_pub_key"`
	PublicKey   string      `json:"public_key"`
	Rct         string      `json:"rct"`
	Height      uint64      `json:"height"`
	Recipient   *struct {
		MajI uint32 `json:"maj_i"`
		MinI uint32 `json:"min_i"`
	} `json:"recipient,omitempty"`
	SpendKeyImages []string `json:"spend_key_images"`
}

// GetUnspentOuts fetches spendable outputs from the LWS.
func (l *LWSClient) GetUnspentOuts(ctx context.Context) ([]LWSOutput, uint64, uint64, error) {
	result, err := l.post(ctx, "get_unspent_outs", map[string]interface{}{
		"address":        l.address,
		"view_key":       l.viewKey,
		"amount":         "0",
		"mixin":          15,
		"use_dust":       true,
		"dust_threshold": "0",
	})
	if err != nil {
		return nil, 0, 0, err
	}

	var resp struct {
		Outputs    []LWSOutput `json:"outputs"`
		Amount     string      `json:"amount"`
		PerByteFee uint64      `json:"per_byte_fee"`
		FeeMask    uint64      `json:"fee_mask"`
		Fees       []uint64    `json:"fees"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, 0, 0, fmt.Errorf("parse unspent outs: %w", err)
	}

	perByteFee := resp.PerByteFee
	feeMask := resp.FeeMask

	// Filter outputs that LWS already knows are spent (have valid key image)
	var unspent []LWSOutput
	for _, out := range resp.Outputs {
		if len(out.SpendKeyImages) > 0 && len(out.SpendKeyImages[0]) == 64 {
			// LWS has a key image for this output - it's been spent
			logrus.WithField("global_index", out.GlobalIndex).Debug("LWS reports output as spent (has key image)")
			continue
		}
		unspent = append(unspent, out)
	}

	logrus.WithFields(logrus.Fields{
		"total":        len(resp.Outputs),
		"unspent":      len(unspent),
		"per_byte_fee": perByteFee,
		"fee_mask":     feeMask,
	}).Info("got unspent outputs from LWS")

	return unspent, perByteFee, feeMask, nil
}

// GetAddressInfo fetches balance info from the LWS.
type LWSAddressInfo struct {
	TotalReceived string `json:"total_received"`
	TotalSent     string `json:"total_sent"`
	LockedFunds   string `json:"locked_funds"`
	ScannedHeight uint64 `json:"scanned_height"`
	BlockchainHeight uint64 `json:"blockchain_height"`
}

func (l *LWSClient) GetAddressInfo(ctx context.Context) (*LWSAddressInfo, error) {
	result, err := l.post(ctx, "get_address_info", map[string]interface{}{
		"address":  l.address,
		"view_key": l.viewKey,
	})
	if err != nil {
		return nil, err
	}

	var resp LWSAddressInfo
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("parse address info: %w", err)
	}
	return &resp, nil
}

// ConvertLWSOutputs converts LWS outputs to the tx_input.Output format used by the builder.
func ConvertLWSOutputs(outputs []LWSOutput, privateViewKey []byte) []tx_input.Output {
	var result []tx_input.Output

	for _, out := range outputs {
		amount, _ := out.Amount.Int64()
		globalIdx, _ := out.GlobalIndex.Int64()

		// Extract commitment from rct field (first 64 hex chars = 32 bytes)
		commitment := ""
		if len(out.Rct) >= 64 {
			commitment = out.Rct[:64]
		}

		// Compute commitment mask from view key + tx pub key
		txPubKeyBytes, _ := hex.DecodeString(out.TxPubKey)
		commitmentMask := ""
		if len(txPubKeyBytes) == 32 && len(privateViewKey) == 32 {
			derivation, err := crypto.GenerateKeyDerivation(txPubKeyBytes, privateViewKey)
			if err == nil {
				scalar, _ := crypto.DerivationToScalar(derivation, uint64(out.Index))
				maskData := append([]byte(crypto.CommitmentMaskLabel), scalar...)
				commitmentMask = hex.EncodeToString(crypto.ScReduce32(crypto.Keccak256(maskData)))
			}
		}

		result = append(result, tx_input.Output{
			Amount:         uint64(amount),
			Index:          uint64(out.Index),
			TxHash:         out.TxHash,
			GlobalIndex:    uint64(globalIdx),
			PublicKey:      out.PublicKey,
			Commitment:     commitment,
			TxPubKey:       out.TxPubKey,
			CommitmentMask: commitmentMask,
		})
	}

	return result
}
