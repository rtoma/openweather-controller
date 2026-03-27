# Progress

This file contains a log of everything we have done so far. It serves as a cold storage journal, in case we need to understand what we did in the past.

# Journal

## 2026-03-27 17:09

Fixed "build and push image" workflow Docker Hub rate limit error:
- `docker/setup-qemu-action` was pulling `tonistiigi/binfmt:latest` from Docker Hub
- Added `image: ghcr.io/tonistiigi/binfmt:latest` override to pull from GHCR instead

## 2026-03-27 17:04

Fixed `make lint` issues (0 issues now):
- `goconst`: Added `const statusError = "Error"` in `openweatherreport_controller.go`; replaced all 6 string literals across controller and test files.
- `staticcheck`: Removed deprecated `result.Requeue` from `reconcile_unit_test.go:375`, keeping only `result.RequeueAfter != 0`.

## 2026-03-27 17:00

Rewrote README.md from the kubebuilder scaffold stub to a publish-ready GitHub README. New content includes: CI badge row, project description, `kubectl get` output example, Quick Start (Secret → deploy → create resource), CRD spec/status reference tables, full-field YAML example, reconciliation behaviour notes, development guide (local run, custom image build, test commands, make targets), uninstall steps, container image tags, and contributing/license sections. All TODO stubs removed.

## 2026-03-27 16:55

Added `FeelsLike` and `Pressure` as `+kubebuilder:printcolumn` markers in `api/v1alpha1/openweatherreport_types.go`:
- `FeelsLike` inserted immediately after `Temperature`
- `Pressure` inserted immediately after `Humidity`
- Column order is now: Location, Temperature, FeelsLike, Humidity, Pressure, Status, Age
- `make manifests` needs to be run to regenerate the CRD YAML

## 2026-03-27 16:52

Created `.github/workflows/image.yml` — GitHub Actions workflow for building and pushing the Docker image to ghcr.io:
- Triggers on push to `main`, version tags (`v*`), and PRs to `main`
- Sets up QEMU + Docker Buildx for multi-arch builds (`linux/amd64`, `linux/arm64`)
- Logs in to ghcr.io using `GITHUB_TOKEN` (skip on PRs)
- Uses `docker/metadata-action` to generate tags: branch name, semver (`v1.2.3`, `1.2`), and short SHA
- Uses GHA layer cache (`type=gha`) for fast rebuilds
- Skips push on pull_request events (build-only validation)

## 2026-03-27 16:51

Written kustomize manifests for deployment:
- Created `config/manager/api-key-secret.yaml` — Secret template with placeholder value; comments explain how to replace or use `kubectl create secret` instead.
- Updated `config/manager/kustomization.yaml` — added `api-key-secret.yaml` to resources.
- Created `config/default/api_key_patch.yaml` — JSON-patch Deployment to inject `OPENWEATHER_API_KEY` env var from the Secret (uses full prefixed name `openweather-controller-openweather-api-key` to match kustomize's namePrefix).
- Updated `config/default/kustomization.yaml` — added `api_key_patch.yaml` to patches (listed first, before metrics patch).
- Verified `bin/kustomize build config/default` produces valid YAML with correct Secret name and env var in Deployment.

## 2026-03-27 16:42

Implemented permanent error handling for terminal API errors (e.g. HTTP 404 city not found):
- Added `APIError` struct to `internal/weather/client.go` with `StatusCode`, `Message`, `IsPermanent()` method.
- Weather client now returns `*APIError` for HTTP errors instead of plain `fmt.Errorf` strings.
- Controller checks `errors.As(err, &apiErr) && apiErr.IsPermanent()` — if true, logs info (not error), updates status to Error, and returns `ctrl.Result{}, nil` (no requeue, no retry).
- Transient errors (e.g. 401, 500) still trigger exponential backoff via controller-runtime as before.
- Added `TestReconcile_PermanentError_CityNotFound` unit test. Updated existing `TestReconcile_FetchError` to use `*weather.APIError`.
- All tests pass (unit + envtest).

## 2026-03-27 16:25

Wrote comprehensive envtest integration tests with a running controller manager.

**Changes:**
- `internal/controller/suite_test.go`: Rewrote to start a real controller manager with a `configurableFetcher` (thread-safe, configurable mock). Disabled metrics server in tests. Manager runs in background goroutine via `mgr.Start(ctx)`.
- `internal/controller/openweatherreport_controller_test.go`: Replaced the single manual-reconcile test with 7 `Eventually`-based integration test scenarios:
  1. Happy path — CR created → status Valid with all weather fields populated
  2. Correct city/country passed to weather API
  3. API error → status Error with errorMessage
  4. Error recovery — API fails then succeeds → Error → Valid, errorMessage cleared
  5. Custom intervalSeconds — works correctly, re-reconciles at 5s interval
  6. CR deletion — handled gracefully, CR removed from cluster
  7. Multiple CRs — both reconciled independently

**Results:** All 7 envtest integration tests + all 10 unit tests pass (13.9s total).

## 2026-03-27 16:13

Fixed `make test` failure — the envtest integration test (`openweatherreport_controller_test.go`) was failing because:
- CR was created without required `spec.city` and `spec.country` fields (CRD validation rejected it)
- CR had `Namespace: "default"` but the CRD is cluster-scoped
- Reconciler was missing the `Weather` (WeatherFetcher) dependency

Fixed by: populating spec fields (Amsterdam, NL), removing namespace, adding a `fakeWeatherFetcher`, and adding status assertions. All tests now pass (84.2% controller coverage, 91.7% weather client coverage).

## 2026-03-27 16:10

Fixed all 8 golangci-lint issues (`make lint` now passes with 0 issues):
- `errcheck`: wrapped `resp.Body.Close()` in defer closure with `_ =` in `internal/weather/client.go`
- `errcheck`: added `_ =` to 3 `json.NewEncoder().Encode()` calls and 2 `w.Write()` calls in `internal/weather/client_test.go`
- `staticcheck` QF1008: removed embedded field `NamespacedName` from selector in `internal/controller/openweatherreport_controller.go` (used `req.String()` instead of `req.NamespacedName.String()`)
- `staticcheck` SA1019: removed deprecated `result.Requeue` check in `internal/controller/reconcile_unit_test.go`

Note: envtest integration test (`TestControllers`) has a pre-existing failure — CR created without required `city`/`country` fields.

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

