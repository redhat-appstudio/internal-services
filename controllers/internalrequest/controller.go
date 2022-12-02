/*
Copyright 2022.

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

package internalrequest

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/redhat-appstudio/internal-services/api/v1alpha1"
	"github.com/redhat-appstudio/operator-goodies/reconciler"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Reconciler reconciles an InternalRequest object
type Reconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// NewInternalRequestReconciler creates and returns a Reconciler.
func NewInternalRequestReconciler(client client.Client, logger *logr.Logger, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		Client: client,
		Log:    logger.WithName("release"),
		Scheme: scheme,
	}
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=internalrequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=internalrequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=internalrequests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("InternalRequest", req.NamespacedName)

	release := &v1alpha1.InternalRequest{}
	err := r.Get(ctx, req.NamespacedName, release)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	adapter := NewAdapter(release, logger, r.Client, ctx)

	return reconciler.ReconcileHandler([]reconciler.ReconcileOperation{
		adapter.EnsureReconcileOperationIsLogged,
	})
}

// SetupController creates a new InternalRequest reconciler and adds it to the Manager.
func SetupController(mgr ctrl.Manager, remoteCluster cluster.Cluster, log *logr.Logger) error {
	return setupControllerWithManager(mgr, remoteCluster, NewInternalRequestReconciler(mgr.GetClient(), log, mgr.GetScheme()))
}

// setupControllerWithManager sets up the controller with the Manager which monitors new Releases and filters out
// status updates. This controller also watches for PipelineRuns created by this controller and owned by the Releases so
// the owner gets reconciled on PipelineRun changes.
func setupControllerWithManager(mgr ctrl.Manager, remoteCluster cluster.Cluster, reconciler *Reconciler) error {
	return ctrl.NewControllerManagedBy(mgr).
		Watches(
			source.NewKindWithCache(&v1alpha1.InternalRequest{}, remoteCluster.GetCache()),
			&handler.EnqueueRequestForObject{},
		).
		Complete(reconciler)
}
