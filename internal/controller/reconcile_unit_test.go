package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	weatherv1alpha1 "github.com/rtoma/openweather-controller/api/v1alpha1"
	"github.com/rtoma/openweather-controller/internal/weather"
)

// mockWeatherFetcher implements WeatherFetcher for unit testing.
type mockWeatherFetcher struct {
	data    *weather.WeatherData
	err     error
	called  bool
	city    string
	country string
}

func (m *mockWeatherFetcher) FetchWeather(_ context.Context, city, country string) (*weather.WeatherData, error) {
	m.called = true
	m.city = city
	m.country = country
	return m.data, m.err
}

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = weatherv1alpha1.AddToScheme(s)
	return s
}

func intPtr(v int) *int { return &v }

func TestReconcile_CRNotFound(t *testing.T) {
	scheme := newTestScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	mock := &mockWeatherFetcher{}
	r := &OpenWeatherReportReconciler{Client: cl, Scheme: scheme, Weather: mock}

	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "does-not-exist"},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("expected no requeue, got %+v", result)
	}
	if mock.called {
		t.Fatal("weather API should not have been called for a missing CR")
	}
}

func TestReconcile_StartupSplay(t *testing.T) {
	scheme := newTestScheme()
	report := &weatherv1alpha1.OpenWeatherReport{
		ObjectMeta: metav1.ObjectMeta{Name: "amsterdam"},
		Spec: weatherv1alpha1.OpenWeatherReportSpec{
			City:    "Amsterdam",
			Country: "NL",
		},
		Status: weatherv1alpha1.OpenWeatherReportStatus{
			Status:      "Valid",
			LastUpdated: "2026-03-27T10:00:00Z", // existing CR from before restart
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(report).
		WithStatusSubresource(report).
		Build()
	mock := &mockWeatherFetcher{}
	r := &OpenWeatherReportReconciler{Client: cl, Scheme: scheme, Weather: mock}

	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "amsterdam"},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.RequeueAfter < 1*time.Second || result.RequeueAfter > 10*time.Second {
		t.Fatalf("expected splay delay between 1s and 10s, got %v", result.RequeueAfter)
	}
	if mock.called {
		t.Fatal("weather API should not be called during startup splay")
	}
}

func TestReconcile_NewCR_NoSplay(t *testing.T) {
	scheme := newTestScheme()
	report := &weatherv1alpha1.OpenWeatherReport{
		ObjectMeta: metav1.ObjectMeta{Name: "amsterdam"},
		Spec: weatherv1alpha1.OpenWeatherReportSpec{
			City:    "Amsterdam",
			Country: "NL",
		},
		// Status intentionally empty → brand new CR, should NOT get splay.
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(report).
		WithStatusSubresource(report).
		Build()
	mock := &mockWeatherFetcher{
		data: &weather.WeatherData{
			Temperature: 18.5,
			FeelsLike:   17.2,
			Humidity:    65,
			Pressure:    1013,
		},
	}
	r := &OpenWeatherReportReconciler{Client: cl, Scheme: scheme, Weather: mock}

	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "amsterdam"},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mock.called {
		t.Fatal("weather API should be called immediately for a new CR")
	}
	if result.RequeueAfter != 60*time.Second {
		t.Fatalf("expected requeue after 60s (default), got %v", result.RequeueAfter)
	}
}

func TestReconcile_SplayOnlyOnce(t *testing.T) {
	scheme := newTestScheme()
	report := &weatherv1alpha1.OpenWeatherReport{
		ObjectMeta: metav1.ObjectMeta{Name: "amsterdam"},
		Spec: weatherv1alpha1.OpenWeatherReportSpec{
			City:    "Amsterdam",
			Country: "NL",
		},
		Status: weatherv1alpha1.OpenWeatherReportStatus{
			Status:      "Valid",
			LastUpdated: "2026-03-27T10:00:00Z",
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(report).
		WithStatusSubresource(report).
		Build()
	mock := &mockWeatherFetcher{
		data: &weather.WeatherData{
			Temperature: 18.5,
			FeelsLike:   17.2,
			Humidity:    65,
			Pressure:    1013,
		},
	}
	r := &OpenWeatherReportReconciler{Client: cl, Scheme: scheme, Weather: mock}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "amsterdam"}}

	// First reconcile: should get splay
	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("first reconcile: expected no error, got %v", err)
	}
	if result.RequeueAfter < 1*time.Second || result.RequeueAfter > 10*time.Second {
		t.Fatalf("first reconcile: expected splay delay, got %v", result.RequeueAfter)
	}
	if mock.called {
		t.Fatal("first reconcile: weather API should not be called during splay")
	}

	// Second reconcile: should proceed normally, no more splay
	result, err = r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("second reconcile: expected no error, got %v", err)
	}
	if !mock.called {
		t.Fatal("second reconcile: weather API should have been called")
	}
	if result.RequeueAfter != 60*time.Second {
		t.Fatalf("second reconcile: expected requeue after 60s, got %v", result.RequeueAfter)
	}
}

func TestReconcile_SuccessfulFetch(t *testing.T) {
	scheme := newTestScheme()
	report := &weatherv1alpha1.OpenWeatherReport{
		ObjectMeta: metav1.ObjectMeta{Name: "amsterdam"},
		Spec: weatherv1alpha1.OpenWeatherReportSpec{
			City:    "Amsterdam",
			Country: "NL",
		},
		Status: weatherv1alpha1.OpenWeatherReportStatus{
			Status: "Valid", // non-empty to skip splay
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(report).
		WithStatusSubresource(report).
		Build()
	mock := &mockWeatherFetcher{
		data: &weather.WeatherData{
			Temperature: 18.5,
			FeelsLike:   17.2,
			Humidity:    65,
			Pressure:    1013,
		},
	}
	r := &OpenWeatherReportReconciler{Client: cl, Scheme: scheme, Weather: mock}

	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "amsterdam"},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.RequeueAfter != 60*time.Second {
		t.Fatalf("expected requeue after 60s (default), got %v", result.RequeueAfter)
	}

	// Verify the status was updated.
	var updated weatherv1alpha1.OpenWeatherReport
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "amsterdam"}, &updated); err != nil {
		t.Fatalf("failed to get updated CR: %v", err)
	}
	assertStatus(t, &updated, statusExpectation{
		Temperature: "18.5°C",
		FeelsLike:   "17.2°C",
		Humidity:    "65%",
		Pressure:    "1013 hPa",
		Location:    "Amsterdam, NL",
		Status:      "Valid",
	})
	if updated.Status.ErrorMessage != "" {
		t.Errorf("expected empty errorMessage, got %q", updated.Status.ErrorMessage)
	}
	if updated.Status.LastUpdated == "" {
		t.Error("expected lastUpdated to be set")
	}
	if _, err := time.Parse(time.RFC3339, updated.Status.LastUpdated); err != nil {
		t.Errorf("lastUpdated is not valid RFC3339: %q", updated.Status.LastUpdated)
	}
}

func TestReconcile_SuccessfulFetch_ClearsError(t *testing.T) {
	scheme := newTestScheme()
	report := &weatherv1alpha1.OpenWeatherReport{
		ObjectMeta: metav1.ObjectMeta{Name: "amsterdam"},
		Spec: weatherv1alpha1.OpenWeatherReportSpec{
			City:    "Amsterdam",
			Country: "NL",
		},
		Status: weatherv1alpha1.OpenWeatherReportStatus{
			Status:       "Error",
			ErrorMessage: "previous failure",
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(report).
		WithStatusSubresource(report).
		Build()
	mock := &mockWeatherFetcher{
		data: &weather.WeatherData{
			Temperature: 20.0,
			FeelsLike:   19.0,
			Humidity:    50,
			Pressure:    1010,
		},
	}
	r := &OpenWeatherReportReconciler{Client: cl, Scheme: scheme, Weather: mock}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "amsterdam"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var updated weatherv1alpha1.OpenWeatherReport
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "amsterdam"}, &updated); err != nil {
		t.Fatalf("failed to get updated CR: %v", err)
	}
	if updated.Status.Status != "Valid" {
		t.Errorf("expected status Valid, got %q", updated.Status.Status)
	}
	if updated.Status.ErrorMessage != "" {
		t.Errorf("expected errorMessage cleared, got %q", updated.Status.ErrorMessage)
	}
}

func TestReconcile_FetchError(t *testing.T) {
	scheme := newTestScheme()
	report := &weatherv1alpha1.OpenWeatherReport{
		ObjectMeta: metav1.ObjectMeta{Name: "amsterdam"},
		Spec: weatherv1alpha1.OpenWeatherReportSpec{
			City:    "Amsterdam",
			Country: "NL",
		},
		Status: weatherv1alpha1.OpenWeatherReportStatus{
			Status: "Valid", // non-empty to skip splay
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(report).
		WithStatusSubresource(report).
		Build()
	mock := &mockWeatherFetcher{
		err: fmt.Errorf("API error: HTTP 401"),
	}
	r := &OpenWeatherReportReconciler{Client: cl, Scheme: scheme, Weather: mock}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "amsterdam"},
	})

	if err == nil {
		t.Fatal("expected an error to be returned for controller-runtime backoff")
	}

	var updated weatherv1alpha1.OpenWeatherReport
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "amsterdam"}, &updated); err != nil {
		t.Fatalf("failed to get updated CR: %v", err)
	}
	if updated.Status.Status != "Error" {
		t.Errorf("expected status Error, got %q", updated.Status.Status)
	}
	if updated.Status.ErrorMessage != "API error: HTTP 401" {
		t.Errorf("expected errorMessage %q, got %q", "API error: HTTP 401", updated.Status.ErrorMessage)
	}
}

func TestReconcile_CustomInterval(t *testing.T) {
	scheme := newTestScheme()
	report := &weatherv1alpha1.OpenWeatherReport{
		ObjectMeta: metav1.ObjectMeta{Name: "amsterdam"},
		Spec: weatherv1alpha1.OpenWeatherReportSpec{
			City:            "Amsterdam",
			Country:         "NL",
			IntervalSeconds: intPtr(120),
		},
		Status: weatherv1alpha1.OpenWeatherReportStatus{
			Status: "Valid",
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(report).
		WithStatusSubresource(report).
		Build()
	mock := &mockWeatherFetcher{
		data: &weather.WeatherData{
			Temperature: 15.0,
			FeelsLike:   14.0,
			Humidity:    70,
			Pressure:    1020,
		},
	}
	r := &OpenWeatherReportReconciler{Client: cl, Scheme: scheme, Weather: mock}

	result, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "amsterdam"},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.RequeueAfter != 120*time.Second {
		t.Fatalf("expected requeue after 120s, got %v", result.RequeueAfter)
	}
}

func TestReconcile_PassesCityAndCountry(t *testing.T) {
	scheme := newTestScheme()
	report := &weatherv1alpha1.OpenWeatherReport{
		ObjectMeta: metav1.ObjectMeta{Name: "tokyo"},
		Spec: weatherv1alpha1.OpenWeatherReportSpec{
			City:    "Tokyo",
			Country: "JP",
		},
		Status: weatherv1alpha1.OpenWeatherReportStatus{
			Status: "Valid",
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(report).
		WithStatusSubresource(report).
		Build()
	mock := &mockWeatherFetcher{
		data: &weather.WeatherData{Temperature: 25.0, FeelsLike: 26.0, Humidity: 80, Pressure: 1005},
	}
	r := &OpenWeatherReportReconciler{Client: cl, Scheme: scheme, Weather: mock}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "tokyo"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.city != "Tokyo" {
		t.Errorf("expected city Tokyo, got %q", mock.city)
	}
	if mock.country != "JP" {
		t.Errorf("expected country JP, got %q", mock.country)
	}
}

func TestEffectiveInterval(t *testing.T) {
	tests := []struct {
		name     string
		input    *int
		expected time.Duration
	}{
		{"nil defaults to 60s", nil, 60 * time.Second},
		{"5s minimum", intPtr(5), 5 * time.Second},
		{"custom 30s", intPtr(30), 30 * time.Second},
		{"below minimum falls back to 60s", intPtr(4), 60 * time.Second},
		{"zero falls back to 60s", intPtr(0), 60 * time.Second},
		{"negative falls back to 60s", intPtr(-1), 60 * time.Second},
		{"large value accepted", intPtr(3600), 3600 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveInterval(tt.input)
			if got != tt.expected {
				t.Errorf("effectiveInterval(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// statusExpectation holds expected status field values for assertion.
type statusExpectation struct {
	Temperature string
	FeelsLike   string
	Humidity    string
	Pressure    string
	Location    string
	Status      string
}

func assertStatus(t *testing.T, report *weatherv1alpha1.OpenWeatherReport, want statusExpectation) {
	t.Helper()
	checks := []struct {
		field string
		got   string
		want  string
	}{
		{"temperature", report.Status.Temperature, want.Temperature},
		{"feelsLike", report.Status.FeelsLike, want.FeelsLike},
		{"humidity", report.Status.Humidity, want.Humidity},
		{"pressure", report.Status.Pressure, want.Pressure},
		{"location", report.Status.Location, want.Location},
		{"status", report.Status.Status, want.Status},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("status.%s = %q, want %q", c.field, c.got, c.want)
		}
	}
}
