# System Architecture

## Overview

A Kubernetes controller built with kubebuilder (full scaffolding). It watches cluster-scoped `OpenWeatherReport` CRs and periodically fetches weather data from the OpenWeather API, writing results back to the CR status.

## Tech Stack

- **Language**: Go (latest stable)
- **Framework**: kubebuilder (full scaffold) using `controller-runtime` under the hood
- **Kubernetes target**: >= 1.31
- **CRD scope**: Cluster-scoped
- **Testing**: envtest (kubebuilder) + kind for integration tests; Go unit tests for business logic
- **Packaging**: kustomize (`config/` directory, standard kubebuilder layout)
- **Image**: `ghcr.io/rtoma/openweather-controller`, multi-arch (`linux/amd64`, `linux/arm64`)
- **CI/CD**: GitHub Actions (image build + push to ghcr.io)

## Directory Structure (kubebuilder standard)

```
.
├── api/
│   └── v1alpha1/
│       ├── openweatherreport_types.go   # CRD spec/status structs
│       └── zz_generated.deepcopy.go
├── config/
│   ├── crd/                             # Generated CRD manifests
│   ├── default/                         # Kustomize base
│   ├── manager/                         # Controller Deployment
│   └── rbac/                            # ClusterRole etc.
├── internal/
│   └── controller/
│       └── openweatherreport_controller.go
├── internal/
│   └── weather/
│       └── client.go                    # OpenWeather API client
├── cmd/
│   └── main.go
├── Dockerfile
├── Makefile
└── .memory-bank/
```

## Data Model

### OpenWeatherReportSpec
```go
type OpenWeatherReportSpec struct {
    City            string `json:"city"`
    Country         string `json:"country"`
    IntervalSeconds *int   `json:"intervalSeconds,omitempty"` // min 5, default 60
}
```

### OpenWeatherReportStatus
```go
type OpenWeatherReportStatus struct {
    Temperature  string `json:"temperature,omitempty"`
    FeelsLike    string `json:"feelsLike,omitempty"`
    Humidity     string `json:"humidity,omitempty"`
    Pressure     string `json:"pressure,omitempty"`
    Status       string `json:"status,omitempty"`       // "Valid" | "Error"
    ErrorMessage string `json:"errorMessage,omitempty"`
    LastUpdated  string `json:"lastUpdated,omitempty"` // RFC3339
}
```

### additionalPrinterColumns
Shown by `kubectl get openweatherreport`:
- **Location**: `.status.location` (set by controller as `"<city>, <country>"`)
- **Temperature**: `.status.temperature`
- **FeelsLike**: `.status.feelsLike`
- **Humidity**: `.status.humidity`
- **Pressure**: `.status.pressure`
- **Status**: `.status.status`
- **Age**: `.status.lastUpdated` (type `date`)

## Key Patterns

### Reconcile loop
1. Fetch `OpenWeatherReport` CR. If not found, return (deleted).
2. Call OpenWeather API client.
3. On success: update status with weather data, set `status=Valid`, clear `errorMessage`, set `lastUpdated=now`. Requeue after `effectiveInterval`.
4. On failure: set `status=Error`, set `errorMessage`, requeue with exponential backoff.
5. `effectiveInterval = max(5, spec.intervalSeconds ?? 60)`.

### Startup splay
- Controller tracks seen CRs via `sync.Map`. On first reconcile after restart, if `LastUpdated` is non-empty (existing CR), a random 1–10s delay is applied. New CRs (empty `LastUpdated`) are reconciled immediately. The splay fires only once per CR per controller lifetime.

### API key
- Read from env var `OPENWEATHER_API_KEY` at startup; fatal if missing.

### Error handling
- API errors → `status.status = "Error"`, `status.errorMessage = <error string>`.
- **Permanent errors** (HTTP 404 city not found) → status updated, no requeue, no retry. Uses `weather.APIError.IsPermanent()`.
- **Transient errors** (other HTTP codes, network errors) → requeue with exponential backoff (controller-runtime).
- Status update failures → return error (controller-runtime will requeue).

## OpenWeather API

- Endpoint: `https://api.openweathermap.org/data/2.5/weather?q={city},{country}&appid={key}&units=metric`
- Response fields used: `main.temp`, `main.feels_like`, `main.humidity`, `main.pressure`.
