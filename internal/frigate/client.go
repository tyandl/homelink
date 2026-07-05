package frigate

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"github.com/tyandl/homelink/internal/config"
)

const maxResponseBytes = 8 << 20 // 8 MiB

// Client sends authenticated requests to the Frigate HTTP API.
// Cloudflare Access service-token headers are injected on every request via
// cfTransport. When Frigate's native auth is also enabled, Login must be called
// once to obtain a session cookie, which the embedded jar carries forward.
type Client struct {
	baseURL string
	http    *http.Client
	config  *config.Config
}

// New constructs a Client from config. Call Login separately if Frigate's own
// authentication is enabled (FRIGATE_USER is set).
func New(cfg *config.Config) *Client {
	transport := http.DefaultTransport
	if cfg.FrigateInsecureSkipVerify {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}
	jar, _ := cookiejar.New(nil)
	return &Client{
		baseURL: cfg.FrigateBaseURL,
		config:  cfg,
		http: &http.Client{
			Timeout:   15 * time.Second,
			Transport: transport,
			Jar:       jar,
		},
	}
}

// Login authenticates to Frigate via POST /api/login, storing the session
// cookie in the client jar. It is a no-op when FRIGATE_USER is not set.
func (client *Client) Login() error {
	if client.config.FrigateUser == "" {
		return nil
	}
	body, err := json.Marshal(map[string]string{
		"user":     client.config.FrigateUser,
		"password": client.config.FrigatePassword,
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, client.baseURL+"/api/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("frigate login %s: %s", response.Status, snippet)
	}
	return nil
}

// CreateEvent fires a manual event on the given camera with the given label.
// On a 401 it re-authenticates once and retries.
func (client *Client) CreateEvent(camera, label string, params CreateEventParams) error {
	response, err := client.postCreateEvent(camera, label, params)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusUnauthorized {
		response.Body.Close()
		if err := client.Login(); err != nil {
			return fmt.Errorf("frigate re-auth: %w", err)
		}
		if response, err = client.postCreateEvent(camera, label, params); err != nil {
			return err
		}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("frigate create event %s: %s", response.Status, snippet)
	}
	return nil
}

// CreateEventParams are the optional fields for a manual Frigate event.
type CreateEventParams struct {
	SourceType string     `json:"source_type,omitempty"`
	SubLabel   string     `json:"sub_label,omitempty"`
	Score      float64    `json:"score,omitempty"`
	Draw       *EventDraw `json:"draw,omitempty"`
	// Duration is how many seconds the event stays open. 0 requires an explicit /end call.
	Duration *int `json:"duration,omitempty"`
	// IncludeRecording attaches a recording clip to the event. Defaults to true in Frigate.
	IncludeRecording *bool `json:"include_recording,omitempty"`
	// PreCapture is seconds of footage before the trigger to include. Frigate's default applies when nil.
	PreCapture *int `json:"pre_capture,omitempty"`
}

// EventDraw carries bounding box information for a Frigate manual event.
// Coordinates use [x, y, dx, dy] format (top-left corner + size, 0.0–1.0).
type EventDraw struct {
	Boxes []EventBox `json:"boxes"`
}

// EventBox is a rendering instruction for the snapshot image, not event metadata.
// Frigate passes these to draw_snapshot_overlay_boxes which paints them onto the
// JPEG — they are not stored in the database or searchable.
//
// Box coordinates are [x, y, dx, dy]: top-left corner + width/height as fractions
// of the frame dimensions (0.0–1.0).
//
// Label is display text painted next to the rectangle on the snapshot. It is
// distinct from the event's sub_label, which is a searchable event attribute stored
// in the database. Use Label only when you want the overlay text to differ from the
// event label in the URL path; otherwise it is redundant.
//
// Color is an optional [R, G, B] tuple (0–255) for the drawn rectangle.
type EventBox struct {
	Box   [4]float64 `json:"box"`
	Score float64    `json:"score,omitempty"`
	Label string     `json:"label,omitempty"`
	Color []int      `json:"color,omitempty"`
}

func (client *Client) postCreateEvent(camera, label string, params CreateEventParams) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/events/%s/%s/create", client.baseURL, camera, label)
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	return client.http.Do(request)
}

// envOr is a helper used in tests.
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
