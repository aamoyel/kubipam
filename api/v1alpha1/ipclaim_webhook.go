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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var ipclaimlog = logf.Log.WithName("ipclaim-resource")

func (r *IPClaim) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-ipam-amoyel-fr-v1alpha1-ipclaim,mutating=true,failurePolicy=fail,sideEffects=None,groups=ipam.didactiklabs.io,resources=ipclaims,verbs=create;update,versions=v1alpha1,name=mipclaim.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &IPClaim{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *IPClaim) Default() {
	// No validation by default
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-ipam-amoyel-fr-v1alpha1-ipclaim,mutating=false,failurePolicy=fail,sideEffects=None,groups=ipam.didactiklabs.io,resources=ipclaims,verbs=create;update,versions=v1alpha1,name=vipclaim.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &IPClaim{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *IPClaim) ValidateCreate() (admission.Warnings, error) {
	err := r.validateClaim()
	if err != nil {
		return nil, err
	}

	ipclaimlog.Info("validate create", "name", r.Name)
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *IPClaim) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *IPClaim) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (r *IPClaim) validateClaim() error {
	var allErrs field.ErrorList
	if err := r.validateSpec(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "ipam.didactiklabs.io", Kind: "IPClaim"},
		r.Name, allErrs)
}

func (r *IPClaim) validateSpec() *field.Error {
	// The field helpers from the kubernetes API machinery help us return nicely
	// structured validation errors.
	if r.Spec.Type == "IP" {
		noRefmsg := field.Forbidden(field.NewPath("spec").Child("ipCidrRef").Child("name"), "the name of a valid IPCidr resource must be referenced when using 'type: IP'")
		if r.Spec.IPCidrRef == nil {
			return noRefmsg
		}
		if r.Spec.IPCidrRef.Name == "" {
			return noRefmsg
		}
	}

	if r.Spec.Type == "CIDR" {
		noRefmsg := field.Forbidden(field.NewPath("spec").Child("ipCidrRef").Child("name"), "the name of a valid IPCidr resource must be referenced when using 'specificChildCidr'")
		if r.Spec.SpecificChildCidr != "" {
			if r.Spec.IPCidrRef == nil {
				return noRefmsg
			}
			if r.Spec.IPCidrRef.Name == "" {
				return noRefmsg
			}
		}

		if r.Spec.SpecificChildCidr == "" && r.Spec.CidrPrefixLength == 0 {
			return field.Forbidden(field.NewPath("spec").Child("cidrPrefixLength"), "the prefix length must be set when using 'type: CIDR'")
		}
	}
	return nil
}
