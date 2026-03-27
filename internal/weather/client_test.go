package weather

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchWeather_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("q"); got != "Amsterdam,NL" {
			t.Errorf("q param = %q, want %q", got, "Amsterdam,NL")
		}
		if got := q.Get("units"); got != "metric" {
			t.Errorf("units param = %q, want %q", got, "metric")
		}
		if got := q.Get("appid"); got != "test-key" {
			t.Errorf("appid param = %q, want %q", got, "test-key")
		}

		resp := map[string]any{
			"main": map[string]any{
				"temp":       18.5,
				"feels_like": 17.2,
				"humidity":   65,
				"pressure":   1013,
			},
			"cod": 200,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	data, err := c.FetchWeather(context.Background(), "Amsterdam", "NL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Temperature != 18.5 {
		t.Errorf("Temperature = %v, want 18.5", data.Temperature)
	}
	if data.FeelsLike != 17.2 {
		t.Errorf("FeelsLike = %v, want 17.2", data.FeelsLike)
	}
	if data.Humidity != 65 {
		t.Errorf("Humidity = %v, want 65", data.Humidity)
	}
	if data.Pressure != 1013 {
		t.Errorf("Pressure = %v, want 1013", data.Pressure)
	}
}

func TestFetchWeather_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cod":     401,
			"message": "Invalid API key",
		})
	}))
	defer srv.Close()

	c := NewClient("bad-key", WithBaseURL(srv.URL))
	_, err := c.FetchWeather(context.Background(), "Amsterdam", "NL")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	want := "API error (HTTP 401): Invalid API key"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestFetchWeather_CityNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cod":     "404",
			"message": "city not found",
		})
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.FetchWeather(context.Background(), "Nonexistent", "XX")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	want := "API error (HTTP 404): city not found"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestFetchWeather_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.FetchWeather(context.Background(), "Amsterdam", "NL")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchWeather_ServerDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srvURL := srv.URL
	srv.Close()

	c := NewClient("test-key", WithBaseURL(srvURL))
	_, err := c.FetchWeather(context.Background(), "Amsterdam", "NL")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchWeather_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.FetchWeather(ctx, "Amsterdam", "NL")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchWeather_HTTPErrorWithoutMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	c := NewClient("test-key", WithBaseURL(srv.URL))
	_, err := c.FetchWeather(context.Background(), "Amsterdam", "NL")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	want := "API error: HTTP 500"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient("my-key")
	if c.apiKey != "my-key" {
		t.Errorf("apiKey = %q, want %q", c.apiKey, "my-key")
	}
	if c.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, defaultBaseURL)
	}
	if c.httpClient == nil {
		t.Fatal("httpClient is nil")
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	custom := &http.Client{}
	c := NewClient("key", WithBaseURL("http://example.com"), WithHTTPClient(custom))
	if c.baseURL != "http://example.com" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "http://example.com")
	}
	if c.httpClient != custom {
		t.Error("httpClient not set to custom client")
	}
}
