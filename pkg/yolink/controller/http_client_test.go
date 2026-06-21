package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tyandl/homelink/pkg/yolink/types"
)

func TestConnectHttpFetchesToken(t *testing.T) {
	fake := newFakeYoLink(t)
	client, err := connectHttp(fake.server.URL, func() string { return "id" }, func() string { return "secret" })
	if err != nil {
		t.Fatal(err)
	}
	token, err := client.AccessToken()
	if err != nil {
		t.Fatal(err)
	}
	if token != "test-access-token" {
		t.Fatalf("token = %q", token)
	}
}

func TestConnectHttpAuthFailure(t *testing.T) {
	// A token endpoint that returns no access token is an auth failure.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"token_type": "bearer"})
	}))
	t.Cleanup(server.Close)
	if _, err := connectHttp(server.URL, func() string { return "id" }, func() string { return "s" }); err == nil {
		t.Fatal("expected auth failure when no access token is issued")
	}
}

func TestPostRoundTrip(t *testing.T) {
	fake := newFakeYoLink(t)
	fake.handle("Home.getGeneralInfo", func(basicDownloadDataPacket) any {
		return map[string]any{"id": "home1", "name": "Test Home"}
	})
	client, err := connectHttp(fake.server.URL, func() string { return "id" }, func() string { return "s" })
	if err != nil {
		t.Fatal(err)
	}

	bddp := newBDDP("Home.getGeneralInfo", nil, nil, nil)
	var budp basicUploadDataPacket
	if err := client.Post(bddp, &budp); err != nil {
		t.Fatal(err)
	}
	if !budp.Code.IsSuccess() {
		t.Fatalf("code = %s", budp.Code)
	}
	if string(budp.Data) != `{"id":"home1","name":"Test Home"}` {
		t.Fatalf("data = %s", budp.Data)
	}
}

func TestPostNon2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open/yolink/token" {
			writeJSON(w, map[string]any{"access_token": "t", "expires_in": 7200})
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	client, err := connectHttp(server.URL, func() string { return "id" }, func() string { return "s" })
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Post(newBDDP("X.y", nil, nil, nil), &basicUploadDataPacket{}); err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestAccessTokenRefreshesWhenExpired(t *testing.T) {
	fake := newFakeYoLink(t)
	client, err := connectHttp(fake.server.URL, func() string { return "id" }, func() string { return "s" })
	if err != nil {
		t.Fatal(err)
	}
	// Force the cached token to look expired so the next access triggers a refresh.
	client.mu.Lock()
	client.auth = &types.AuthResponse{AccessToken: "stale", RefreshToken: "r", ExpiresIn: 1}
	client.tokenExpiry = client.tokenExpiry.Add(-2 * tokenRefreshBuffer)
	client.tokenExpiry = client.tokenExpiry.AddDate(0, 0, -1)
	client.mu.Unlock()

	token, err := client.AccessToken()
	if err != nil {
		t.Fatal(err)
	}
	if token != "test-access-token" {
		t.Fatalf("expected refreshed token, got %q", token)
	}
}
