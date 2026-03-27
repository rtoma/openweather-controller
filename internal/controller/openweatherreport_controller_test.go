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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	weatherv1alpha1 "github.com/rtoma/openweather-controller/api/v1alpha1"
	"github.com/rtoma/openweather-controller/internal/weather"
)

var _ = Describe("OpenWeatherReport Controller", func() {

	const timeout = 10 * time.Second
	const interval = 250 * time.Millisecond

	// Helper to delete a CR if it exists.
	cleanupCR := func(name string) {
		cr := &weatherv1alpha1.OpenWeatherReport{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, cr)
		if err == nil {
			Expect(k8sClient.Delete(ctx, cr)).To(Succeed())
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: name}, cr))
			}, timeout, interval).Should(BeTrue())
		}
	}

	Context("When a new OpenWeatherReport is created", func() {
		const crName = "integration-amsterdam"

		BeforeEach(func() {
			testWeatherFetcher.SetResult(&weather.WeatherData{
				Temperature: 18.5,
				FeelsLike:   17.0,
				Humidity:    65,
				Pressure:    1013,
			}, nil)
			testWeatherFetcher.ResetCalls()
		})

		AfterEach(func() {
			cleanupCR(crName)
		})

		It("should reconcile and populate status with weather data", func() {
			cr := &weatherv1alpha1.OpenWeatherReport{
				ObjectMeta: metav1.ObjectMeta{Name: crName},
				Spec: weatherv1alpha1.OpenWeatherReportSpec{
					City:    "Amsterdam",
					Country: "NL",
				},
			}
			Expect(k8sClient.Create(ctx, cr)).To(Succeed())

			Eventually(func(g Gomega) {
				var report weatherv1alpha1.OpenWeatherReport
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crName}, &report)).To(Succeed())
				g.Expect(report.Status.Status).To(Equal("Valid"))
				g.Expect(report.Status.Temperature).To(Equal("18.5°C"))
				g.Expect(report.Status.FeelsLike).To(Equal("17.0°C"))
				g.Expect(report.Status.Humidity).To(Equal("65%"))
				g.Expect(report.Status.Pressure).To(Equal("1013 hPa"))
				g.Expect(report.Status.Location).To(Equal("Amsterdam, NL"))
				g.Expect(report.Status.ErrorMessage).To(BeEmpty())
				g.Expect(report.Status.LastUpdated).NotTo(BeEmpty())
				_, err := time.Parse(time.RFC3339, report.Status.LastUpdated)
				g.Expect(err).NotTo(HaveOccurred())
			}, timeout, interval).Should(Succeed())
		})

		It("should pass the correct city and country to the weather API", func() {
			cr := &weatherv1alpha1.OpenWeatherReport{
				ObjectMeta: metav1.ObjectMeta{Name: crName},
				Spec: weatherv1alpha1.OpenWeatherReportSpec{
					City:    "Amsterdam",
					Country: "NL",
				},
			}
			Expect(k8sClient.Create(ctx, cr)).To(Succeed())

			Eventually(func() int {
				return testWeatherFetcher.CallCount()
			}, timeout, interval).Should(BeNumerically(">=", 1))

			city, country := testWeatherFetcher.LastRequest()
			Expect(city).To(Equal("Amsterdam"))
			Expect(country).To(Equal("NL"))
		})
	})

	Context("When the weather API returns an error", func() {
		const crName = "integration-error"

		BeforeEach(func() {
			testWeatherFetcher.SetResult(nil, fmt.Errorf("API error: HTTP 503"))
			testWeatherFetcher.ResetCalls()
		})

		AfterEach(func() {
			// Reset to success before cleanup to avoid errors during CR deletion requeue.
			testWeatherFetcher.SetResult(&weather.WeatherData{}, nil)
			cleanupCR(crName)
		})

		It("should set status to Error with errorMessage", func() {
			cr := &weatherv1alpha1.OpenWeatherReport{
				ObjectMeta: metav1.ObjectMeta{Name: crName},
				Spec: weatherv1alpha1.OpenWeatherReportSpec{
					City:    "Berlin",
					Country: "DE",
				},
			}
			Expect(k8sClient.Create(ctx, cr)).To(Succeed())

			Eventually(func(g Gomega) {
				var report weatherv1alpha1.OpenWeatherReport
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crName}, &report)).To(Succeed())
				g.Expect(report.Status.Status).To(Equal(statusError))
				g.Expect(report.Status.ErrorMessage).To(ContainSubstring("API error: HTTP 503"))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When the weather API recovers after an error", func() {
		const crName = "integration-recovery"

		AfterEach(func() {
			testWeatherFetcher.SetResult(&weather.WeatherData{}, nil)
			cleanupCR(crName)
		})

		It("should transition from Error to Valid and clear errorMessage", func() {
			// Start with API returning an error.
			testWeatherFetcher.SetResult(nil, fmt.Errorf("temporary failure"))
			testWeatherFetcher.ResetCalls()

			cr := &weatherv1alpha1.OpenWeatherReport{
				ObjectMeta: metav1.ObjectMeta{Name: crName},
				Spec: weatherv1alpha1.OpenWeatherReportSpec{
					City:    "Paris",
					Country: "FR",
				},
			}
			Expect(k8sClient.Create(ctx, cr)).To(Succeed())

			// Wait for Error status.
			Eventually(func(g Gomega) {
				var report weatherv1alpha1.OpenWeatherReport
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crName}, &report)).To(Succeed())
				g.Expect(report.Status.Status).To(Equal(statusError))
			}, timeout, interval).Should(Succeed())

			// Now make the API succeed.
			testWeatherFetcher.SetResult(&weather.WeatherData{
				Temperature: 22.0,
				FeelsLike:   21.0,
				Humidity:    55,
				Pressure:    1015,
			}, nil)

			// Wait for recovery to Valid.
			Eventually(func(g Gomega) {
				var report weatherv1alpha1.OpenWeatherReport
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crName}, &report)).To(Succeed())
				g.Expect(report.Status.Status).To(Equal("Valid"))
				g.Expect(report.Status.ErrorMessage).To(BeEmpty())
				g.Expect(report.Status.Temperature).To(Equal("22.0°C"))
				g.Expect(report.Status.Location).To(Equal("Paris, FR"))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When a CR uses a custom interval", func() {
		const crName = "integration-custom-interval"

		BeforeEach(func() {
			testWeatherFetcher.SetResult(&weather.WeatherData{
				Temperature: 10.0,
				FeelsLike:   8.0,
				Humidity:    90,
				Pressure:    1000,
			}, nil)
			testWeatherFetcher.ResetCalls()
		})

		AfterEach(func() {
			cleanupCR(crName)
		})

		It("should reconcile successfully with the custom interval", func() {
			intervalSec := 5
			cr := &weatherv1alpha1.OpenWeatherReport{
				ObjectMeta: metav1.ObjectMeta{Name: crName},
				Spec: weatherv1alpha1.OpenWeatherReportSpec{
					City:            "Oslo",
					Country:         "NO",
					IntervalSeconds: &intervalSec,
				},
			}
			Expect(k8sClient.Create(ctx, cr)).To(Succeed())

			Eventually(func(g Gomega) {
				var report weatherv1alpha1.OpenWeatherReport
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crName}, &report)).To(Succeed())
				g.Expect(report.Status.Status).To(Equal("Valid"))
				g.Expect(report.Status.Temperature).To(Equal("10.0°C"))
			}, timeout, interval).Should(Succeed())

			// With a 5s interval the controller should re-reconcile quickly.
			// Record the call count after first success.
			initialCalls := testWeatherFetcher.CallCount()

			// Wait long enough for at least one re-reconcile at the 5s interval.
			Eventually(func() int {
				return testWeatherFetcher.CallCount()
			}, 12*time.Second, interval).Should(BeNumerically(">", initialCalls))
		})
	})

	Context("When the CR is deleted", func() {
		const crName = "integration-delete"

		BeforeEach(func() {
			testWeatherFetcher.SetResult(&weather.WeatherData{
				Temperature: 30.0,
				FeelsLike:   32.0,
				Humidity:    40,
				Pressure:    1020,
			}, nil)
			testWeatherFetcher.ResetCalls()
		})

		It("should handle deletion gracefully", func() {
			cr := &weatherv1alpha1.OpenWeatherReport{
				ObjectMeta: metav1.ObjectMeta{Name: crName},
				Spec: weatherv1alpha1.OpenWeatherReportSpec{
					City:    "Madrid",
					Country: "ES",
				},
			}
			Expect(k8sClient.Create(ctx, cr)).To(Succeed())

			// Wait for Valid status first.
			Eventually(func(g Gomega) {
				var report weatherv1alpha1.OpenWeatherReport
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crName}, &report)).To(Succeed())
				g.Expect(report.Status.Status).To(Equal("Valid"))
			}, timeout, interval).Should(Succeed())

			// Delete the CR.
			var toDelete weatherv1alpha1.OpenWeatherReport
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crName}, &toDelete)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &toDelete)).To(Succeed())

			// Confirm it's gone.
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: crName}, &weatherv1alpha1.OpenWeatherReport{}))
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When multiple CRs exist", func() {
		const cr1Name = "integration-multi-1"
		const cr2Name = "integration-multi-2"

		BeforeEach(func() {
			testWeatherFetcher.SetResult(&weather.WeatherData{
				Temperature: 15.0,
				FeelsLike:   14.0,
				Humidity:    70,
				Pressure:    1010,
			}, nil)
			testWeatherFetcher.ResetCalls()
		})

		AfterEach(func() {
			cleanupCR(cr1Name)
			cleanupCR(cr2Name)
		})

		It("should reconcile both CRs independently", func() {
			cr1 := &weatherv1alpha1.OpenWeatherReport{
				ObjectMeta: metav1.ObjectMeta{Name: cr1Name},
				Spec: weatherv1alpha1.OpenWeatherReportSpec{
					City:    "Tokyo",
					Country: "JP",
				},
			}
			cr2 := &weatherv1alpha1.OpenWeatherReport{
				ObjectMeta: metav1.ObjectMeta{Name: cr2Name},
				Spec: weatherv1alpha1.OpenWeatherReportSpec{
					City:    "Sydney",
					Country: "AU",
				},
			}
			Expect(k8sClient.Create(ctx, cr1)).To(Succeed())
			Expect(k8sClient.Create(ctx, cr2)).To(Succeed())

			// Both should reach Valid status.
			Eventually(func(g Gomega) {
				var report1 weatherv1alpha1.OpenWeatherReport
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: cr1Name}, &report1)).To(Succeed())
				g.Expect(report1.Status.Status).To(Equal("Valid"))
				g.Expect(report1.Status.Location).To(Equal("Tokyo, JP"))

				var report2 weatherv1alpha1.OpenWeatherReport
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: cr2Name}, &report2)).To(Succeed())
				g.Expect(report2.Status.Status).To(Equal("Valid"))
				g.Expect(report2.Status.Location).To(Equal("Sydney, AU"))
			}, timeout, interval).Should(Succeed())

			// The fetcher should have been called at least twice (once per CR).
			Expect(testWeatherFetcher.CallCount()).To(BeNumerically(">=", 2))
		})
	})
})
