package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeYoLink is a stand-in for the YoLink HTTP API used to exercise the HTTP transport, the
// Home, and the device controllers without touching the network. It serves the token endpoint
// and the v2 API endpoint, dispatching API requests by their BDDP "method" to registered
// handlers.
type fakeYoLink struct {
	server   *httptest.Server
	handlers map[string]func(bddp basicDownloadDataPacket) any // method -> response data
	requests []basicDownloadDataPacket                         // every API request received, in order
}

// newFakeYoLink starts a fake server. The caller registers method handlers via handle and
// closes it with t.Cleanup. The host is server.URL.
func newFakeYoLink(t *testing.T) *fakeYoLink {
	t.Helper()
	fake := &fakeYoLink{handlers: map[string]func(basicDownloadDataPacket) any{}}
	mux := http.NewServeMux()

	mux.HandleFunc("/open/yolink/token", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"access_token":  "test-access-token",
			"token_type":    "bearer",
			"expires_in":    7200,
			"refresh_token": "test-refresh-token",
		})
	})

	mux.HandleFunc("/open/yolink/v2/api", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var bddp basicDownloadDataPacket
		if err := json.Unmarshal(body, &bddp); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fake.requests = append(fake.requests, bddp)

		response := map[string]any{"method": bddp.Method, "code": "000000", "time": 0}
		if handler, ok := fake.handlers[bddp.Method]; ok {
			if data := handler(bddp); data != nil {
				response["data"] = data
			}
		} else {
			response["code"] = "010203" // method not supported
			response["desc"] = "no handler registered for " + bddp.Method
		}
		writeJSON(w, response)
	})

	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)
	return fake
}

// handle registers the response data returned for an API method.
func (f *fakeYoLink) handle(method string, data func(bddp basicDownloadDataPacket) any) {
	f.handlers[method] = data
}

// newHome builds a Home pointed at the fake server with canned credentials.
func (f *fakeYoLink) newHome(t *testing.T) *Home {
	t.Helper()
	home, err := NewHome(f.server.URL, func() string { return "id" }, func() string { return "secret" })
	if err != nil {
		t.Fatalf("NewHome: %v", err)
	}
	return home
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

// deviceJSON renders a device list entry in the YoLink wire format.
func deviceJSON(deviceId, name, deviceType string) map[string]any {
	return map[string]any{
		"deviceId":   deviceId,
		"deviceUDID": "udid-" + deviceId,
		"name":       name,
		"token":      "tok-" + deviceId,
		"type":       deviceType,
	}
}

// requestMethods returns the methods of all API requests the fake received.
func (f *fakeYoLink) requestMethods() string {
	methods := make([]string, len(f.requests))
	for i, request := range f.requests {
		methods[i] = request.Method
	}
	return strings.Join(methods, ",")
}
