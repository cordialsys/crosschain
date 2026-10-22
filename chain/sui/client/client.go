// Package client implements Sui's gRPC API while preserving the sui transaction
// input format consumed by existing offline builders. Staking discovery still
// uses suix_getStakes on SecondaryURL (or URL when SecondaryURL is unset).
package client

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/sui"
	pb "github.com/cordialsys/crosschain/chain/sui/client/rpc"
	xclient "github.com/cordialsys/crosschain/client"
	legacy "github.com/cordialsys/go-sui-sdk/v2/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type Client struct {
	Asset     *xc.ChainConfig
	Ledger    pb.LedgerServiceClient
	State     pb.StateServiceClient
	Execution pb.TransactionExecutionServiceClient
	SuiClient *legacy.Client // Only used for suix_getStakes.
	conn      *grpc.ClientConn
}

var _ xclient.Client = (*Client)(nil)
var _ xclient.StakingClient = (*Client)(nil)

// NewClient connects to cfg.URL over TLS by default; http:// explicitly selects
// plaintext. Extra dial options can supply provider-specific credentials.
func NewClient(cfg *xc.ChainConfig, options ...grpc.DialOption) (*Client, error) {
	endpoint, err := parseEndpoint(cfg.URL)
	if err != nil {
		return nil, err
	}
	var creds credentials.TransportCredentials = credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	if !endpoint.secure {
		creds = insecure.NewCredentials()
	}
	token := ""
	if cfg.Auth2 != "" {
		token, err = cfg.Auth2.Load()
		if err != nil {
			return nil, fmt.Errorf("load Sui RPC authentication: %w", err)
		}
	}
	timeout := cfg.DefaultHttpClient().Timeout
	interceptor := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ctx, method = endpoint.request(ctx, method, token)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithChainUnaryInterceptor(interceptor),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(32 * 1024 * 1024)),
	}
	conn, err := grpc.NewClient(endpoint.target, append(dialOptions, options...)...)
	if err != nil {
		return nil, fmt.Errorf("create Sui gRPC client: %w", err)
	}
	stakingURL := cfg.SecondaryURL
	if stakingURL == "" {
		stakingURL = endpoint.httpURL
	}
	staking, err := legacy.DialWithClient(stakingURL, cfg.DefaultHttpClient())
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &Client{
		Asset:     cfg,
		Ledger:    pb.NewLedgerServiceClient(conn),
		State:     pb.NewStateServiceClient(conn),
		Execution: pb.NewTransactionExecutionServiceClient(conn),
		SuiClient: staking,
		conn:      conn,
	}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

// rpcEndpoint separates the dial address from HTTP routing/authentication.
type rpcEndpoint struct {
	target, httpURL  string
	secure           bool
	path, query      string
	quicknode        bool
	token, basicAuth string
}

func parseEndpoint(raw string) (*rpcEndpoint, error) {
	if raw == "" {
		return nil, fmt.Errorf("Sui gRPC URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid Sui gRPC URL")
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.Fragment != "" {
		return nil, fmt.Errorf("Sui gRPC URL must use http(s) and include a host, without a fragment")
	}
	e := &rpcEndpoint{httpURL: u.String(), secure: u.Scheme == "https", path: strings.TrimRight(u.EscapedPath(), "/"), query: u.RawQuery}
	if u.User != nil {
		password, _ := u.User.Password()
		e.basicAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(u.User.Username()+":"+password))
	}
	host := strings.ToLower(u.Hostname())
	e.quicknode = strings.HasSuffix(host, ".sui-mainnet.quiknode.pro") || strings.HasSuffix(host, ".sui-testnet.quiknode.pro") || strings.HasSuffix(host, ".sui-devnet.quiknode.pro")
	port := u.Port()
	if e.quicknode {
		// QuickNode HTTP URLs carry a token in the path. Native gRPC uses port
		// 9000 and x-token instead: quicknode.com/docs/sui/sui-grpc/overview/go.
		e.token = strings.Trim(u.Path, "/")
		if strings.Contains(e.token, "/") || e.query != "" {
			return nil, fmt.Errorf("QuickNode Sui URL must contain a single token path and no query")
		}
		e.path = ""
		if port == "" || port == "443" {
			port = "9000"
		}
	}
	if port == "" {
		port = "443"
		if !e.secure {
			port = "80"
		}
	}
	e.target = "dns:///" + net.JoinHostPort(u.Hostname(), port)
	return e, nil
}

func (e *rpcEndpoint) request(ctx context.Context, method, configuredToken string) (context.Context, string) {
	token := configuredToken
	if e.quicknode {
		if token == "" {
			token = e.token
		}
		if token != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "x-token", token)
		}
	} else if token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}
	if e.basicAuth != "" && token == "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", e.basicAuth)
	}
	method = e.path + method
	if e.query != "" {
		method += "?" + e.query
	}
	return ctx, method
}

func mask(paths ...string) *fieldmaskpb.FieldMask { return &fieldmaskpb.FieldMask{Paths: paths} }

func (c *Client) nativeCoin() string {
	if c.Asset.ChainCoin != "" {
		return sui.NormalizeCoinContract(c.Asset.ChainCoin)
	}
	return sui.NativeCoin
}

func (c *Client) FetchBalance(ctx context.Context, args *xclient.BalanceArgs) (xc.AmountBlockchain, error) {
	coin := c.nativeCoin()
	if contract, ok := args.Contract(); ok {
		coin = string(contract)
	}
	return c.FetchBalanceFor(ctx, args.Address(), coin)
}

func (c *Client) FetchBalanceFor(ctx context.Context, address xc.Address, coin string) (xc.AmountBlockchain, error) {
	resp, err := c.State.GetBalance(ctx, &pb.GetBalanceRequest{Owner: proto.String(string(address)), CoinType: proto.String(sui.NormalizeCoinContract(coin))})
	if err != nil {
		return xc.AmountBlockchain{}, err
	}
	if resp.GetBalance() == nil || resp.Balance.Balance == nil {
		return xc.AmountBlockchain{}, fmt.Errorf("Sui returned no balance")
	}
	return xc.NewAmountBlockchainFromUint64(resp.Balance.GetBalance()), nil
}

func (c *Client) FetchDecimals(ctx context.Context, contract xc.ContractAddress) (int, error) {
	if contract == "" || c.Asset.IsChain(contract) || sameCoin(string(contract), c.nativeCoin()) {
		return int(c.Asset.Decimals), nil
	}
	resp, err := c.State.GetCoinInfo(ctx, &pb.GetCoinInfoRequest{CoinType: proto.String(sui.NormalizeCoinContract(string(contract)))})
	if err != nil {
		return 0, err
	}
	if resp.GetMetadata() == nil || resp.Metadata.Decimals == nil {
		return 0, fmt.Errorf("Sui returned no coin metadata for %s", contract)
	}
	return int(resp.Metadata.GetDecimals()), nil
}

// Sui returns fully padded package addresses whereas older inputs use 0x2.
func sameCoin(a, b string) bool {
	normalize := func(s string) string {
		parts := strings.SplitN(sui.NormalizeCoinContract(s), "::", 2)
		if len(parts) != 2 {
			return s
		}
		return strings.TrimLeft(strings.TrimPrefix(parts[0], "0x"), "0") + "::" + parts[1]
	}
	return normalize(a) == normalize(b)
}
