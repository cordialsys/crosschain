package testutil

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	xclient "github.com/cordialsys/crosschain/client"
	"github.com/stretchr/testify/require"
)

func wrapRPCResult(res string) string {
	return `{"jsonrpc":"2.0","result":` + res + `,"id":0}`
}

func wrapRPCError(err string) string {
	return `{"jsonrpc":"2.0","error":` + err + `,"id":0}`
}

// MockJSONRPCServer is a mocked RPC server
type MockJSONRPCServer struct {
	*httptest.Server
	body       []byte
	Counter    int
	ForceError int
	Response   interface{}
}

// MockJSONRPC creates a new MockJSONRPCServer given a response, or array of responses
func MockJSONRPC(t *testing.T, response interface{}) (mock *MockJSONRPCServer, close func()) {
	mock = &MockJSONRPCServer{
		Response: response,
		Server: httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			var err error
			mock.body, err = io.ReadAll(req.Body)
			log.Println("rpc>>", string(mock.body))

			require.NoError(t, err)
			curResponse := mock.Response
			if a, ok := mock.Response.([]string); ok {
				curResponse = a[mock.Counter]
			}
			mock.Counter++

			// error => send RPC error
			if e, ok := curResponse.(error); ok {
				rw.WriteHeader(400)
				rw.Write([]byte(wrapRPCError(e.Error())))
				return
			}

			// string => convert to JSON
			if s, ok := curResponse.(string); ok {
				if strings.Contains(s, "jsonrpc") {
					curResponse = json.RawMessage(s)
				} else {
					if mock.ForceError > 0 {
						rw.WriteHeader(mock.ForceError)
						curResponse = json.RawMessage(wrapRPCError(s))
					} else {
						curResponse = json.RawMessage(wrapRPCResult(s))
					}
				}
			}

			// JSON input, or serializable into JSON
			var responseBody []byte
			if v, ok := curResponse.(json.RawMessage); ok {
				responseBody = v
			} else {
				responseBody, err = json.Marshal(curResponse)
				require.NoError(t, err)
			}
			if len(responseBody) > 100*1024 {
				log.Println("<<rpc", fmt.Sprintf("[omitted %d byte response]", len(responseBody)))
			} else {
				log.Println("<<rpc", string(responseBody))
			}
			rw.Write(responseBody)
		})),
	}
	return mock, func() { mock.Close() }
}

// MockHTTPServer is a mocked HTTP server
type MockHTTPServer struct {
	*httptest.Server
	body        []byte
	Counter     int
	Response    interface{}
	StatusCodes []int
}

// MockHTTP creates a new MockHTTPServer given a response, or array of responses
func MockHTTP(t *testing.T, response interface{}, status int) (mock *MockHTTPServer, close func()) {
	mock = &MockHTTPServer{
		Response:    response,
		StatusCodes: []int{},
		Server: httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			curResponse := mock.Response
			if a, ok := mock.Response.([]string); ok {
				if mock.Counter >= len(a) {
					require.Fail(t, fmt.Sprintf("received another request but there's no response configured len=%d count=%d", len(a), mock.Counter))
				}
				curResponse = a[mock.Counter]
			}
			// default success
			if mock.Counter < len(mock.StatusCodes) {
				status = mock.StatusCodes[mock.Counter]
			}
			mock.Counter++

			// error => send RPC error
			if e, ok := curResponse.(error); ok {
				rw.WriteHeader(400)
				errBz, _ := json.Marshal(e)
				rw.Write([]byte(wrapRPCError(string(errBz))))
				return
			}
			if status == 0 {
				status = http.StatusOK
			}

			// string => convert to JSON
			if s, ok := curResponse.(string); ok {
				rw.WriteHeader(status)
				curResponse = json.RawMessage(s)
			}

			var err error
			mock.body, err = io.ReadAll(req.Body)
			log.Println("http>>", req)
			require.NoError(t, err)

			// JSON input, or serializable into JSON
			var responseBody []byte
			if v, ok := curResponse.(json.RawMessage); ok {
				responseBody = v
			} else {
				responseBody, err = json.Marshal(curResponse)
				require.NoError(t, err)
			}
			log.Printf("<<http(%d) %s\n", status, string(responseBody))
			rw.Write(responseBody)
		})),
	}
	return mock, func() { mock.Close() }
}

// reserialize will drop internal fields set by constructors
func Reserialize[T any](val *T) *T {
	var newVal T
	bz, _ := json.Marshal(val)
	json.Unmarshal(bz, &newVal)
	return &newVal
}

// Produces more readable diffs when comparing TxInfo in unit tests
func TxInfoEqual(t *testing.T, expected, actual xclient.TxInfo) {
	a := Reserialize(&expected)
	b := Reserialize(&actual)

	aBz, _ := json.MarshalIndent(a, "", "  ")
	bBz, _ := json.MarshalIndent(b, "", "  ")

	// t.Logf("expected TxInfo:\n%s", string(aBz))
	// t.Logf("actual TxInfo:\n%s", string(bBz))
	require.Equal(t, string(aBz), string(bBz))
}
