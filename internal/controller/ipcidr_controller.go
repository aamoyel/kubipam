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

package controller

import (
	"context"
	"fmt"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerror "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ipamv1alpha1 "github.com/aamoyel/kubipam/api/v1alpha1"
	"github.com/go-logr/logr"
	goipam "github.com/metal-stack/go-ipam"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// IPCidrReconciler reconciles a IPCidr object
type IPCidrReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Log         logr.Logger
	Ipamer      goipam.Ipamer
	Initialized bool
}

//+kubebuilder:rbac:groups=ipam.amoyel.fr,resources=ipcidrs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ipam.amoyel.fr,resources=ipcidrs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ipam.amoyel.fr,resources=ipcidrs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *IPCidrReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log = log.FromContext(ctx)
	r.Log.Info("reconcile", "req", req)

	// Populate the ipam at the first reconciliation
	if !r.Initialized {
		err := r.initRegisteredCidrs(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.Initialized = true
	}

	// Fetch the IPCidr resource
	ipcidrCR := &ipamv1alpha1.IPCidr{}
	if err := r.Get(ctx, req.NamespacedName, ipcidrCR); err != nil { // check if resource is available and stored in err
		if apierrors.IsNotFound(err) { // check for "not found error"
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Add finalizers
	if ipcidrCR.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(ipcidrCR, ipcidrFinalizer) {
		r.Log.Info("Adding finalizer to the object", "IPCidr", ipcidrCR.Name)
		controllerutil.AddFinalizer(ipcidrCR, ipcidrFinalizer)
		if err := r.Update(ctx, ipcidrCR); err != nil {
			r.Log.Error(err, "Error adding finalizer")
			return ctrl.Result{}, err
		}
	}

	// Change the status before start the cidr reconciliation
	readyCondition := apimeta.FindStatusCondition(ipcidrCR.Status.Conditions, ipamv1alpha1.ConditionTypeReady)
	if readyCondition == nil || readyCondition.ObservedGeneration != ipcidrCR.GetGeneration() {
		IPCidrProgressing(ipcidrCR)
		if err := r.patchIPCidrStatus(ctx, ipcidrCR); err != nil {
			err = fmt.Errorf("unable to patch status after progressing: %w", err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	// Check if object was delete
	if !ipcidrCR.ObjectMeta.DeletionTimestamp.IsZero() {
		// check if the cidr is registered in the ipam
		if ipcidrCR.Status.Registered {
			cidr, err := r.Ipamer.PrefixFrom(ctx, ipcidrCR.Spec.Cidr)
			if err != nil {
				r.Log.Error(err, "Error getting prefix ", ipcidrCR.Spec.Cidr, ipcidrCR.Name)
				return ctrl.Result{RequeueAfter: requeueTime}, err
			}

			// get child ips
			ips := cidr.Usage().AcquiredIPs
			// get child cidrs
			cidrs := cidr.Usage().AcquiredPrefixes
			// check if the cidr have child prefixes before removing it (the ipamer doesn't send error when parent cidr has child prefixes).
			if cidrs > 0 {
				err = fmt.Errorf("cidr '%s' has %v chid prefixes, delete is not possible", ipcidrCR.Name, cidrs)
				setIPCidrDeletionStatus(ipcidrCR, err)
				if patchStatusErr := r.patchIPCidrStatus(ctx, ipcidrCR); patchStatusErr != nil {
					err = kerror.NewAggregate([]error{err, patchStatusErr})
					err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
					return ctrl.Result{RequeueAfter: requeueTime}, err
				}
				return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}

			// remove prefix from the ipam
			_, err = r.Ipamer.DeletePrefix(ctx, ipcidrCR.Spec.Cidr)
			if err != nil {
				// write the error in the statuses if the cidr has ips
				if ips > 0 {
					setIPCidrDeletionStatus(ipcidrCR, err)
					if patchStatusErr := r.patchIPCidrStatus(ctx, ipcidrCR); patchStatusErr != nil {
						err = kerror.NewAggregate([]error{err, patchStatusErr})
						err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
						return ctrl.Result{RequeueAfter: requeueTime}, err
					}
					return ctrl.Result{RequeueAfter: 10 * time.Second}, err
				} else {
					r.Log.Error(err, "Error deleting prefix in the ipam", ipcidrCR.Spec.Cidr, ipcidrCR.Name)
					return ctrl.Result{RequeueAfter: requeueTime}, err
				}
			}
		}

		// The object is being deleted
		if controllerutil.ContainsFinalizer(ipcidrCR, ipcidrFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(ipcidrCR, ipcidrFinalizer)
			if err := r.Update(ctx, ipcidrCR); err != nil {
				r.Log.Error(err, "Failed to remove finalizer, retrying...")
			}
		}
		r.Log.Info("Object has been deleted", "IPCidr", ipcidrCR.Name)
		return ctrl.Result{}, nil
	}

	// Start to create the cidr in the ipam and update statuses
	err := r.reconcileIPCidr(ctx, ipcidrCR)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	r.Log.Info("Successfully reconciled, status was updated", "IPCidr", ipcidrCR.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IPCidrReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.IPCidr{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&ipamv1alpha1.IPCidr{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				mapFunc, err := ipCidrReconcileRequests(ctx, mgr)
				if err != nil {
					r.Log.Error(err, "Error syncing reconcile request with ipcidr")
				}
				return mapFunc
			}),
		).
		Watches(
			&ipamv1alpha1.IPClaim{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				mapFunc, err := ipClaimReconcileRequests(ctx, mgr)
				if err != nil {
					r.Log.Error(err, "Error syncing reconcile request with ipclaim")
				}
				return mapFunc
			}),
		).
		Complete(r)
}

// IPCidrProgressing registers progress toward reconciling the given IPCidr
// by resetting the status to a progressing state.
func IPCidrProgressing(o *ipamv1alpha1.IPCidr) {
	o.Status.Conditions = []metav1.Condition{}
	apimeta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{
		Status:             metav1.ConditionFalse,
		Reason:             ipamv1alpha1.ProgressingReason,
		Message:            "Reconciliation progressing",
		Type:               ipamv1alpha1.ConditionTypeReady,
		ObservedGeneration: o.GetGeneration(),
	})
}

func (r *IPCidrReconciler) patchIPCidrStatus(ctx context.Context, ipCidr *ipamv1alpha1.IPCidr) error {
	key := client.ObjectKeyFromObject(ipCidr)
	latest := &ipamv1alpha1.IPCidr{}
	if err := r.Client.Get(ctx, key, latest); err != nil {
		return err
	}
	return r.Client.Status().Patch(ctx, ipCidr, client.MergeFrom(latest))
}

func (r *IPCidrReconciler) reconcileIPCidr(ctx context.Context, cr *ipamv1alpha1.IPCidr) error {
	// Check if the cidr is already registered
	_, err := r.Ipamer.PrefixFrom(ctx, cr.Spec.Cidr)
	if err != nil {
		// Check overlaping
		err := r.isCidrOverlapping(ctx, cr)
		if err != nil {
			return err
		}

		// Register the new prefix in the ipam
		_, err = r.Ipamer.NewPrefix(ctx, cr.Spec.Cidr)
		if err != nil {
			r.Log.Error(err, "Error when requesting cidr", cr.Name, cr.Spec.Cidr)
			setIPCidrErrorStatus(cr, err)
			if patchStatusErr := r.patchIPCidrStatus(ctx, cr); patchStatusErr != nil {
				err = kerror.NewAggregate([]error{err, patchStatusErr})
				err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
			}
			return err
		}
		cr.Status.Registered = true
	}

	if !cr.Status.Registered {
		if err := r.isCidrOverlapping(ctx, cr); err != nil {
			return err
		}
		return nil
	} else if cr.Status.Registered {
		cidr, err := r.Ipamer.PrefixFrom(ctx, cr.Spec.Cidr)
		if err != nil {
			return err
		}

		cr.Status.AvailableIPs = cidr.Usage().AvailableIPs - cidr.Usage().AcquiredIPs
		prefixes := cidr.Usage().AvailablePrefixes
		if len(prefixes) > 0 {
			cr.Status.AvailablePrefixes = prefixes
		}
	}

	// if reconcile succeed
	apimeta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Status:             metav1.ConditionTrue,
		Reason:             ipamv1alpha1.ReconciliationSucceededReason,
		Message:            "Reconciliation suceeded",
		Type:               ipamv1alpha1.ConditionTypeReady,
		ObservedGeneration: cr.GetGeneration(),
	})
	if err := r.patchIPCidrStatus(ctx, cr); err != nil {
		err = fmt.Errorf("unable to patch status after reconciliation succeeded: %w", err)
		return err
	}

	return nil
}

func setIPCidrErrorStatus(cr *ipamv1alpha1.IPCidr, err error) {
	apimeta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Status:             metav1.ConditionFalse,
		Reason:             ipamv1alpha1.ReconciliationFailedReason,
		Message:            err.Error(),
		Type:               ipamv1alpha1.ConditionTypeReady,
		ObservedGeneration: cr.GetGeneration(),
	})
}

func setIPCidrDeletionStatus(cr *ipamv1alpha1.IPCidr, err error) {
	apimeta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Status:             metav1.ConditionTrue,
		Reason:             ipamv1alpha1.ReconciliationSucceededReason,
		Message:            err.Error(),
		Type:               ipamv1alpha1.ConditionTypeReady,
		ObservedGeneration: cr.GetGeneration(),
	})
}

func (r *IPCidrReconciler) isCidrOverlapping(ctx context.Context, cr *ipamv1alpha1.IPCidr) error {
	existingPrefixes, err := r.Ipamer.ReadAllPrefixCidrs(ctx)
	if err != nil {
		err = fmt.Errorf("unable to read all prefix in the ipam: %w", err)
		setIPCidrErrorStatus(cr, err)
		if patchStatusErr := r.patchIPCidrStatus(ctx, cr); patchStatusErr != nil {
			err = kerror.NewAggregate([]error{err, patchStatusErr})
			err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
			return err
		}
	}
	if err := goipam.PrefixesOverlapping(existingPrefixes, []string{cr.Spec.Cidr}); err != nil {
		if !cr.Status.Registered {
			err = fmt.Errorf("cidr is already registered, overlap: %w", err)
		}
		setIPCidrErrorStatus(cr, err)
		if patchStatusErr := r.patchIPCidrStatus(ctx, cr); patchStatusErr != nil {
			err = kerror.NewAggregate([]error{err, patchStatusErr})
			err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
			return err
		}
	}
	return nil
}

// ipCidrReconcileRequests returns a list of reconcile.Request based on the ipcidr resource.
func ipCidrReconcileRequests(ctx context.Context, mgr manager.Manager) ([]reconcile.Request, error) {
	IPCidrList := &ipamv1alpha1.IPCidrList{}
	err := mgr.GetClient().List(ctx, IPCidrList)
	if err != nil {
		return []reconcile.Request{}, err
	}
	var requests []reconcile.Request
	for _, IPCidr := range IPCidrList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      IPCidr.GetName(),
				Namespace: IPCidr.GetNamespace(),
			},
		})
	}
	return requests, nil
}

// initRegistered allows registration of cidrs that are previously reconcilied by another instance of this controller.
func (r *IPCidrReconciler) initRegisteredCidrs(ctx context.Context) error {
	IPCidrList := &ipamv1alpha1.IPCidrList{}
	if err := r.List(ctx, IPCidrList); err != nil {
		return err
	}

	if len(IPCidrList.Items) > 0 {
		for _, IPCidr := range IPCidrList.Items {
			if IPCidr.Status.Registered {
				_, err := r.Ipamer.NewPrefix(ctx, IPCidr.Spec.Cidr)
				if err != nil {
					r.Log.Error(err, "Error when requesting cidr", IPCidr.Name, IPCidr.Spec.Cidr)
					return err
				}
			}
		}
		r.Log.Info("All CIDRs was successfully initialized in the IPAM")
		return nil
	} else {
		return nil
	}
}
