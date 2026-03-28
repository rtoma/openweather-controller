# Product Context

Kubernetes controller for looking up weather information for cities.

Published at: https://github.com/rtoma/openweather-controller

## Source

OpenWeather API (https://openweathermap.org/api) is used as the data source.
The API key is provided via environment variable `OPENWEATHER_API_KEY`.

## CRD

Custom resource definition name: **OpenWeatherReport** (group: `weather.io`, version: `v1alpha1`).
Scope: **cluster-scoped** (not namespaced).

### Spec fields

| Field            | Type   | Required | Notes                                  |
|------------------|--------|----------|----------------------------------------|
| `city`           | string | yes      | City name                              |
| `country`        | string | yes      | Country code (e.g. `NL`)               |
| `intervalSeconds`| int    | no       | Poll interval override; minimum 5s; defaults to 60s |

### Status fields

| Field           | Type   | Notes                                              |
|-----------------|--------|----------------------------------------------------|
| `temperature`   | string | In Celsius                                         |
| `feelsLike`     | string | In Celsius                                         |
| `humidity`      | string | Percentage                                         |
| `pressure`      | string | hPa                                                |
| `status`        | enum   | `Valid` or `Error`                                 |
| `errorMessage`  | string | Populated when status is `Error`; empty otherwise  |
| `lastUpdated`   | string | RFC3339 timestamp of last successful API call      |

### kubectl get columns (additionalPrinterColumns)

`kubectl get openweatherreport` shows: **Location** (city+country), **Temperature**, **FeelsLike**, **Humidity**, **Pressure**, **Status**, **Age** (based on `lastUpdated`).

## Reconciliation behaviour

- **New CR**: reconcile immediately.
- **Existing CR**: requeue after `intervalSeconds` (default 60s, minimum 5s).
- **Startup splay**: one-time random delay of 1–10 seconds added per existing CR at controller start to avoid thundering herd. New CRs are reconciled immediately.
- **API failure**: retry with exponential backoff; set `status.status=Error` and populate `status.errorMessage`.

## Deployment

- Packaged with **kustomize** (plain YAML manifests).
- Container image published to `ghcr.io/rtoma/openweather-controller`.
- Multi-arch build: `linux/amd64` and `linux/arm64`.
- Minimum Kubernetes version: **1.31**.
