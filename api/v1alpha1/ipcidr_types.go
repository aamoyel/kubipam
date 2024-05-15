/*
Copyright 2024 AlanAmoyel.

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

// IPCidrSpec defines the desired state of IPCidr
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="IPCidrSpec is immutable"
type IPCidrSpec struct {
	// Cidr defines the cidr you want to register in the ipam, it can also be an address if a /32 or /128 is specified
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:Pattern=`(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])/(([0-9])|([1-2][0-9])|(3[0-2]))|((:|[0-9a-fA-F]{0,4}):)([0-9a-fA-F]{0,4}:){0,5}((([0-9a-fA-F]{0,4}:)?(:|[0-9a-fA-F]{0,4}))|(((25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])))(/(([0-9])|([0-9]{2})|(1[0-1][0-9])|(12[0-8])))`
	Cidr string `json:"cidr" yaml:"cidr"`
}

// IPCidrStatus defines the observed state of IPCidr
type IPCidrStatus struct {
	//+kubebuilder:default:=false
	Registered bool `json:"registered"`
	//+kubebuilder:default:=0
	AvailableIPs      uint64             `json:"availableIPs"`
	AvailablePrefixes []string           `json:"availablePrefixes,omitempty"`
	Conditions        []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Cidr",type="string",JSONPath=".spec.cidr"
//+kubebuilder:printcolumn:name="AvailableIPs",type="integer",JSONPath=".status.availableIPs"
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"

// IPCidr is the Schema for the ipcidrs API
type IPCidr struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPCidrSpec   `json:"spec,omitempty"`
	Status IPCidrStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IPCidrList contains a list of IPCidr
type IPCidrList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPCidr `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPCidr{}, &IPCidrList{})
}
