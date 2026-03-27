package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultBaseURL = "https://api.openweathermap.org/data/2.5/weather"

// WeatherData holds the parsed weather information returned by the API.
type WeatherData struct {
	Temperature float64
	FeelsLike   float64
	Humidity    int
	Pressure    int
}

// apiResponse mirrors the relevant parts of the OpenWeather API JSON response.
type apiResponse struct {
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Humidity  int     `json:"humidity"`
		Pressure  int     `json:"pressure"`
	} `json:"main"`
	Message string `json:"message"`
	Cod     any    `json:"cod"`
}

// Client is an OpenWeather API client.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL (useful for testing).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = u }
}

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// NewClient creates a new OpenWeather API client.
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// FetchWeather retrieves weather data for the given city and country code.
func (c *Client) FetchWeather(ctx context.Context, city, country string) (*WeatherData, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	q := u.Query()
	q.Set("q", city+","+country)
	q.Set("appid", c.apiKey)
	q.Set("units", "metric")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr apiResponse
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, apiErr.Message)
		}
		return nil, fmt.Errorf("API error: HTTP %d", resp.StatusCode)
	}

	var data apiResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &WeatherData{
		Temperature: data.Main.Temp,
		FeelsLike:   data.Main.FeelsLike,
		Humidity:    data.Main.Humidity,
		Pressure:    data.Main.Pressure,
	}, nil
}
