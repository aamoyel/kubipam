/*
Copyright 2024.

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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	goipam "github.com/metal-stack/go-ipam"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerror "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ipamv1alpha1 "github.com/aamoyel/kubipam/api/v1alpha1"
)

// IPClaimReconciler reconciles a IPClaim object
type IPClaimReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	Log         logr.Logger
	Ipamer      goipam.Ipamer
	Initialized bool
}

//+kubebuilder:rbac:groups=ipam.didactiklabs.io,resources=ipclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ipam.didactiklabs.io,resources=ipclaims/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ipam.didactiklabs.io,resources=ipclaims/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *IPClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log = log.FromContext(ctx)
	r.Log.Info("New reconciliation")

	// Populate the ipam at the first reconciliation
	if !r.Initialized {
		err := r.initRegisteredClaims(ctx)
		if err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		r.Initialized = true
	}

	// Fetch the IPClaim resource
	ipclaimCR := &ipamv1alpha1.IPClaim{}
	if err := r.Get(ctx, req.NamespacedName, ipclaimCR); err != nil { // check if resource is available and stored in err
		if apierrors.IsNotFound(err) { // check for "not found error"
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Add finalizers
	if ipclaimCR.ObjectMeta.DeletionTimestamp.IsZero() &&
		!controllerutil.ContainsFinalizer(ipclaimCR, ipclaimFinalizer) {
		r.Log.Info("Adding finalizer to the object", "IPClaim", ipclaimCR.Name)
		controllerutil.AddFinalizer(ipclaimCR, ipclaimFinalizer)
		if err := r.Update(ctx, ipclaimCR); err != nil {
			r.Log.Error(err, "Error adding finalizer")
			return ctrl.Result{}, err
		}
	}

	// Change the status before start the claim reconciliation
	readyCondition := apimeta.FindStatusCondition(
		ipclaimCR.Status.Conditions,
		ipamv1alpha1.ConditionTypeReady,
	)
	if readyCondition == nil || readyCondition.ObservedGeneration != ipclaimCR.GetGeneration() {
		IPClaimProgressing(ipclaimCR)
		if err := r.patchIPClaimStatus(ctx, ipclaimCR); err != nil {
			err = fmt.Errorf("unable to patch status after progressing: %w", err)
			return ctrl.Result{Requeue: true}, err
		}
	}

	// Check if object was delete
	if !ipclaimCR.ObjectMeta.DeletionTimestamp.IsZero() {
		if ipclaimCR.Spec.Type == "IP" {
			if ipclaimCR.Status.Registered {
				// release ip from the ipam
				err := r.Ipamer.ReleaseIPFromPrefix(
					ctx,
					ipclaimCR.Status.ParentCidr,
					ipclaimCR.Status.Claim,
				)
				if err != nil {
					r.Log.Error(
						err,
						"Error releasing the ip from the ipam",
						ipclaimCR.Status.Claim,
						ipclaimCR.Name,
					)
					return ctrl.Result{RequeueAfter: requeueTime}, err
				}
			}
		} else {
			if ipclaimCR.Status.Registered {
				// release prefix from the parent cidr
				childCidr, err := r.Ipamer.PrefixFrom(ctx, ipclaimCR.Status.Claim)
				if err != nil {
					r.Log.Error(err, "Error getting child cidr from the ipam", ipclaimCR.Status.Claim, ipclaimCR.Name)
					return ctrl.Result{RequeueAfter: requeueTime}, err
				}
				err = r.Ipamer.ReleaseChildPrefix(ctx, childCidr)
				if err != nil {
					r.Log.Error(err, "Error releasing child cidr from the ipam", ipclaimCR.Status.Claim, ipclaimCR.Name)
					return ctrl.Result{RequeueAfter: requeueTime}, err
				}
			}
		}
		// The object is being deleted
		if controllerutil.ContainsFinalizer(ipclaimCR, ipclaimFinalizer) {
			// our finalizer is present, so lets handle any external dependency
			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(ipclaimCR, ipclaimFinalizer)
			if err := r.Update(ctx, ipclaimCR); err != nil {
				r.Log.Error(err, "Failed to remove finalizer, retrying...")
			}
		}
		r.Log.Info("Object has been deleted", "IPClaim", ipclaimCR.Name)
		return ctrl.Result{}, nil
	}

	// Start to create the claim in the ipam and update statuses
	err := r.reconcileIPClaim(ctx, ipclaimCR)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	r.Log.Info("Successfully reconciled, status was updated", "IPClaim", ipclaimCR.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IPClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ipamv1alpha1.IPClaim{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
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
		Watches(
			&ipamv1alpha1.IPCidr{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				mapFunc, err := ipCidrReconcileRequests(ctx, mgr)
				if err != nil {
					r.Log.Error(err, "Error syncing reconcile request with ipcidr")
				}
				return mapFunc
			}),
			builder.WithPredicates(predicate.Funcs{
				CreateFunc: func(e event.CreateEvent) bool {
					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return true
				},
			}, predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}

// IPClaimProgressing registers progress toward reconciling the given IPClaim
// by resetting the status to a progressing state.
func IPClaimProgressing(o *ipamv1alpha1.IPClaim) {
	o.Status.Conditions = []metav1.Condition{}
	apimeta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{
		Status:             metav1.ConditionFalse,
		Reason:             ipamv1alpha1.ProgressingReason,
		Message:            "Reconciliation progressing",
		Type:               ipamv1alpha1.ConditionTypeReady,
		ObservedGeneration: o.GetGeneration(),
	})
}

func (r *IPClaimReconciler) patchIPClaimStatus(
	ctx context.Context,
	ipClaim *ipamv1alpha1.IPClaim,
) error {
	key := client.ObjectKeyFromObject(ipClaim)
	latest := &ipamv1alpha1.IPClaim{}
	if err := r.Client.Get(ctx, key, latest); err != nil {
		return err
	}
	return r.Client.Status().Patch(ctx, ipClaim, client.MergeFrom(latest))
}

func (r *IPClaimReconciler) reconcileIPClaim(ctx context.Context, cr *ipamv1alpha1.IPClaim) error {
	if !cr.Status.Registered {
		// Check the type of the claim
		if cr.Spec.Type == "IP" {
			// Get parent cidr
			cidrCR, err := r.getParentCidr(ctx, cr)
			if err != nil {
				return err
			}
			if !cidrCR.Status.Registered {
				err := fmt.Errorf(
					"the cidr resource in the IPCidrRefSpec for the %s claim is not in a ready state",
					cr.Name,
				)
				setIPClaimErrorStatus(cr, err)
				if patchStatusErr := r.patchIPClaimStatus(ctx, cr); patchStatusErr != nil {
					err = kerror.NewAggregate([]error{err, patchStatusErr})
					err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
					return err
				}
				return nil
			}

			// Claim the specific ip or handle errors
			err = r.getIpAddr(ctx, cr, cidrCR.Spec.Cidr)
			if err != nil {
				return err
			}
		} else {
			err := r.getChildCidr(ctx, cr)
			if err != nil {
				return err
			}
		}
	}

	// if reconcile succeed
	apimeta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Status:             metav1.ConditionTrue,
		Reason:             ipamv1alpha1.ReconciliationSucceededReason,
		Message:            "Reconciliation succeeded",
		Type:               ipamv1alpha1.ConditionTypeReady,
		ObservedGeneration: cr.GetGeneration(),
	})
	if err := r.patchIPClaimStatus(ctx, cr); err != nil {
		err = fmt.Errorf("unable to patch status after reconciliation succeeded: %w", err)
		return err
	}

	return nil
}

func (r *IPClaimReconciler) getIpAddr(
	ctx context.Context,
	cr *ipamv1alpha1.IPClaim,
	parentCidr string,
) error {
	claim, err := r.Ipamer.AcquireSpecificIP(ctx, parentCidr, cr.Spec.SpecificIPAddress)
	if claim == nil {
		err = fmt.Errorf("%w", err)
		setIPClaimErrorStatus(cr, err)
		if patchStatusErr := r.patchIPClaimStatus(ctx, cr); patchStatusErr != nil {
			err = kerror.NewAggregate([]error{err, patchStatusErr})
			err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
		}
		return err
	} else if err != nil {
		setIPClaimErrorStatus(cr, err)
		if patchStatusErr := r.patchIPClaimStatus(ctx, cr); patchStatusErr != nil {
			err = kerror.NewAggregate([]error{err, patchStatusErr})
			err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
		}
		return err
	}
	cr.Status.Registered = true
	cr.Status.Claim = claim.IP.String()
	cr.Status.ParentCidr = parentCidr
	return nil
}

func (r *IPClaimReconciler) getChildCidr(
	ctx context.Context,
	cr *ipamv1alpha1.IPClaim,
) (err error) {
	var (
		claim  *goipam.Prefix
		cidrCr *ipamv1alpha1.IPCidr
	)

	if cr.Spec.SpecificChildCidr != "" {
		cidrCr, err = r.getParentCidr(ctx, cr)
		if err != nil {
			return err
		}

		// check if the parent cidr is registered in the ipam
		if err = r.checkParentCidr(ctx, cr, cidrCr.Spec.Cidr); err != nil {
			return err
		}

		claim, err = r.Ipamer.AcquireSpecificChildPrefix(
			ctx,
			cidrCr.Spec.Cidr,
			cr.Spec.SpecificChildCidr,
		)
		if claim == nil {
			err = fmt.Errorf("%w", err)
			setIPClaimErrorStatus(cr, err)
			if patchStatusErr := r.patchIPClaimStatus(ctx, cr); patchStatusErr != nil {
				err = kerror.NewAggregate([]error{err, patchStatusErr})
				err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
			}
			return err
		} else if err != nil {
			setIPClaimErrorStatus(cr, err)
			if patchStatusErr := r.patchIPClaimStatus(ctx, cr); patchStatusErr != nil {
				err = kerror.NewAggregate([]error{err, patchStatusErr})
				err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
			}
			return err
		}
	} else if cr.Spec.SpecificChildCidr == "" {
		// Check if we need to use a specific parent CIDR
		if cr.Spec.IPCidrRef.Name != "" {
			// Get parent cidr
			cidrCR, err := r.getParentCidr(ctx, cr)
			if err != nil {
				return err
			}
			if !cidrCR.Status.Registered {
				err := fmt.Errorf("the cidr resource in the IPCidrRefSpec for the %s claim is not in a ready state", cr.Name)
				setIPClaimErrorStatus(cr, err)
				if patchStatusErr := r.patchIPClaimStatus(ctx, cr); patchStatusErr != nil {
					err = kerror.NewAggregate([]error{err, patchStatusErr})
					err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
					return err
				}
				return nil
			}
		} else {
			// find available parent cidr for this prefix length
			cidrList := &ipamv1alpha1.IPCidrList{}
			if err := r.List(ctx, cidrList); err != nil {
				return err
			}

			if len(cidrList.Items) == 0 {
				err = fmt.Errorf("no available parent cidr")
				setIPClaimErrorStatus(cr, err)
				if patchStatusErr := r.patchIPClaimStatus(ctx, cr); patchStatusErr != nil {
					err = kerror.NewAggregate([]error{err, patchStatusErr})
					err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
					return err
				}
				return err
			} else {
				for _, cidr := range cidrList.Items {
					claim, err = r.Ipamer.AcquireChildPrefix(ctx, cidr.Spec.Cidr, cr.Spec.CidrPrefixLength)
					if err == nil {
						cidrCr = &cidr

						// check if the parent cidr is registered in the ipam
						if err = r.checkParentCidr(ctx, cr, cidrCr.Spec.Cidr); err != nil {
							return err
						}
						break
					} else {
						err = fmt.Errorf("no matching cidr for your prefix length")
						setIPClaimErrorStatus(cr, err)
						if patchStatusErr := r.patchIPClaimStatus(ctx, cr); patchStatusErr != nil {
							err = kerror.NewAggregate([]error{err, patchStatusErr})
							err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
							return err
						}
						return err
					}
				}
			}
		}
	}

	cr.Status.Registered = true
	cr.Status.Claim = claim.Cidr
	cr.Status.ParentCidr = cidrCr.Spec.Cidr
	return nil
}

func setIPClaimErrorStatus(cr *ipamv1alpha1.IPClaim, err error) {
	apimeta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
		Status:             metav1.ConditionFalse,
		Reason:             ipamv1alpha1.ReconciliationFailedReason,
		Message:            err.Error(),
		Type:               ipamv1alpha1.ConditionTypeReady,
		ObservedGeneration: cr.GetGeneration(),
	})
}

// ipClaimReconcileRequests returns a list of reconcile.Request based on the ipclaim resource.
func ipClaimReconcileRequests(
	ctx context.Context,
	mgr manager.Manager,
) ([]reconcile.Request, error) {
	IPClaimList := &ipamv1alpha1.IPClaimList{}
	err := mgr.GetClient().List(ctx, IPClaimList)
	if err != nil {
		return []reconcile.Request{}, err
	}
	var requests []reconcile.Request
	for _, IPClaim := range IPClaimList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      IPClaim.GetName(),
				Namespace: IPClaim.GetNamespace(),
			},
		})
	}
	return requests, nil
}

func (r *IPClaimReconciler) getParentCidr(
	ctx context.Context,
	cr *ipamv1alpha1.IPClaim,
) (*ipamv1alpha1.IPCidr, error) {
	cidrCR := &ipamv1alpha1.IPCidr{}
	cidrNamepacedName := types.NamespacedName{
		Name: cr.Spec.IPCidrRef.Name,
	}

	if err := r.Get(ctx, cidrNamepacedName, cidrCR); err != nil { // check if resource is available and stored in err
		if apierrors.IsNotFound(err) { // check for "not found error"
			err = fmt.Errorf(
				"unable to find or get cidr in IPCidrRefSpec for the %s claim: %w",
				cr.Name,
				err,
			)
			setIPClaimErrorStatus(cr, err)
			if patchStatusErr := r.patchIPClaimStatus(ctx, cr); patchStatusErr != nil {
				err = kerror.NewAggregate([]error{err, patchStatusErr})
				err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
				return nil, err
			}
		}
		return nil, err
	}
	return cidrCR, nil
}

// initRegistered allows registration of claims that are previously reconcilied by another instance of this controller.
func (r *IPClaimReconciler) initRegisteredClaims(ctx context.Context) error {
	IPClaimList := &ipamv1alpha1.IPClaimList{}
	if err := r.List(ctx, IPClaimList); err != nil {
		return err
	}

	if len(IPClaimList.Items) > 0 {
		for _, IPClaim := range IPClaimList.Items {
			if IPClaim.Status.Registered {
				// Check the type of the claim
				if IPClaim.Spec.Type == "IP" {
					_, err := r.Ipamer.AcquireSpecificIP(
						ctx,
						IPClaim.Status.ParentCidr,
						IPClaim.Status.Claim,
					)
					if err != nil {
						return err
					}
				} else {
					// check if the parent cidr is registered in the ipam
					if err := r.checkParentCidr(ctx, &IPClaim, IPClaim.Status.ParentCidr); err != nil {
						return err
					}
					_, err := r.Ipamer.AcquireSpecificChildPrefix(ctx, IPClaim.Status.ParentCidr, IPClaim.Status.Claim)
					if err != nil {
						return err
					}
				}
			}
		}
		r.Log.Info("All Claims was successfully initialized in the IPAM")
		return nil
	} else {
		return nil
	}
}

// checkParentCidr return an error when the parent cidr is not registered in the ipam.
func (r *IPClaimReconciler) checkParentCidr(
	ctx context.Context,
	claimCr *ipamv1alpha1.IPClaim,
	parentCidr string,
) error {
	_, err := r.Ipamer.PrefixFrom(ctx, parentCidr)
	if err != nil {
		err = fmt.Errorf("unable to find parent cidr in the ipam, err: %w", err)
		setIPClaimErrorStatus(claimCr, err)
		if patchStatusErr := r.patchIPClaimStatus(ctx, claimCr); patchStatusErr != nil {
			err = kerror.NewAggregate([]error{err, patchStatusErr})
			err = fmt.Errorf("unable to patch status after reconciliation failed: %w", err)
		}
		return err
	}
	return nil
}
