package twinstake

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cordialsys/crosschain/client/services"
	"github.com/sirupsen/logrus"
)

type Client struct {
	Chain string
	Url   string

	// AWS needs a lot of pesky parameters
	Username string
	Region   string
	ClientId string

	password string
}
type Error struct {
	Message string `json:"message"`
}
type AwsError struct {
	Type    string `json:"__type"`
	Message string `json:"message"`
}

// func ensure0x(val string) string {
// 	if !strings.HasPrefix(val, "0x") {
// 		return "0x" + val
// 	}
// 	return val
// }

func NewClient(chain string, cfg *services.TwinstakeConfig) (*Client, error) {
	url := cfg.BaseUrl
	username := cfg.Username
	region := cfg.Region
	clientId := cfg.ClientId
	password, err := cfg.Password.Load()
	if err != nil {
		return nil, fmt.Errorf("could not load twinstake api password: %v", err)
	}

	return &Client{
		Chain:    chain,
		Url:      url,
		Username: username,
		Region:   region,
		ClientId: clientId,
		password: password,
	}, nil
}

// func (cli *Client) Get(path string, response any) error {
// 	return cli.Send("GET", path, nil, response)
// }

// func (cli *Client) GetAndPrint(path string) error {
// 	var res json.RawMessage
// 	err := cli.Send("GET", path, nil, &res)
// 	fmt.Println(string(res))
// 	return err
// }

//	func (cli *Client) Post(path string, requestBody any, response any) error {
//		return cli.Send("POST", path, requestBody, response)
//	}
func (cli *Client) Login() (string, error) {
	var requestData = &AwsAuthRequest{
		AuthParameters: AwsAuthParameters{
			Username: cli.Username,
			Password: cli.password,
		},
		AuthFlow: "USER_PASSWORD_AUTH",
		ClientID: cli.ClientId,
	}
	requestBody, _ := json.Marshal(requestData)
	var err error
	region := cli.Region
	if region == "" {
		region = "eu-west-3"
	}

	url := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/", region)
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}

	// AWS oddity
	request.Header.Add("Content-Type", "application/x-amz-json-1.1")
	request.Header.Add("X-Amz-Target", "AWSCognitoIdentityProviderService.InitiateAuth")

	logrus.WithField("url", url).Debug("POST")
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("failed to POST: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}
	logrus.WithFields(logrus.Fields{
		"body":   string(body),
		"status": resp.StatusCode,
	}).Debug("response")

	// AWS seems to not care about HTTP status codes, so we have to preemptively decode as an error to
	// see if it's an error.
	var errorMaybe AwsError
	if err := json.Unmarshal(body, &errorMaybe); err != nil {
		return "", fmt.Errorf("failed to unmarshal error response: %v", err)
	}
	if errorMaybe.Type != "" {
		if errorMaybe.Message != "" {
			return "", fmt.Errorf("%s: %s", errorMaybe.Type, errorMaybe.Message)
		}
		return "", fmt.Errorf("%s", errorMaybe.Type)
	}

	var incognitoResponse AwsIncognitoResponse
	if err := json.Unmarshal(body, &incognitoResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal aws authentication: %v", err)
	}
	return incognitoResponse.AuthenticationResult.AccessToken, nil

}
