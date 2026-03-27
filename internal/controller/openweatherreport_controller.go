/*
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
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	weatherv1alpha1 "github.com/rtoma/openweather-controller/api/v1alpha1"
	"github.com/rtoma/openweather-controller/internal/weather"
)

const defaultIntervalSeconds = 60
const statusError = "Error"

// WeatherFetcher abstracts weather data retrieval for testability.
type WeatherFetcher interface {
	FetchWeather(ctx context.Context, city, country string) (*weather.WeatherData, error)
}

// OpenWeatherReportReconciler reconciles a OpenWeatherReport object
type OpenWeatherReportReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Weather WeatherFetcher
	seen    sync.Map // tracks CRs reconciled since controller start
}

// +kubebuilder:rbac:groups=weather.io,resources=openweatherreports,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=weather.io,resources=openweatherreports/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=weather.io,resources=openweatherreports/finalizers,verbs=update

// Reconcile fetches weather data from the OpenWeather API and updates the CR status.
func (r *OpenWeatherReportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the CR.
	var report weatherv1alpha1.OpenWeatherReport
	if err := r.Get(ctx, req.NamespacedName, &report); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Startup splay: when the controller restarts, stagger existing CRs to avoid
	// a thundering herd. New CRs (empty status) are reconciled immediately.
	if _, alreadySeen := r.seen.LoadOrStore(req.String(), true); !alreadySeen {
		if report.Status.LastUpdated != "" {
			delay := time.Duration(rand.IntN(10)+1) * time.Second
			log.Info("Startup splay: staggering existing CR", "delay", delay)
			return ctrl.Result{RequeueAfter: delay}, nil
		}
	}

	// Call the weather API.
	data, err := r.Weather.FetchWeather(ctx, report.Spec.City, report.Spec.Country)
	if err != nil {
		report.Status.Status = statusError
		report.Status.ErrorMessage = err.Error()
		if updateErr := r.Status().Update(ctx, &report); updateErr != nil {
			log.Error(updateErr, "Failed to update status after API error")
			return ctrl.Result{}, updateErr
		}

		// Permanent errors (e.g. city not found) are terminal — don't retry.
		var apiErr *weather.APIError
		if errors.As(err, &apiErr) && apiErr.IsPermanent() {
			log.Info("Permanent API error, will not retry", "error", err.Error())
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to fetch weather data")
		// Exponential backoff is handled by controller-runtime when we return an error.
		return ctrl.Result{}, err
	}

	// Success: update status with weather data.
	report.Status.Temperature = fmt.Sprintf("%.1f°C", data.Temperature)
	report.Status.FeelsLike = fmt.Sprintf("%.1f°C", data.FeelsLike)
	report.Status.Humidity = fmt.Sprintf("%d%%", data.Humidity)
	report.Status.Pressure = fmt.Sprintf("%d hPa", data.Pressure)
	report.Status.Location = fmt.Sprintf("%s, %s", report.Spec.City, report.Spec.Country)
	report.Status.Status = "Valid"
	report.Status.ErrorMessage = ""
	report.Status.LastUpdated = time.Now().UTC().Format(time.RFC3339)

	if err := r.Status().Update(ctx, &report); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	interval := effectiveInterval(report.Spec.IntervalSeconds)
	log.Info("Weather updated successfully", "location", report.Status.Location, "requeueAfter", interval)
	return ctrl.Result{RequeueAfter: interval}, nil
}

// effectiveInterval returns the polling interval for the CR.
func effectiveInterval(intervalSeconds *int) time.Duration {
	if intervalSeconds != nil && *intervalSeconds >= 5 {
		return time.Duration(*intervalSeconds) * time.Second
	}
	return defaultIntervalSeconds * time.Second
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenWeatherReportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&weatherv1alpha1.OpenWeatherReport{}).
		Named("openweatherreport").
		Complete(r)
}
