package agent

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	icpaddress "github.com/cordialsys/crosschain/chain/internet_computer/address"
	"github.com/cordialsys/crosschain/chain/internet_computer/candid"
	"github.com/cordialsys/crosschain/chain/internet_computer/certification"
	"github.com/cordialsys/crosschain/chain/internet_computer/certification/hashtree"
	types "github.com/cordialsys/crosschain/chain/internet_computer/client/types"
	"github.com/fxamacker/cbor/v2"
	log "github.com/sirupsen/logrus"
)

const (
	V2CanisterApi = "/api/v2/canister/"
)

type AgentConfig struct {
	// Agent identity. Anynymous if not set.
	Identity icpaddress.Ed25519Identity
	// Defaults to 2 minutes
	IngressExpiry time.Duration
	// Defaults to "https://icp-api.io"
	Url    *url.URL
	Logger *log.Entry
}

type Agent struct {
	Identity icpaddress.Ed25519Identity
	Config   AgentConfig
	Url      *url.URL
	Logger   *log.Entry
}

func (a *Agent) Info(msg string) {
	if a == nil || a.Logger == nil {
		return
	}
	a.Logger.Info(msg)
}

func NewAgent(config AgentConfig) (*Agent, error) {
	identity := icpaddress.Ed25519Identity{
		PublicKey: nil,
	}

	if config.Identity.PublicKey != nil {
		identity = config.Identity
	}

	url := config.Url
	if url == nil || url.Host == "" {
		defaultUrl, err := url.Parse("https://icp-api.io")
		if err != nil {
			return nil, fmt.Errorf("failed to parse default URL: %w", err)
		}
		url = defaultUrl
	}

	logger := config.Logger
	if logger == nil {
		raw := log.New()
		logger = log.NewEntry(raw)
	}

	return &Agent{
		Identity: identity,
		Config:   config,
		Url:      url,
		Logger:   logger,
	}, nil
}

func (config *AgentConfig) ExpiryDate() uint64 {
	ingressExpiry := config.IngressExpiry
	if ingressExpiry == 0 {
		// Defaults to 2 minutes
		ingressExpiry = 2 * time.Minute
	}
	return uint64(time.Now().Add(ingressExpiry).UnixNano())
}

func newNonce() ([]byte, error) {
	/* Read 10 bytes of random data, which is smaller than the max allowed by the IC (32 bytes)
	 * and should still be enough from a practical point of view. */
	nonce := make([]byte, 10)
	_, err := rand.Read(nonce)
	return nonce, err
}

func (a AgentConfig) CreateUnsignedRequest(canisterID icpaddress.Principal, typ types.RequestType, methodName string, args ...any) (types.Request, error) {
	rawArgs, err := candid.Marshal(args)
	if err != nil {
		return types.Request{}, fmt.Errorf("failed to marshal args: %w", err)
	}

	nonce, err := newNonce()
	if err != nil {
		return types.Request{}, fmt.Errorf("failed to generate nonce: %w", err)
	}

	return types.Request{
		Type:          typ,
		Sender:        a.Identity,
		Nonce:         nonce,
		IngressExpiry: a.ExpiryDate(),
		CanisterID:    canisterID,
		MethodName:    methodName,
		Arguments:     rawArgs,
	}, nil
}

func (a Agent) Call(canisterID icpaddress.Principal, requestID types.RequestID, signedPayload []byte, out []any) error {
	url := fmt.Sprintf("%s/api/v3/canister/%s/call", a.Url, canisterID.Encode())
	resp, err := http.Post(url, "application/cbor", bytes.NewBuffer(signedPayload))
	if err != nil {
		return fmt.Errorf("failed to post the request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted:
		return nil
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		var status struct {
			Status string `cbor:"status"`
		}
		if err := cbor.Unmarshal(body, &status); err != nil {
			return fmt.Errorf("failed to unmarshal response status: %w", err)
		}
		switch status.Status {
		case "replied":
			var certificate struct {
				Certificate []byte `cbor:"certificate"`
			}
			err = cbor.Unmarshal(body, &certificate)

			rawCertificate := certificate.Certificate
			if len(rawCertificate) != 0 {
				var certificate certification.Certificate
				if err := cbor.Unmarshal(rawCertificate, &certificate); err != nil {
					return err
				}

				path := []hashtree.Label{hashtree.Label("request_status"), requestID[:]}
				if raw, err := certificate.Tree.Lookup(append(path, hashtree.Label("reply"))...); err == nil {
					return candid.Unmarshal(raw, out)
				}

				rejectCode, err := certificate.Tree.Lookup(append(path, hashtree.Label("reject_code"))...)
				if err != nil {
					return err
				}
				message, err := certificate.Tree.Lookup(append(path, hashtree.Label("reject_message"))...)
				if err != nil {
					return err
				}
				errorCode, err := certificate.Tree.Lookup(append(path, hashtree.Label("error_code"))...)
				if err != nil {
					return err
				}
				return types.PreprocessingError{
					RejectCode: uint64FromBytes(rejectCode),
					Message:    string(message),
					ErrorCode:  string(errorCode),
				}
			}
			return err
		case "non_replicated_rejection":
			var pErr types.PreprocessingError
			if err := cbor.Unmarshal(body, &pErr); err != nil {
				return err
			}
			return pErr
		default:
			return fmt.Errorf("unknown status: %s", status)
		}
	default:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("(%d) %s: %s", resp.StatusCode, resp.Status, body)
	}
}

func (a Agent) CallAnonymous(canisterID icpaddress.Principal, methodName string, in []any, out []any) error {
	unsignedPayload, err := a.Config.CreateUnsignedRequest(canisterID, types.RequestTypeCall, methodName, in...)
	if err != nil {
		return fmt.Errorf("failed to create payload: %w", err)
	}
	requestID := unsignedPayload.RequestID()

	payload, err := unsignedPayload.Sign(nil)
	if err != nil {
		return fmt.Errorf("failed to sign the payload: %w", err)
	}

	return a.Call(canisterID, requestID, payload, out)
}

func (a Agent) Query(canisterID icpaddress.Principal, methodName string, in []any, out []any) error {
	unsignedPayload, err := a.Config.CreateUnsignedRequest(canisterID, types.RequestTypeQuery, methodName, in...)
	if err != nil {
		return fmt.Errorf("failed to create payload: %w", err)
	}

	payload, err := unsignedPayload.Sign(nil)
	if err != nil {
		return fmt.Errorf("failed to sign the payload: %w", err)
	}

	url := fmt.Sprintf("%s/api/v2/canister/%s/query", a.Url, canisterID.Encode())

	logger := a.Logger.WithFields(log.Fields{
		"raw_args":         in,
		"unsigned_payload": unsignedPayload,
		"payload":          hex.EncodeToString(payload),
		"method":           methodName,
		"url":              url,
	})
	logger.Debug("posting request")

	resp, err := http.Post(url, "application/cbor", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to post the request: %w", err)
	}
	defer resp.Body.Close()

	logger = logger.WithField("raw_response", resp)
	logger.Debug("got response")

	switch resp.StatusCode {
	case http.StatusOK:

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		logger.Debugf("response body: %s", hex.EncodeToString(body))

		var resp types.Response
		if err := cbor.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to unmarshal response body: %w", err)
		}

		switch resp.Status {
		case "replied":
			var reply struct {
				Arg []byte `ic:"arg"`
			}
			if err := cbor.Unmarshal(resp.Reply, &reply); err != nil {
				return err
			}
			return candid.Unmarshal(reply.Arg, out)
		case "rejected":
			return types.PreprocessingError{
				RejectCode: resp.RejectCode,
				Message:    resp.RejectMsg,
				ErrorCode:  resp.ErrorCode,
			}
		default:
			panic("unreachable")
		}
	default:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		return fmt.Errorf("(%d) %s: %s", resp.StatusCode, resp.Status, body)
	}
}

func uint64FromBytes(raw []byte) uint64 {
	switch len(raw) {
	case 1:
		return uint64(raw[0])
	case 2:
		return uint64(binary.BigEndian.Uint16(raw))
	case 4:
		return uint64(binary.BigEndian.Uint32(raw))
	case 8:
		return binary.BigEndian.Uint64(raw)
	default:
		panic(raw)
	}
}
