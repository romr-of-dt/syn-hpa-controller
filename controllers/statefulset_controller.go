/*
Copyright 2023.

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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/romr-of-dt/syn-hpa-controller/controllers/autoscaler"
	"github.com/romr-of-dt/syn-hpa-controller/controllers/kubejects"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// StatefulSetReconciler reconciles a StatefulSet object
type StatefulSetReconciler struct {
	statefulSets *kubejects.ApiRequests[
		appsv1.StatefulSet,
		*appsv1.StatefulSet,
		appsv1.StatefulSetList,
		*appsv1.StatefulSetList,
	]
}

var (
	logger logr.Logger

	statefulSetLabelSelector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/component":  "synthetic",
			"app.kubernetes.io/created-by": "dynakube",
			"app.kubernetes.io/managed-by": "dynatrace-operator",
			"app.kubernetes.io/name":       "activegate",
		},
	}
)

//nolint:revive
func NewStatefulSetReconciler(
	reader client.Reader,
	client client.Client,
	scheme *runtime.Scheme,
) *StatefulSetReconciler {
	return &StatefulSetReconciler{
		statefulSets: kubejects.NewApiRequests[
			appsv1.StatefulSet,
			*appsv1.StatefulSet,
			appsv1.StatefulSetList,
			*appsv1.StatefulSetList,
		](
			context.TODO(),
			reader,
			client,
			scheme,
		),
	}
}

//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;create;update;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StatefulSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *StatefulSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger = log.FromContext(ctx)

	toScale := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}
	return ctrl.Result{}, r.reconcileAutoscaler(toScale)
}

// SetupWithManager sets up the controller with the Manager.
func (r *StatefulSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	predicate, err := predicate.LabelSelectorPredicate(statefulSetLabelSelector)
	if err != nil {
		return errors.WithStack(err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.StatefulSet{}).
		WithEventFilter(predicate).
		Complete(r)
}

func (r *StatefulSetReconciler) reconcileAutoscaler(toScale *appsv1.StatefulSet) error {
	deployed, err := r.statefulSets.Get(toScale)
	switch {
	case k8serrors.IsNotFound(err):
		err = nil
	case deployed != nil:
		logger.Info("found statefulset to reconcile")
		err = autoscaler.NewReconciler(
			context.TODO(),
			r.statefulSets.Reader,
			r.statefulSets.Client,
			r.statefulSets.Scheme,
			deployed,
		).Reconcile()
	}

	return errors.WithStack(err)
}
