package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	xc "github.com/cordialsys/crosschain"
	pb "github.com/cordialsys/crosschain/chain/sui/client/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestProviderEndpoints(t *testing.T) {
	for _, tc := range []struct{ name, url, target, path, header, value, auth string }{
		{name: "quicknode_url", url: "https://example.sui-mainnet.quiknode.pro/test-token", target: "dns:///example.sui-mainnet.quiknode.pro:9000", header: "x-token", value: "test-token"},
		{name: "quicknode_https_port", url: "https://example.sui-mainnet.quiknode.pro:443/test-token/", target: "dns:///example.sui-mainnet.quiknode.pro:9000", header: "x-token", value: "test-token"},
		{name: "quicknode_explicit_port", url: "https://example.sui-testnet.quiknode.pro:9001/test-token", target: "dns:///example.sui-testnet.quiknode.pro:9001", header: "x-token", value: "test-token"},
		{name: "quicknode_auth_override", url: "https://example.sui-devnet.quiknode.pro/old-token", target: "dns:///example.sui-devnet.quiknode.pro:9000", header: "x-token", value: "configured", auth: "configured"},
		{name: "quicknode_native_endpoint", url: "example.sui-mainnet.quiknode.pro:9000", target: "dns:///example.sui-mainnet.quiknode.pro:9000", header: "x-token", value: "configured", auth: "configured"},
		{name: "proxy_path", url: "https://example.org/rpc/token/", target: "dns:///example.org:443", path: "/rpc/token"},
		{name: "proxy_query", url: "https://example.org/rpc?key=test", target: "dns:///example.org:443", path: "/rpc", value: "?key=test"},
		{name: "escaped_path", url: "https://example.org/token%2Fpart", target: "dns:///example.org:443", path: "/token%2Fpart"},
		{name: "basic_auth", url: "https://user:password@example.org/rpc", target: "dns:///example.org:443", path: "/rpc", header: "authorization", value: "Basic dXNlcjpwYXNzd29yZA=="},
		{name: "not_quicknode", url: "https://example.sui-mainnet.quiknode.pro.example.org/token", target: "dns:///example.sui-mainnet.quiknode.pro.example.org:443", path: "/token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, err := parseEndpoint(tc.url)
			require.NoError(t, err)
			require.Equal(t, tc.target, e.target)
			if tc.url[:5] == "https" {
				require.Equal(t, tc.url, e.httpURL)
			}
			ctx, method := e.request(context.Background(), "/sui.rpc.v2.StateService/GetBalance", tc.auth)
			expected := tc.path + "/sui.rpc.v2.StateService/GetBalance"
			if tc.name == "proxy_query" {
				expected += tc.value
			}
			require.Equal(t, expected, method)
			md, _ := metadata.FromOutgoingContext(ctx)
			if tc.header != "" {
				require.Equal(t, []string{tc.value}, md.Get(tc.header))
			} else {
				require.Empty(t, md)
			}
		})
	}
}

// Verify URL routing on the actual HTTP/2 transport, not just endpoint parsing.
func TestGRPCProxyPathAndQuery(t *testing.T) {
	rpc := grpc.NewServer()
	pb.RegisterStateServiceServer(rpc, &rpcServer{balance: func(context.Context, *pb.GetBalanceRequest) (*pb.GetBalanceResponse, error) {
		return &pb.GetBalanceResponse{Balance: &pb.Balance{Balance: proto.Uint64(42)}}, nil
	}})
	defer rpc.Stop()
	requests := make(chan string, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.RequestURI()
		r.URL.Path = "/sui.rpc.v2.StateService/GetBalance"
		r.URL.RawPath = ""
		r.URL.RawQuery = ""
		rpc.ServeHTTP(w, r)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	c, err := NewClient(xc.NewChainConfig(xc.SUI).WithUrl(server.URL+"/rpc/token?api-key=test"), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{RootCAs: pool})))
	require.NoError(t, err)
	defer c.Close()
	balance, err := c.FetchBalanceFor(context.Background(), sender, "0x2::sui::SUI")
	require.NoError(t, err)
	require.Equal(t, uint64(42), balance.Uint64())
	require.Equal(t, "/rpc/token/sui.rpc.v2.StateService/GetBalance?api-key=test", <-requests)
}
