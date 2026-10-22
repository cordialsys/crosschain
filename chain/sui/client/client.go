// Package client implements Sui's gRPC API while preserving the sui transaction
// input format consumed by existing offline builders. Staking discovery still
// uses suix_getStakes on SecondaryURL (or URL when SecondaryURL is unset).
package client

import (
	"context"
	"crypto/tls"
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
	target, httpURL, secure, err := endpoint(cfg.URL)
	if err != nil {
		return nil, err
	}
	var creds credentials.TransportCredentials = credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	if !secure {
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
		if token != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithChainUnaryInterceptor(interceptor),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(32 * 1024 * 1024)),
	}
	conn, err := grpc.NewClient(target, append(dialOptions, options...)...)
	if err != nil {
		return nil, fmt.Errorf("create Sui gRPC client: %w", err)
	}
	stakingURL := cfg.SecondaryURL
	if stakingURL == "" {
		stakingURL = httpURL
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

func endpoint(raw string) (target, httpURL string, secure bool, err error) {
	if raw == "" {
		return "", "", false, fmt.Errorf("Sui gRPC URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", false, fmt.Errorf("invalid Sui gRPC URL")
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", "", false, fmt.Errorf("Sui gRPC URL must be an http(s) host and port without a path, query, or user credentials")
	}
	port := u.Port()
	if port == "" {
		port = "443"
		if u.Scheme == "http" {
			port = "80"
		}
	}
	return "dns:///" + net.JoinHostPort(u.Hostname(), port), strings.TrimRight(u.String(), "/"), u.Scheme == "https", nil
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
