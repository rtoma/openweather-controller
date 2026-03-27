# Active Context

## Current Focus

Splay bug fixed. Controller unit tests complete. Ready to write envtest integration tests or tackle remaining items (Dockerfile, CI, kustomize manifests, README).

## Recent Decisions

- CRD name: `OpenWeatherReport` (group `weather.io`, version `v1alpha1`)
- Cluster-scoped CRD (not namespaced)
- Framework: kubebuilder v4.13.1 with full scaffolding (no OLM)
- API group fixed to `weather.io` (not `weather.weather.io`)
- API key via env var `OPENWEATHER_API_KEY`
- Default poll interval 60s; per-CR override via `spec.intervalSeconds` (minimum 5s)
- On API failure: exponential backoff requeue + set `status.status=Error` + `status.errorMessage`
- Status includes `lastUpdated` (RFC3339 timestamp)
- `kubectl get openweatherreport` shows: Location, Temperature, Humidity, Status, Age
- Location column = single `status.location` field set by controller
- Kustomize for packaging; multi-arch image (`linux/amd64`, `linux/arm64`) on `ghcr.io/rtoma/openweather-controller`
- Kubernetes target: >= 1.31
- Tests: envtest + kind
- Startup splay: 1–10s random delay per CR on first reconcile to avoid thundering herd

## Next Steps (unsorted)

- Write envtest integration tests for controller
- ~~Write Dockerfile (multi-stage, multi-arch)~~ Done
- Write GitHub Actions workflow for image build + push to ghcr.io
- Write kustomize manifests for deployment (ClusterRole, Deployment with OPENWEATHER_API_KEY env, CRD)
- Write README with usage instructions
