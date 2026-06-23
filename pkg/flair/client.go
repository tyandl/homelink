package flair

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tyandl/homelink/pkg/flair/types"
)

// tokenRefreshBuffer is how long before expiry the access token is proactively refreshed.
const tokenRefreshBuffer = 60 * time.Second

// httpClientTimeout bounds each HTTP request so a stalled endpoint cannot block callers.
const httpClientTimeout = 30 * time.Second

// maxResponseBytes caps response body reads, bounding memory use against oversized responses.
const maxResponseBytes = 8 << 20 // 8 MiB

const (
	acceptJSONAPI   = "application/vnd.api+json"
	contentTypeJSON = "application/json"
)

// Client makes authenticated requests to the Flair REST API.
type Client struct {
	host         string
	httpClient   *http.Client
	clientId     func() string
	clientSecret func() string

	// mu guards auth and tokenExpiry, which are swapped out on refresh and may be read
	// concurrently by requests from multiple goroutines.
	mu          sync.Mutex
	auth        *types.AuthResponse
	tokenExpiry time.Time
}

// NewClient creates an authenticated Flair API client. host should be "https://api.flair.co"
// (no trailing slash). clientId and clientSecret supply OAuth2 credentials and may be called
// on each token fetch, so they can read from a vault or environment at call time.
func NewClient(host string, clientId func() string, clientSecret func() string) (*Client, error) {
	client := &Client{
		host:         host,
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

// accessToken returns a currently-valid access token, refreshing it first if at or near expiry.
func (client *Client) accessToken() (string, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if time.Now().After(client.tokenExpiry.Add(-tokenRefreshBuffer)) {
		if err := client.refreshLocked(); err != nil {
			return "", err
		}
	}
	return client.auth.AccessToken, nil
}

// refreshLocked obtains a new access token, preferring the refresh_token grant and falling
// back to client credentials. The caller must hold client.mu.
func (client *Client) refreshLocked() error {
	slog.Debug("refreshing flair access token")
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
		slog.Warn("flair token refresh failed, re-authenticating", "error", err)
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

// setAuth records a new auth response and computes when the access token expires.
// The caller must hold client.mu, except during construction.
func (client *Client) setAuth(auth *types.AuthResponse) {
	client.auth = auth
	client.tokenExpiry = time.Now().Add(time.Duration(auth.ExpiresIn) * time.Second)
	slog.Debug("flair access token set", "expiresIn", auth.ExpiresIn)
}

// fetchToken posts form values to /oauth2/token and returns a parsed AuthResponse.
func (client *Client) fetchToken(values url.Values) (*types.AuthResponse, error) {
	resp, err := client.httpClient.PostForm(client.host+"/oauth2/token", values)
	if err != nil {
		return nil, fmt.Errorf("flair auth: %w", err)
	}
	defer resp.Body.Close()
	body, err := readBody(resp)
	if err != nil {
		return nil, fmt.Errorf("flair auth: %w", err)
	}
	var auth types.AuthResponse
	if err := json.Unmarshal(body, &auth); err != nil {
		return nil, fmt.Errorf("flair auth: decode response: %w", err)
	}
	if auth.AccessToken == "" {
		return nil, fmt.Errorf("flair auth: no access token in response")
	}
	return &auth, nil
}

// get sends an authenticated GET and decodes a single-resource JSON:API envelope.
func (client *Client) get(urlOrPath string, params map[string]string) (rawEnvelope, error) {
	body, err := client.do(http.MethodGet, urlOrPath, params, nil)
	if err != nil {
		return rawEnvelope{}, err
	}
	var envelope rawEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return rawEnvelope{}, fmt.Errorf("flair GET %s: decode response: %w", urlOrPath, err)
	}
	if len(envelope.Errors) > 0 {
		return rawEnvelope{}, &APIError{Errors: envelope.Errors}
	}
	return envelope, nil
}

// list sends an authenticated GET and decodes a collection JSON:API envelope.
func (client *Client) list(urlOrPath string, params map[string]string) (rawCollection, error) {
	body, err := client.do(http.MethodGet, urlOrPath, params, nil)
	if err != nil {
		return rawCollection{}, err
	}
	var collection rawCollection
	if err := json.Unmarshal(body, &collection); err != nil {
		return rawCollection{}, fmt.Errorf("flair GET %s: decode response: %w", urlOrPath, err)
	}
	if len(collection.Errors) > 0 {
		return rawCollection{}, &APIError{Errors: collection.Errors}
	}
	return collection, nil
}

// patch sends an authenticated PATCH and decodes the resulting JSON:API envelope.
func (client *Client) patch(path, id, resourceType string, attributes any) (rawEnvelope, error) {
	payload, err := json.Marshal(patchBody{
		Data: patchData{ID: id, Type: resourceType, Attributes: attributes},
	})
	if err != nil {
		return rawEnvelope{}, fmt.Errorf("flair PATCH %s: marshal request: %w", path, err)
	}
	body, err := client.do(http.MethodPatch, path, nil, payload)
	if err != nil {
		return rawEnvelope{}, err
	}
	var envelope rawEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return rawEnvelope{}, fmt.Errorf("flair PATCH %s: decode response: %w", path, err)
	}
	if len(envelope.Errors) > 0 {
		return rawEnvelope{}, &APIError{Errors: envelope.Errors}
	}
	return envelope, nil
}

// post sends an authenticated POST and decodes the resulting JSON:API envelope.
func (client *Client) post(path, resourceType string, attributes any, relationships map[string]relWrapper) (rawEnvelope, error) {
	payload, err := json.Marshal(createBody{
		Data: createData{Type: resourceType, Attributes: attributes, Relationships: relationships},
	})
	if err != nil {
		return rawEnvelope{}, fmt.Errorf("flair POST %s: marshal request: %w", path, err)
	}
	body, err := client.do(http.MethodPost, path, nil, payload)
	if err != nil {
		return rawEnvelope{}, err
	}
	var envelope rawEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return rawEnvelope{}, fmt.Errorf("flair POST %s: decode response: %w", path, err)
	}
	if len(envelope.Errors) > 0 {
		return rawEnvelope{}, &APIError{Errors: envelope.Errors}
	}
	return envelope, nil
}

// deleteResource sends an authenticated DELETE. The Flair API returns 204 No Content on success.
func (client *Client) deleteResource(path string) error {
	_, err := client.do(http.MethodDelete, path, nil, nil)
	return err
}

// do executes an authenticated HTTP request. urlOrPath may be a full URL (from a relationship
// link) or a bare path such as "/api/structures".
func (client *Client) do(method, urlOrPath string, params map[string]string, requestBody []byte) ([]byte, error) {
	token, err := client.accessToken()
	if err != nil {
		return nil, err
	}

	var requestURL string
	if strings.HasPrefix(urlOrPath, "http://") || strings.HasPrefix(urlOrPath, "https://") {
		requestURL = urlOrPath
	} else {
		requestURL = client.host + urlOrPath
	}
	if len(params) > 0 {
		query := url.Values{}
		for key, value := range params {
			query.Set(key, value)
		}
		requestURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if requestBody != nil {
		bodyReader = bytes.NewReader(requestBody)
	}

	request, err := http.NewRequest(method, requestURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("flair %s %s: %w", method, urlOrPath, err)
	}
	request.Header.Set("Accept", acceptJSONAPI)
	request.Header.Set("Authorization", "Bearer "+token)
	if requestBody != nil {
		request.Header.Set("Content-Type", contentTypeJSON)
	}

	slog.Debug("flair request", "method", method, "url", urlOrPath)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("flair %s %s: %w", method, urlOrPath, err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	responseBody, err := readBody(response)
	if err != nil {
		return nil, fmt.Errorf("flair %s %s: %w", method, urlOrPath, err)
	}
	slog.Debug("flair response", "method", method, "url", urlOrPath, "status", response.StatusCode)
	return responseBody, nil
}

// readBody verifies a 2xx status and returns the response body bounded to maxResponseBytes.
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
		return nil, fmt.Errorf("HTTP %s: %s", response.Status, snippet)
	}
	return body, nil
}
