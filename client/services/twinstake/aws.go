package twinstake

// AuthenticationResult represents the structure for the authentication result.
type AwsAuthenticationResult struct {
	AccessToken  string `json:"AccessToken"`
	ExpiresIn    int    `json:"ExpiresIn"`
	IdToken      string `json:"IdToken"`
	RefreshToken string `json:"RefreshToken"`
	TokenType    string `json:"TokenType"`
}

// Response represents the main structure of the response.
type AwsIncognitoResponse struct {
	AuthenticationResult AwsAuthenticationResult `json:"AuthenticationResult"`
	ChallengeParameters  map[string]string       `json:"ChallengeParameters"`
}

type AwsAuthParameters struct {
	Username string `json:"USERNAME"`
	Password string `json:"PASSWORD"`
}

// AuthRequest represents the structure for the authentication request.
type AwsAuthRequest struct {
	AuthParameters AwsAuthParameters `json:"AuthParameters"`
	AuthFlow       string            `json:"AuthFlow"`
	ClientID       string            `json:"ClientId"`
}
