# Progress

This file contains a log of everything we have done so far. It serves as a cold storage journal, in case we need to understand what we did in the past.

# Journal

## 2026-03-27 15:59

Dockerfile completed (multi-stage, multi-arch ready):
- Two-stage build: golang:1.25 builder → gcr.io/distroless/static:nonroot runtime
- CGO_ENABLED=0 with TARGETOS/TARGETARCH for cross-compilation
- Fixed `.dockerignore` to explicitly re-include `cmd/`, `api/`, `internal/` directories
- Fixed `go build` path from `cmd/main.go` to `./cmd/` (Go 1.25 compat)
- Updated Makefile: IMG default to `ghcr.io/rtoma/openweather-controller:latest`, platforms to arm64+amd64 only, sed-based Dockerfile.cross for buildx
- Verified: image builds successfully, ~30MB final size, linux/arm64

## 2026-03-27 15:47

Fixed splay implementation — was applying to new CRs (wrong), now applies to existing CRs on controller restart (correct).

- **Bug**: splay triggered when `LastUpdated=="" && Status==""` (i.e. new CRs). New CRs don't need splay; existing CRs at controller restart do.
- **Fix**: Added `seen sync.Map` field to reconciler. On first encounter of a CR:
  - If `LastUpdated != ""` (existing CR from before restart) → apply 1–10s random splay
  - If `LastUpdated == ""` (brand new CR) → reconcile immediately
  - Subsequent reconciles for the same CR → no splay (already in `seen` map)
- Updated tests:
  - `TestReconcile_StartupSplay` — now tests existing CR with `LastUpdated` set gets splay
  - `TestReconcile_NewCR_NoSplay` — new test: brand new CR is fetched immediately
  - `TestReconcile_SplayOnlyOnce` — new test: second reconcile of same CR skips splay
- All 12 unit tests passing

## 2026-03-27 15:43

Added unit tests for controller reconcile logic in `internal/controller/reconcile_unit_test.go`.
- Uses standard Go `testing` (no envtest dependency) with `fake.NewClientBuilder` and a mock `WeatherFetcher`.
- 8 test functions, 15 test cases total, all passing:
  - `TestReconcile_CRNotFound` — deleted CR returns no-requeue
  - `TestReconcile_StartupSplay` — new CR triggers 1–10s random delay
  - `TestReconcile_SuccessfulFetch` — weather data written to status, default 60s requeue
  - `TestReconcile_SuccessfulFetch_ClearsError` — previous Error status is cleared on success
  - `TestReconcile_FetchError` — status set to Error, error returned for backoff
  - `TestReconcile_CustomInterval` — custom intervalSeconds honored in requeue
  - `TestReconcile_PassesCityAndCountry` — city/country forwarded to weather client
  - `TestEffectiveInterval` — 7 sub-tests covering nil, minimum, below-minimum, negative, large values

## 2026-03-27 15:38

Implemented reconcile loop and wired up weather client.

- Implemented full `Reconcile` method in `internal/controller/openweatherreport_controller.go`:
  - Fetches CR, returns nil if deleted (not found)
  - Startup splay: 1–10s random delay on first reconcile (when `lastUpdated` and `status` are empty)
  - Calls weather API via `WeatherFetcher` interface for testability
  - On success: sets temperature, feelsLike, humidity, pressure, location, status=Valid, clears errorMessage, sets lastUpdated (RFC3339)
  - On failure: sets status=Error + errorMessage, returns error for exponential backoff
  - Requeues after effective interval (spec.intervalSeconds or 60s default, minimum 5s)
- Added `WeatherFetcher` interface to decouple controller from concrete weather client
- Added `effectiveInterval()` helper function
- Updated `cmd/main.go`: reads `OPENWEATHER_API_KEY` env var (fatal if missing), creates `weather.Client`, passes to reconciler as `Weather` field
- Build passes, all 9 weather client tests still pass

## 2026-03-27 15:32

Implemented OpenWeather API client and unit tests.

- Created `internal/weather/client.go`: HTTP client with `FetchWeather(ctx, city, country)` method
  - Calls `api.openweathermap.org/data/2.5/weather` with metric units
  - Returns `WeatherData` struct (Temperature, FeelsLike, Humidity, Pressure)
  - Functional options pattern: `WithBaseURL()`, `WithHTTPClient()` for testability
  - 10s HTTP timeout, 1MiB response body limit, proper error messages from API
- Created `internal/weather/client_test.go`: 9 tests all passing
  - Success path, API errors (401, 404), invalid JSON, server down, context cancellation, HTTP 500 without message body, client defaults, client options

## 2026-03-27 15:01

Project scaffolded and CRD types implemented.

- Ran `kubebuilder init --domain weather.io --repo github.com/rtoma/openweather-controller`
- Ran `kubebuilder create api --group weather --version v1alpha1 --kind OpenWeatherReport --resource --controller`
- Fixed API group from `weather.weather.io` → `weather.io` (group+domain concatenation issue)
- Implemented `OpenWeatherReportSpec`: `city` (required, minLen=1), `country` (required, 2 chars), `intervalSeconds` (optional, min=5)
- Implemented `OpenWeatherReportStatus`: `temperature`, `feelsLike`, `humidity`, `pressure`, `location`, `status` (Valid/Error enum), `errorMessage`, `lastUpdated`
- Added kubebuilder markers: `scope=Cluster`, printer columns (Location, Temperature, Humidity, Status, Age)
- Generated manifests and deepcopy; build passes cleanly
- Updated sample CR with `city: Amsterdam, country: NL`

## 2026-03-27 14:52

Design interview completed. No code written yet.

Decisions captured:
- CRD: `OpenWeatherReport`, cluster-scoped, group `weather.io/v1alpha1`
- API key from `OPENWEATHER_API_KEY` env var
- Spec: `city`, `country`, `intervalSeconds` (optional, min 5s, default 60s)
- Status: `temperature`, `feelsLike`, `humidity`, `pressure`, `status` (Valid/Error), `errorMessage`, `lastUpdated`, `location`
- `kubectl get` printer columns: Location, Temperature, Humidity, Status, Age
- Framework: kubebuilder full scaffolding, no OLM
- Packaging: kustomize; image `ghcr.io/rtoma/openweather-controller` (amd64 + arm64)
- Kubernetes >= 1.31
- Tests: envtest + kind
- Startup splay: 1-10s random delay on first reconcile

Memory bank files updated: product-context.md, system-architecture.md, active-context.md

