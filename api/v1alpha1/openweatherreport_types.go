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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenWeatherReportSpec defines the desired state of OpenWeatherReport.
type OpenWeatherReportSpec struct {
	// city is the name of the city to get weather for.
	// +kubebuilder:validation:MinLength=1
	City string `json:"city"`

	// country is the ISO 3166-1 alpha-2 country code (e.g. "NL").
	// +kubebuilder:validation:MinLength=2
	// +kubebuilder:validation:MaxLength=2
	Country string `json:"country"`

	// intervalSeconds is the polling interval in seconds. Minimum 5, defaults to 60.
	// +kubebuilder:validation:Minimum=5
	// +optional
	IntervalSeconds *int `json:"intervalSeconds,omitempty"`
}

// OpenWeatherReportStatus defines the observed state of OpenWeatherReport.
type OpenWeatherReportStatus struct {
	// temperature in Celsius.
	Temperature string `json:"temperature,omitempty"`

	// feelsLike temperature in Celsius.
	FeelsLike string `json:"feelsLike,omitempty"`

	// humidity percentage.
	Humidity string `json:"humidity,omitempty"`

	// pressure in hPa.
	Pressure string `json:"pressure,omitempty"`

	// location is "<city>, <country>" set by the controller for printer column display.
	Location string `json:"location,omitempty"`

	// status is "Valid" or "Error".
	// +kubebuilder:validation:Enum=Valid;Error
	Status string `json:"status,omitempty"`

	// errorMessage is populated when status is "Error".
	ErrorMessage string `json:"errorMessage,omitempty"`

	// lastUpdated is the RFC3339 timestamp of the last successful API call.
	LastUpdated string `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Location",type=string,JSONPath=`.status.location`
// +kubebuilder:printcolumn:name="Temperature",type=string,JSONPath=`.status.temperature`
// +kubebuilder:printcolumn:name="Humidity",type=string,JSONPath=`.status.humidity`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.status.lastUpdated`

// OpenWeatherReport is the Schema for the openweatherreports API.
type OpenWeatherReport struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of OpenWeatherReport
	// +required
	Spec OpenWeatherReportSpec `json:"spec"`

	// status defines the observed state of OpenWeatherReport
	// +optional
	Status OpenWeatherReportStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// OpenWeatherReportList contains a list of OpenWeatherReport
type OpenWeatherReportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []OpenWeatherReport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenWeatherReport{}, &OpenWeatherReportList{})
}
