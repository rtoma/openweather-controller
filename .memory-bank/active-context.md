# Active Context

## Current Focus

Permanent error handling implemented. Controller no longer retries terminal 404 errors. Ready to tackle remaining items: GitHub Actions CI, kustomize manifests, README.

## Recent Decisions

- CRD name: `OpenWeatherReport` (group `weather.io`, version `v1alpha1`)
- Cluster-scoped CRD (not namespaced)
- Framework: kubebuilder v4.13.1 with full scaffolding (no OLM)
- API group fixed to `weather.io` (not `weather.weather.io`)
- API key via env var `OPENWEATHER_API_KEY`
- Default poll interval 60s; per-CR override via `spec.intervalSeconds` (minimum 5s)
- On API failure: exponential backoff requeue + set `status.status=Error` + `status.errorMessage`
- Status includes `lastUpdated` (RFC3339 timestamp)
- `kubectl get openweatherreport` shows: Location, Temperature, FeelsLike, Humidity, Pressure, Status, Age
- Location column = single `status.location` field set by controller
- Kustomize for packaging; multi-arch image (`linux/amd64`, `linux/arm64`) on `ghcr.io/rtoma/openweather-controller`
- Kubernetes target: >= 1.31
- Tests: envtest + kind
- Startup splay: 1–10s random delay per existing CR on first reconcile to avoid thundering herd; new CRs reconcile immediately

## Next Steps (unsorted)

- ~~Write envtest integration tests for controller~~ Done
- ~~Write Dockerfile (multi-stage, multi-arch)~~ Done
- ~~Write kustomize manifests for deployment~~ Done
- ~~Write GitHub Actions workflow for image build + push to ghcr.io~~ Done
- ~~Write README with usage instructions~~ Done
- Add additional weather fields: `wind.speed`, `wind.deg`, `weather[0].description`, `clouds.all` — extend WeatherData, status struct, kubectl get columns, and tests. Low complexity, self-contained.
- Adopt Kubernetes-standard status conditions (`metav1.Condition`) replacing custom `status.status` / `status.errorMessage` strings. Enables `kubectl wait`, follows API conventions. Medium complexity — API type + controller + tests.
- Add validating/defaulting webhook: reject blank city/country, reject intervalSeconds < 5, default intervalSeconds to 60. Gives immediate feedback at `kubectl apply` time instead of silent Error status. Medium complexity — adds cert-manager dependency.
- Lifecycle gap: add `status.observedGeneration` (set to `metadata.generation` after each reconcile). Standard Kubernetes convention; lets users/tools tell whether the controller has processed the current spec. Low complexity — one field + one line in controller.
- Lifecycle gap: `seen` sync.Map leaks entries for deleted CRs — grows unbounded if CRs are created/deleted frequently. Fix with a `DeleteFunc` predicate event handler or watch deletion events to prune the map. Low complexity.
- Lifecycle gap: status update after slow external API call risks resource-version conflict — controller fetches CR, calls weather API, then Status().Update() may fail if CR was modified in the meantime, causing a redundant second API call on retry. Fix with a fresh Get before the update. Low complexity.
- Dependency update: run `go get -u k8s.io/apimachinery k8s.io/client-go golang.org/x/crypto golang.org/x/net github.com/go-jose/go-jose/v4 && go mod tidy` — k8s patch releases (0.35.0→0.35.3), crypto/net security updates, go-jose minor update.
- Dockerfile: pin builder from `golang:1.25` to `golang:1.25.3` to match go.mod and avoid surprise builder upgrades.
