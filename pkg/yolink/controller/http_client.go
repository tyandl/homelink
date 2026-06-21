package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/tyandl/homelink/pkg/yolink/types"
)

// tokenRefreshBuffer is how long before expiry the access token is proactively refreshed.
const tokenRefreshBuffer = 60 * time.Second

// httpClientTimeout bounds each HTTP request so a stalled endpoint can't block callers
// indefinitely (and, since refresh holds the mutex, every other caller waiting on it).
const httpClientTimeout = 30 * time.Second

// maxResponseBytes caps how much of a response body is read, bounding memory use against
// an oversized or malicious response.
const maxResponseBytes = 8 << 20 // 8 MiB

// HttpClient makes authenticated calls to the YoLink HTTP API, managing the access token
// (initial auth plus refresh) on behalf of the connection.
type HttpClient struct {
	httpHost     string
	httpClient   *http.Client
	clientId     func() string
	clientSecret func() string

	// mu guards auth and tokenExpiry, which are swapped out on refresh and may be read
	// concurrently by Post calls from multiple goroutines.
	mu          sync.Mutex
	auth        *types.AuthResponse
	tokenExpiry time.Time
}

func connectHttp(httpHost string, clientId func() string, clientSecret func() string) (*HttpClient, error) {
	client := &HttpClient{
		httpHost:     httpHost,
		httpClient:   &http.Client{Timeout: httpClientTimeout},
		clientId:     clientId,
		clientSecret: clientSecret,
	}
	auth, err := client.fetchToken(url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientId()},
		"client_secret": {clientSecret()},
	})
	if err != nil {
		return nil, err
	}
	client.setAuth(auth)
	return client, nil
}

// AccessToken returns a currently-valid access token, refreshing if necessary.
func (client *HttpClient) AccessToken() (string, error) { return client.accessToken() }

func (client *HttpClient) Post(bddp *basicDownloadDataPacket, budp *basicUploadDataPacket) error {
	accessToken, err := client.accessToken()
	if err != nil {
		return err
	}
	slog.Debug("yolink http request", "method", bddp.Method, "device", bddp.TargetDevice, "msgid", bddp.MsgId)
	payload, err := json.Marshal(bddp)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, client.httpHost+"/open/yolink/v2/api", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := readBody(response)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, &budp); err != nil {
		return err
	}
	slog.Debug("yolink http response", "method", budp.Method, "code", budp.Code)
	return nil
}

// readBody verifies a 2xx status and returns the response body, bounded to maxResponseBytes.
func readBody(response *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		snippet := body
		if len(snippet) > 512 {
			snippet = snippet[:512]
		}
		return nil, fmt.Errorf("unexpected HTTP status %s: %s", response.Status, snippet)
	}
	return body, nil
}

// fetchToken posts the given form values to the token endpoint and returns the parsed
// auth response, validating that an access token was issued.
func (client *HttpClient) fetchToken(values url.Values) (*types.AuthResponse, error) {
	resp, err := client.httpClient.PostForm(client.httpHost+"/open/yolink/token", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var auth types.AuthResponse
	if err := json.Unmarshal(body, &auth); err != nil {
		return nil, err
	}
	if auth.AccessToken == "" {
		return nil, fmt.Errorf("auth failed: no access token in response")
	}
	return &auth, nil
}

// setAuth records a new auth response and computes when its access token expires. The
// caller must hold client.mu, except during construction.
func (client *HttpClient) setAuth(auth *types.AuthResponse) {
	client.auth = auth
	client.tokenExpiry = time.Now().Add(time.Duration(auth.ExpiresIn) * time.Second)
	slog.Debug("yolink access token set", "expiresIn", auth.ExpiresIn)
}

// accessToken returns a currently-valid access token, refreshing it first if it is at or
// near expiry.
func (client *HttpClient) accessToken() (string, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if time.Now().After(client.tokenExpiry.Add(-tokenRefreshBuffer)) {
		if err := client.refreshLocked(); err != nil {
			return "", err
		}
	}
	return client.auth.AccessToken, nil
}

// refreshLocked obtains a new access token, preferring the refresh_token grant (as YoLink
// recommends) and falling back to re-authenticating with the client credentials. The
// caller must hold client.mu.
func (client *HttpClient) refreshLocked() error {
	slog.Debug("refreshing yolink access token")
	if client.auth != nil && client.auth.RefreshToken != "" {
		auth, err := client.fetchToken(url.Values{
			"grant_type":    {"refresh_token"},
			"client_id":     {client.clientId()},
			"refresh_token": {client.auth.RefreshToken},
		})
		if err == nil {
			client.setAuth(auth)
			return nil
		}
		slog.Warn("token refresh failed, re-authenticating", "error", err)
	}
	auth, err := client.fetchToken(url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {client.clientId()},
		"client_secret": {client.clientSecret()},
	})
	if err != nil {
		return err
	}
	client.setAuth(auth)
	return nil
}
