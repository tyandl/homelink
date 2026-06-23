package types

// AuthResponse is the OAuth2 token response from POST /oauth2/token.
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    uint   `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"` // space-delimited scope string
}
