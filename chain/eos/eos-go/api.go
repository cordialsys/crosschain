package eos

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cordialsys/crosschain/chain/eos/eos-go/ecc"
	"github.com/sirupsen/logrus"
)

type API struct {
	HttpClient *http.Client
	BaseURL    string
	Compress   CompressionType
	// Header is one or more headers to be added to all outgoing calls
	Header                  http.Header
	DefaultMaxCPUUsageMS    uint8
	DefaultMaxNetUsageWords uint32 // in 8-bytes words

	lastGetInfo      *InfoResp
	lastGetInfoStamp time.Time
	lastGetInfoLock  sync.Mutex

	customGetRequiredKeys     func(ctx context.Context, tx *Transaction) ([]ecc.PublicKey, error)
	enablePartialRequiredKeys bool
}

func New(baseURL string, timeout time.Duration) *API {
	api := &API{
		HttpClient: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   timeout,
					KeepAlive: 30 * time.Second,
					DualStack: true,
				}).DialContext,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				DisableKeepAlives:     true, // default behavior, because of `nodeos`'s lack of support for Keep alives.
			},
			Timeout: timeout,
		},
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Compress: CompressionZlib,
		Header:   make(http.Header),
	}

	return api
}

func (api *API) EnableKeepAlives() bool {
	if tr, ok := api.HttpClient.Transport.(*http.Transport); ok {
		tr.DisableKeepAlives = false
		return true
	}
	return false
}

func (api *API) SetCustomGetRequiredKeys(f func(ctx context.Context, tx *Transaction) ([]ecc.PublicKey, error)) {
	api.customGetRequiredKeys = f
}

func (api *API) UsePartialRequiredKeys() {
	api.enablePartialRequiredKeys = true
}

// ProducerPause will pause block production on a nodeos with
// `producer_api` plugin loaded.
func (api *API) ProducerPause(ctx context.Context) error {
	return api.callv1(ctx, "producer", "pause", nil, nil)
}

// CreateSnapshot will write a snapshot file on a nodeos with
// `producer_api` plugin loaded.
func (api *API) CreateSnapshot(ctx context.Context) (out *CreateSnapshotResp, err error) {
	err = api.callv1(ctx, "producer", "create_snapshot", nil, &out)
	return
}

// GetIntegrityHash will produce a hash corresponding to current
// state. Requires `producer_api` and useful when loading
// from a snapshot
func (api *API) GetIntegrityHash(ctx context.Context) (out *GetIntegrityHashResp, err error) {
	err = api.callv1(ctx, "producer", "get_integrity_hash", nil, &out)
	return
}

// ProducerResume will resume block production on a nodeos with
// `producer_api` plugin loaded. Obviously, this needs to be a
// producing node on the producers schedule for it to do anything.
func (api *API) ProducerResume(ctx context.Context) error {
	return api.callv1(ctx, "producer", "resume", nil, nil)
}

// IsProducerPaused queries the blockchain for the pause statement of
// block production.
func (api *API) IsProducerPaused(ctx context.Context) (out bool, err error) {
	err = api.callv1(ctx, "producer", "paused", nil, &out)
	return
}

func (api *API) GetProducerProtocolFeatures(ctx context.Context) (out []ProtocolFeature, err error) {
	err = api.callv1(ctx, "producer", "get_supported_protocol_features", nil, &out)
	return
}

func (api *API) ScheduleProducerProtocolFeatureActivations(ctx context.Context, protocolFeaturesToActivate []Checksum256) error {
	return api.callv1(ctx, "producer", "schedule_protocol_feature_activations", M{"protocol_features_to_activate": protocolFeaturesToActivate}, nil)
}

type GetAccountOption interface {
	apply(body M)
}

type getAccountOptionFunc func(body M)

func (f getAccountOptionFunc) apply(body M) {
	f(body)
}

func WithCoreSymbol(symbol Symbol) GetAccountOption {
	return getAccountOptionFunc(func(body M) {
		body["expected_core_symbol"] = symbol.String()
	})
}

func (api *API) GetAccount(ctx context.Context, name AccountName, opts ...GetAccountOption) (out *AccountResp, err error) {
	body := M{"account_name": name}
	for _, opt := range opts {
		opt.apply(body)
	}

	err = api.callv1(ctx, "chain", "get_account", body, &out)
	return
}

func (api *API) GetRawCodeAndABI(ctx context.Context, account AccountName) (out *GetRawCodeAndABIResp, err error) {
	err = api.callv1(ctx, "chain", "get_raw_code_and_abi", M{"account_name": account}, &out)
	return
}

func (api *API) GetCode(ctx context.Context, account AccountName) (out *GetCodeResp, err error) {
	err = api.callv1(ctx, "chain", "get_code", M{"account_name": account, "code_as_wasm": true}, &out)
	return
}

func (api *API) GetCodeHash(ctx context.Context, account AccountName) (out Checksum256, err error) {
	resp := GetCodeHashResp{}
	if err = api.callv1(ctx, "chain", "get_code_hash", M{"account_name": account}, &resp); err != nil {
		return
	}

	buffer, err := hex.DecodeString(resp.CodeHash)
	return Checksum256(buffer), err
}

// PushTransaction submits a properly filled (tapos), packed and
// signed transaction to the blockchain.
func (api *API) PushTransaction(ctx context.Context, tx *PackedTransaction) (out *PushTransactionFullResp, err error) {
	err = api.callv1(ctx, "chain", "push_transaction", tx, &out)
	return
}

func (api *API) PushRawTransaction(ctx context.Context, tx json.RawMessage) (out *PushTransactionFullResp, err error) {
	err = api.callv1(ctx, "chain", "push_transaction", tx, &out)
	return
}

func (api *API) PushRawTransactionRaw(ctx context.Context, tx json.RawMessage) (out json.RawMessage, err error) {
	err = api.callv1(ctx, "chain", "push_transaction", tx, &out)
	return
}

func (api *API) SendTransaction(ctx context.Context, tx *PackedTransaction) (out *PushTransactionFullResp, err error) {
	err = api.callv1(ctx, "chain", "send_transaction", tx, &out)
	return
}

func (api *API) PushTransactionRaw(ctx context.Context, tx *PackedTransaction) (out json.RawMessage, err error) {
	err = api.callv1(ctx, "chain", "push_transaction", tx, &out)
	return
}
func (api *API) SendTransactionRaw(ctx context.Context, tx *PackedTransaction) (out json.RawMessage, err error) {
	err = api.callv1(ctx, "chain", "send_transaction", tx, &out)
	return
}

func (api *API) GetInfo(ctx context.Context) (out *InfoResp, err error) {
	err = api.callv1(ctx, "chain", "get_info", nil, &out)
	return
}

func (api *API) GetNetConnections(ctx context.Context) (out []*NetConnectionsResp, err error) {
	err = api.callv1(ctx, "net", "connections", nil, &out)
	return
}

func (api *API) NetConnect(ctx context.Context, host string) (out NetConnectResp, err error) {
	err = api.callv1(ctx, "net", "connect", host, &out)
	return
}

func (api *API) NetDisconnect(ctx context.Context, host string) (out NetDisconnectResp, err error) {
	err = api.callv1(ctx, "net", "disconnect", host, &out)
	return
}

func (api *API) GetNetStatus(ctx context.Context, host string) (out *NetStatusResp, err error) {
	err = api.callv1(ctx, "net", "status", M{"host": host}, &out)
	return
}

func (api *API) GetBlockByID(ctx context.Context, id string) (out *BlockResp, err error) {
	err = api.callv1(ctx, "chain", "get_block", M{"block_num_or_id": id}, &out)
	return
}

// GetScheduledTransactionsWithBounds returns scheduled transactions within specified bounds
func (api *API) GetScheduledTransactionsWithBounds(ctx context.Context, lower_bound string, limit uint32) (out *ScheduledTransactionsResp, err error) {
	err = api.callv1(ctx, "chain", "get_scheduled_transactions", M{"json": true, "lower_bound": lower_bound, "limit": limit}, &out)
	return
}

// GetScheduledTransactions returns the Top 100 scheduled transactions
func (api *API) GetScheduledTransactions(ctx context.Context) (out *ScheduledTransactionsResp, err error) {
	return api.GetScheduledTransactionsWithBounds(ctx, "", 100)
}

func (api *API) GetProducers(ctx context.Context) (out *ProducersResp, err error) {
	/*
		+FC_REFLECT( eosio::chain_apis::read_only::get_producers_params, (json)(lower_bound)(limit) )
		+FC_REFLECT( eosio::chain_apis::read_only::get_producers_result, (rows)(total_producer_vote_weight)(more) ); */
	err = api.callv1(ctx, "chain", "get_producers", M{"json": true}, &out)
	return
}

func (api *API) GetBlockByNum(ctx context.Context, num uint32) (out *BlockResp, err error) {
	err = api.callv1(ctx, "chain", "get_block", M{"block_num_or_id": fmt.Sprintf("%d", num)}, &out)
	//err = api.call("chain", "get_block", M{"block_num_or_id": num}, &out)
	return
}

func (api *API) GetBlockByNumOrID(ctx context.Context, query string) (out *SignedBlock, err error) {
	err = api.callv1(ctx, "chain", "get_block", M{"block_num_or_id": query}, &out)
	return
}

func (api *API) GetBlockByNumOrIDRaw(ctx context.Context, query string) (out interface{}, err error) {
	err = api.callv1(ctx, "chain", "get_block", M{"block_num_or_id": query}, &out)
	return
}

func (api *API) GetDBSize(ctx context.Context) (out *DBSizeResp, err error) {
	err = api.callv1(ctx, "db_size", "get", nil, &out)
	return
}

func (api *API) GetTransaction(ctx context.Context, id string) (out *TransactionResp, err error) {
	err = api.callv1(ctx, "history", "get_transaction", M{"id": id}, &out)
	return
}

func (api *API) GetTransactionV1(ctx context.Context, id string) (out *TransactionResp, err error) {
	err = api.callv1(ctx, "history", "get_transaction", M{"id": id}, &out)
	return
}

func (api *API) GetTransactionRaw(ctx context.Context, id string) (out json.RawMessage, err error) {
	err = api.callv1(ctx, "history", "get_transaction", M{"id": id}, &out)
	return
}

func (api *API) GetActions(ctx context.Context, params GetActionsRequest) (out *ActionsResp, err error) {
	err = api.callv1(ctx, "history", "get_actions", params, &out)
	return
}

func (api *API) GetKeyAccounts(ctx context.Context, publicKey string) (out *KeyAccountsResp, err error) {
	err = api.callv1(ctx, "history", "get_key_accounts", M{"public_key": publicKey}, &out)
	return
}

func (api *API) GetControlledAccounts(ctx context.Context, controllingAccount string) (out *ControlledAccountsResp, err error) {
	err = api.callv1(ctx, "history", "get_controlled_accounts", M{"controlling_account": controllingAccount}, &out)
	return
}

func (api *API) GetTransactions(ctx context.Context, name AccountName) (out *TransactionsResp, err error) {
	err = api.callv1(ctx, "account_history", "get_transactions", M{"account_name": name}, &out)
	return
}

func (api *API) GetTableByScope(ctx context.Context, params GetTableByScopeRequest) (out *GetTableByScopeResp, err error) {
	err = api.callv1(ctx, "chain", "get_table_by_scope", params, &out)
	return
}

func (api *API) GetTableRows(ctx context.Context, params GetTableRowsRequest) (out *GetTableRowsResp, err error) {
	err = api.callv1(ctx, "chain", "get_table_rows", params, &out)
	return
}

func (api *API) GetRawABI(ctx context.Context, params GetRawABIRequest) (out *GetRawABIResp, err error) {
	err = api.callv1(ctx, "chain", "get_raw_abi", params, &out)
	return
}

func (api *API) GetAccountsByAuthorizers(ctx context.Context, authorizations []PermissionLevel, keys []string) (out *GetAccountsByAuthorizersResp, err error) {
	err = api.callv1(ctx, "chain", "get_accounts_by_authorizers", M{"accounts": authorizations, "keys": keys}, &out)
	return
}

func (api *API) GetCurrencyBalance(ctx context.Context, account AccountName, symbol string, code AccountName) (out []Asset, err error) {
	params := M{"account": account, "code": code}
	if symbol != "" {
		params["symbol"] = symbol
	}
	err = api.callv1(ctx, "chain", "get_currency_balance", params, &out)
	return
}

func (api *API) GetCurrencyStats(ctx context.Context, code AccountName, symbol string) (out *GetCurrencyStatsResp, err error) {
	params := M{"code": code, "symbol": symbol}

	outWrapper := make(map[string]*GetCurrencyStatsResp)
	err = api.callv1(ctx, "chain", "get_currency_stats", params, &outWrapper)
	out = outWrapper[symbol]

	return
}

func (api *API) callv1(ctx context.Context, baseAPI string, endpoint string, body interface{}, out interface{}) error {
	return api.call(ctx, "POST", baseAPI, "v1", endpoint, body, out)
}

func (api *API) callv2(ctx context.Context, method string, baseAPI string, endpoint string, body interface{}, out interface{}) error {
	return api.call(ctx, method, baseAPI, "v2", endpoint, body, out)
}

func (api *API) call(ctx context.Context, method string, baseAPI string, version string, endpoint string, body interface{}, out interface{}) error {

	targetURL := fmt.Sprintf("%s/%s/%s/%s", api.BaseURL, version, baseAPI, endpoint)
	var req *http.Request
	var err error
	var jsonBodyBz []byte
	if body != nil {
		jsonBodyRef, err := enc(body)
		if err != nil {
			return err
		}
		jsonBodyBz, _ = io.ReadAll(jsonBodyRef)
	}

	if body != nil && method != "GET" {
		var jsonBody io.Reader
		jsonBody, err = enc(body)
		if err != nil {
			return err
		}
		req, err = http.NewRequest(method, targetURL, jsonBody)
	} else {
		if body != nil {
			bodyMap := map[string]interface{}{}
			bodyBz, _ := json.Marshal(body)
			_ = json.Unmarshal(bodyBz, &bodyMap)

			query := url.Values{}
			for k, v := range bodyMap {
				query.Add(k, fmt.Sprintf("%v", v))
			}
			targetURL = fmt.Sprintf("%s?%s", targetURL, query.Encode())
		}
		req, err = http.NewRequest(method, targetURL, nil)
	}
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	for k, v := range api.Header {
		if req.Header == nil {
			req.Header = http.Header{}
		}
		req.Header[k] = append(req.Header[k], v...)
	}
	log := logrus.WithFields(logrus.Fields{
		"method": method,
		"url":    req.URL.String(),
		"body":   string(jsonBodyBz),
	})

	log.Debug("request")

	resp, err := api.HttpClient.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("%s: %w", req.URL.String(), err)
	}
	defer resp.Body.Close()

	var cnt bytes.Buffer
	_, err = io.Copy(&cnt, resp.Body)
	if err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	if resp.StatusCode == 404 {
		var apiErr APIError
		if err := json.Unmarshal(cnt.Bytes(), &apiErr); err != nil {
			return ErrNotFound
		}
		return apiErr
	}

	if resp.StatusCode > 299 {
		var apiErr APIError
		if err := json.Unmarshal(cnt.Bytes(), &apiErr); err != nil {
			return fmt.Errorf("%s: status code=%d, body=%s", req.URL.String(), resp.StatusCode, cnt.String())
		}

		// Handle cases where some API calls (/v1/chain/get_account for example) returns a 500
		// error when retrieving data that does not exist.
		if apiErr.IsUnknownKeyError() {
			return ErrNotFound
		}

		return apiErr
	}

	log.WithFields(logrus.Fields{
		"status": resp.StatusCode,
		"body":   cnt.String(),
	}).Debug("response")

	if err := json.Unmarshal(cnt.Bytes(), &out); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	return nil
}

var ErrNotFound = errors.New("resource not found")

type M map[string]interface{}

func enc(v interface{}) (io.Reader, error) {
	if v == nil {
		return nil, nil
	}

	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)

	err := encoder.Encode(v)
	if err != nil {
		return nil, err
	}

	return buffer, nil
}
