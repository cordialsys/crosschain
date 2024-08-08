package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	xc "github.com/cordialsys/crosschain"
	xcclient "github.com/cordialsys/crosschain/client"

	"github.com/sirupsen/logrus"
)

type GetValidatorResponse struct {
	ExecutionOptimistic bool      `json:"execution_optimistic"`
	Finalized           bool      `json:"finalized"`
	Data                Validator `json:"data"`
}

type Validator struct {
	Index     string           `json:"index"`
	Balance   string           `json:"balance"`
	Status    ValidatorStatus  `json:"status"`
	Validator ValidatorDetails `json:"validator"`
}

type ValidatorStatus string

func (s ValidatorStatus) ToState() (xcclient.State, bool) {
	var state xcclient.State = ""
	// ethereum validator states
	switch s {
	case "pending_initialized", "pending_queued":
		state = xcclient.Activating
	case "active_ongoing":
		state = xcclient.Active
	case "withdrawal_possible", "withdrawal_done", "exited_unslashed", "exited_slashed":
		state = xcclient.Inactive
	case "active_exiting":
		state = xcclient.Deactivating
	default:
	}
	return state, state != ""
}

type ValidatorDetails struct {
	Pubkey                     string `json:"pubkey"`
	WithdrawalCredentials      string `json:"withdrawal_credentials"`
	EffectiveBalance           string `json:"effective_balance"`
	Slashed                    bool   `json:"slashed"`
	ActivationEligibilityEpoch string `json:"activation_eligibility_epoch"`
	ActivationEpoch            string `json:"activation_epoch"`
	ExitEpoch                  string `json:"exit_epoch"`
	WithdrawableEpoch          string `json:"withdrawable_epoch"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Fetch validator using beacon API
func (client *Client) FetchValidator(ctx context.Context, validator string) (*GetValidatorResponse, error) {
	if !strings.HasPrefix(validator, "0x") {
		validator = "0x" + validator
	}
	var info GetValidatorResponse
	err := client.Get("eth/v1/beacon/states/head/validators/"+validator, &info)
	return &info, err
}

func (client *Client) FetchValidatorBalance(ctx context.Context, validator string) (*xcclient.StakedBalance, error) {
	val, err := client.FetchValidator(ctx, validator)
	if err != nil {
		return nil, err
	}
	gwei, _ := xc.NewAmountHumanReadableFromStr(val.Data.Validator.EffectiveBalance)
	amount := gwei.ToBlockchain(9)
	var ok bool
	status, ok := val.Data.Status.ToState()
	if !ok {
		// assume it's still activating
		status = xcclient.Activating
		logrus.Warn("unknown beacon validator state", status)
	}
	return xcclient.NewStakedBalance(amount, status, validator, ""), nil
}

func (cli *Client) Get(path string, response any) error {
	return cli.Send("GET", path, nil, response)
}

func (cli *Client) GetAndPrint(path string) error {
	var res json.RawMessage
	err := cli.Send("GET", path, nil, &res)
	fmt.Println(string(res))
	return err
}

func (cli *Client) Post(path string, requestBody any, response any) error {
	return cli.Send("POST", path, requestBody, response)
}
func (cli *Client) Send(method string, path string, requestBody any, response any) error {
	path = strings.TrimPrefix(path, "/")
	baseUrl := cli.Asset.GetChain().URL
	baseUrl = strings.TrimSuffix(baseUrl, "/")

	url := fmt.Sprintf("%s/%s", baseUrl, path)
	var request *http.Request
	var err error
	if requestBody == nil {
		request, err = http.NewRequest(method, url, nil)
	} else {
		bz, _ := json.Marshal(requestBody)
		request, err = http.NewRequest(method, url, bytes.NewBuffer(bz))
		if err == nil {
			request.Header.Add("content-type", "application/json")
		}
	}
	if err != nil {
		return err
	}

	logrus.WithField("url", url).Debug(method)
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to GET: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}
	logrus.WithFields(logrus.Fields{
		"body":   string(body),
		"status": resp.StatusCode,
	}).Debug("response")

	if resp.StatusCode == http.StatusOK || resp.StatusCode == 201 {
		if response != nil {
			if err := json.Unmarshal(body, response); err != nil {
				return fmt.Errorf("failed to unmarshal response: %v", err)
			}
		}
		return nil
	} else {
		// Deserialize to ErrorResponse struct for other status codes
		var errorResponse Error
		logrus.WithField("body", string(body)).Debug("error")
		if err := json.Unmarshal(body, &errorResponse); err != nil {
			return fmt.Errorf("failed to unmarshal error response: %v", err)
		}
		if errorResponse.Message != "" {
			return fmt.Errorf("%s", errorResponse.Message)
		}
		logrus.WithField("body", string(body)).WithField("chain", cli.Asset.GetChain().Chain).Warn("unknown beacon api error")
		return fmt.Errorf("unknown beacon api error (%d)", resp.StatusCode)
	}
}
