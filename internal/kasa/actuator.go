package kasa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 8 << 10 // 8 KiB

// Actuator implements engine.Actuator for Kasa by calling the kasa-agent
// sidecar over HTTP. It never talks the Kasa local protocol itself, and
// never resolves a device name to an IP — that's the agent's job.
type Actuator struct {
	baseURL string
	http    *http.Client
}

// requestTimeout comfortably exceeds the agent's worst-case lookup-miss
// path (a full subnet rescan, a retry delay, then another rescan — see
// kasa-agent's KASA_LOOKUP_MAX_RETRIES/KASA_LOOKUP_RETRY_DELAY_SECONDS) so
// that path's real 404 reaches homelink instead of being masked by a Go
// HTTP timeout. A cache hit, the common case, still returns almost
// instantly.
const requestTimeout = 30 * time.Second

// NewActuator constructs an Actuator from Settings.
func NewActuator(settings Settings) *Actuator {
	return &Actuator{
		baseURL: strings.TrimRight(settings.AgentBaseURL, "/"),
		http:    &http.Client{Timeout: requestTimeout},
	}
}

// Ping checks that the kasa-agent sidecar is reachable, for use in health checks.
func (a *Actuator) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("kasa agent unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("kasa agent healthz: %s", resp.Status)
	}
	return nil
}

type agentRequest struct {
	Action     string `json:"action"`
	Brightness *int   `json:"brightness,omitempty"`
}

// Execute performs the action described by action, which must be the
// *ActionConfig produced by DecodeAction, by calling the kasa-agent sidecar.
func (a *Actuator) Execute(ctx context.Context, action any) error {
	cfg, ok := action.(*ActionConfig)
	if !ok {
		return fmt.Errorf("kasa actuator: unexpected config type %T", action)
	}

	body, err := json.Marshal(agentRequest{Action: cfg.Action, Brightness: cfg.Brightness})
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/devices/%s/action", a.baseURL, url.PathEscape(cfg.Device))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("kasa agent request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		return fmt.Errorf("kasa agent %s: %s", resp.Status, snippet)
	}
	return nil
}
