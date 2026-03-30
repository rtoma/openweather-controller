# openweather-controller

[![Build and Push Image](https://github.com/rtoma/openweather-controller/actions/workflows/image.yml/badge.svg)](https://github.com/rtoma/openweather-controller/actions/workflows/image.yml)
[![Lint](https://github.com/rtoma/openweather-controller/actions/workflows/lint.yml/badge.svg)](https://github.com/rtoma/openweather-controller/actions/workflows/lint.yml)
[![Tests](https://github.com/rtoma/openweather-controller/actions/workflows/test.yml/badge.svg)](https://github.com/rtoma/openweather-controller/actions/workflows/test.yml)
[![E2E Tests](https://github.com/rtoma/openweather-controller/actions/workflows/test-e2e.yml/badge.svg)](https://github.com/rtoma/openweather-controller/actions/workflows/test-e2e.yml)

A Kubernetes controller that watches `OpenWeatherReport` custom resources and periodically fetches weather data from the [OpenWeather API](https://openweathermap.org/api), writing results back to each resource's status.

Note: this project is using my [AI agent Memory Bank Protocol](https://github.com/rtoma/agent-markdown-memory-bank-protocol).

## How it works

You declare which cities you want to monitor as cluster-scoped `OpenWeatherReport` resources. The controller continuously polls the OpenWeather API at a configurable interval and updates the resource status with current weather data — temperature, feels-like, humidity, and pressure.

```
kubectl get openweatherreport

NAME        LOCATION            TEMPERATURE   FEELSLIKE   HUMIDITY   PRESSURE   STATUS   AGE
amsterdam   Amsterdam, NL       14.2°C        13.1°C      72%        1015 hPa   Valid    2m
new-york    New York City, US   22.5°C        21.8°C      55%        1012 hPa   Valid    2m
```

## Prerequisites

- Kubernetes >= 1.31
- `kubectl`
- An [OpenWeather API key](https://openweathermap.org/appid) (free tier works)

## Quick Start

### 1. Create the API key Secret

```sh
kubectl create namespace openweather-controller-system

kubectl create secret generic openweather-api-key \
  --from-literal=api-key=<YOUR_OPENWEATHER_API_KEY> \
  -n openweather-controller-system
```

### 2. Deploy the controller

```sh
kubectl apply -k https://github.com/rtoma/openweather-controller/config/default
```

This installs the CRD, RBAC, and the controller Deployment in the `openweather-controller-system` namespace.

Verify the controller is running:

```sh
kubectl get pods -n openweather-controller-system
```

### 3. Create an OpenWeatherReport

```yaml
apiVersion: weather.io/v1alpha1
kind: OpenWeatherReport
metadata:
  name: amsterdam
spec:
  city: Amsterdam
  country: NL
```

```sh
kubectl apply -f my-report.yaml
```

Within a few seconds the controller will fetch weather data and populate the status:

```sh
kubectl get openweatherreport amsterdam -o yaml
```

```yaml
status:
  location: "Amsterdam, NL"
  temperature: "14.2°C"
  feelsLike: "13.1°C"
  humidity: "72%"
  pressure: "1015 hPa"
  status: Valid
  lastUpdated: "2026-03-27T10:00:00Z"
```

## CRD Reference

### OpenWeatherReport

**API group/version:** `weather.io/v1alpha1`  
**Scope:** Cluster-scoped (no namespace required)

#### Spec

| Field             | Type    | Required | Description |
|-------------------|---------|----------|-------------|
| `city`            | string  | yes      | City name (e.g. `Amsterdam`) |
| `country`         | string  | yes      | ISO 3166-1 alpha-2 country code (e.g. `NL`) |
| `intervalSeconds` | integer | no       | Poll interval in seconds. Minimum `5`, default `60` |

#### Status

| Field          | Type   | Description |
|----------------|--------|-------------|
| `temperature`  | string | Current temperature in Celsius |
| `feelsLike`    | string | Feels-like temperature in Celsius |
| `humidity`     | string | Relative humidity percentage |
| `pressure`     | string | Atmospheric pressure in hPa |
| `location`     | string | `"<city>, <country>"` — displayed by `kubectl get` |
| `status`       | string | `Valid` or `Error` |
| `errorMessage` | string | Populated when `status` is `Error`; empty otherwise |
| `lastUpdated`  | string | RFC3339 timestamp of the last successful API call |

### Example with all fields

```yaml
apiVersion: weather.io/v1alpha1
kind: OpenWeatherReport
metadata:
  name: london
spec:
  city: London
  country: GB
  intervalSeconds: 120   # poll every 2 minutes instead of the default 60s
```

## Reconciliation behaviour

- **New resources** are reconciled immediately.
- **Existing resources** are requeued after `intervalSeconds` (default 60 s, minimum 5 s).
- **Startup splay**: on controller restart a random 1–10 s delay is added per existing resource to avoid thundering-herd requests to the API.
- **Transient API errors** (network issues, 5xx responses) trigger exponential backoff retries; status is set to `Error`.
- **Permanent API errors** (city not found / HTTP 404) update the status to `Error` and stop retrying — fix the city/country in the spec to resume.

## Development

### Prerequisites

- Go >= 1.24
- Docker (with buildx for multi-arch builds)
- `make`

### Run locally against a cluster

```sh
# Export your API key
export OPENWEATHER_API_KEY=<your-key>

# Install CRDs into the cluster pointed to by ~/.kube/config
make install

# Run the controller locally (uses your current kubeconfig)
make run
```

### Build and push a custom image

```sh
make docker-build docker-push IMG=<registry>/openweather-controller:<tag>
make deploy IMG=<registry>/openweather-controller:<tag>
```

### Run tests

```sh
# Unit and integration tests
make test

# End-to-end tests (requires a running cluster)
make test-e2e
```

### Other useful make targets

```sh
make help          # List all available targets
make lint          # Run golangci-lint
make manifests     # Regenerate CRD manifests
make generate      # Regenerate DeepCopy methods
```

## Uninstall

```sh
# Delete all OpenWeatherReport resources
kubectl delete openweatherreports --all

# Remove the controller and CRD
make undeploy
make uninstall
```

## Container image

Pre-built multi-arch images (`linux/amd64`, `linux/arm64`) are published to GitHub Container Registry on every push to `main` and on version tags:

```
ghcr.io/rtoma/openweather-controller:main
ghcr.io/rtoma/openweather-controller:v1.0.0
```

## Contributing

Bug reports and pull requests are welcome. Please open an issue first to discuss significant changes.

Run `make help` for a full list of available make targets.

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

