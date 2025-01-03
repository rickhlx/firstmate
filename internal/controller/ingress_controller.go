/*
Copyright 2025.

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

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Ingress object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile
// Reconcile handles the business logic
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the skipper-ingress service
	service := &corev1.Service{}
	skipperServiceName := "skipper-ingress"
	skipperNamespace := "kube-system" // Adjust if skipper-ingress is in a different namespace

	if err := r.Get(ctx, client.ObjectKey{Name: skipperServiceName, Namespace: skipperNamespace}, service); err != nil {
		logger.Error(err, "Failed to fetch skipper-ingress service")
		return ctrl.Result{}, err
	}

	// Check if the Service has a LoadBalancer IP
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer || len(service.Status.LoadBalancer.Ingress) == 0 {
		logger.Info("skipper-ingress service is not a LoadBalancer or does not have an IP yet")
		return ctrl.Result{}, nil
	}

	lbIP := service.Status.LoadBalancer.Ingress[0].IP
	if lbIP == "" {
		logger.Info("skipper-ingress LoadBalancer does not have a valid IP")
		return ctrl.Result{}, nil
	}

	// List all Ingresses in the cluster
	ingressList := &networkingv1.IngressList{}
	if err := r.List(ctx, ingressList); err != nil {
		logger.Error(err, "Failed to list Ingress resources")
		return ctrl.Result{}, err
	}

	// Iterate over each Ingress and update it as needed
	for _, ingress := range ingressList.Items {
		// Skip if the Ingress is marked for deletion
		if !ingress.DeletionTimestamp.IsZero() {
			continue
		}

		// Update Ingress status with the LoadBalancer IP if needed
		needsUpdate := false
		if len(ingress.Status.LoadBalancer.Ingress) == 0 || ingress.Status.LoadBalancer.Ingress[0].IP != lbIP {
			ingress.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{IP: lbIP}}
			needsUpdate = true
		}

		if needsUpdate {
			if err := r.Status().Update(ctx, &ingress); err != nil {
				logger.Error(err, "Failed to update Ingress status", "Ingress", ingress.Name)
				return ctrl.Result{}, err
			}
			logger.Info("Updated Ingress with LoadBalancer IP", "Ingress", ingress.Name, "IP", lbIP)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Named("ingress").
		Complete(r)
}
