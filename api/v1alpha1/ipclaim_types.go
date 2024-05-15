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

// The reference to the IPCidr custom resource
type IPCidrRefSpec struct {
	// The name of the IPCidr custom resource
	//+kubebuilder:validation:Required
	Name string `json:"name"`
}

// IPClaimSpec defines the desired state of IPClaim
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="IPCidrSpec is immutable"
type IPClaimSpec struct {
	// The type of the claim. The value must be IP or CIDR
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:Pattern=`^(IP|CIDR)$`
	Type string `json:"type" yaml:"type"`

	// The prefix length of the cidr you want to claim.
	CidrPrefixLength uint8 `json:"cidrPrefixLength,omitempty"`

	// The reference to the IPCidr custom resource. It must be set if you want to acquire a specific IP or cidr.
	IPCidrRef *IPCidrRefSpec `json:"ipCidrRef,omitempty"`
	// A specific ip address you want to get in the ipam. You must set IPCidrRef to claim a specific IP.
	//+kubebuilder:validation:Pattern=`(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])`
	SpecificIPAddress string `json:"specificIPAddress,omitempty"`
	//+kubebuilder:validation:Pattern=`(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])/(([0-9])|([1-2][0-9])|(3[0-2]))|((:|[0-9a-fA-F]{0,4}):)([0-9a-fA-F]{0,4}:){0,5}((([0-9a-fA-F]{0,4}:)?(:|[0-9a-fA-F]{0,4}))|(((25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9]?[0-9])))(/(([0-9])|([0-9]{2})|(1[0-1][0-9])|(12[0-8])))`
	// A specific cidr you want to get in the ipam. You must set IPCidrRef to claim a specific cidr.
	SpecificChildCidr string `json:"specificChildCidr,omitempty"`
}

// IPClaimStatus defines the observed state of IPClaim
type IPClaimStatus struct {
	//+kubebuilder:default:=false
	Registered bool               `json:"registered"`
	ParentCidr string             `json:"parentCidr,omitempty"`
	Claim      string             `json:"claim,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Claim",type="string",JSONPath=".status.claim"
//+kubebuilder:printcolumn:name="ParentCidr",type="string",JSONPath=".status.parentCidr"
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"

// IPClaim is the Schema for the ipclaims API
type IPClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPClaimSpec   `json:"spec,omitempty"`
	Status IPClaimStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IPClaimList contains a list of IPClaim
type IPClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IPClaim `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IPClaim{}, &IPClaimList{})
}
