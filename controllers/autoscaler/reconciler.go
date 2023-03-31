package autoscaler

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/romr-of-dt/syn-hpa-controller/controllers/kubejects"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Reconciler struct {
	*builder
	log logr.Logger

	autoscalers *kubejects.ApiRequests[
		autoscalingv2.HorizontalPodAutoscaler,
		*autoscalingv2.HorizontalPodAutoscaler,
		autoscalingv2.HorizontalPodAutoscalerList,
		*autoscalingv2.HorizontalPodAutoscalerList,
	]
	foundAutoscaler *autoscalingv2.HorizontalPodAutoscaler
}

//nolint:revive
func NewReconciler(
	context context.Context,
	reader client.Reader,
	client client.Client,
	scheme *runtime.Scheme,
	statefulSet *appsv1.StatefulSet,
) *Reconciler {
	return &Reconciler{
		builder: newBuilder(statefulSet),
		log:     log.FromContext(context),
		autoscalers: kubejects.NewApiRequests[
			autoscalingv2.HorizontalPodAutoscaler,
			*autoscalingv2.HorizontalPodAutoscaler,
			autoscalingv2.HorizontalPodAutoscalerList,
			*autoscalingv2.HorizontalPodAutoscalerList,
		](
			context,
			reader,
			client,
			scheme,
		),
	}
}

func (r *Reconciler) Reconcile() error {
	toReconcile, err := r.builder.newAutoscaler()
	if err != nil {
		return errors.WithStack(err)
	}

	err = r.findAutoscaler(toReconcile)
	if err != nil {
		return errors.WithStack(err)
	}

	if r.ignores(toReconcile) {
		return nil
	}

	switch {
	case r.foundAutoscaler == nil:
		err = r.create(toReconcile)
	case r.foundAutoscaler != nil:
		err = r.update(toReconcile)
	}

	return errors.WithStack(err)
}

func (r *Reconciler) findAutoscaler(toFind *autoscalingv2.HorizontalPodAutoscaler) (err error) {
	r.foundAutoscaler, err = r.autoscalers.Get(toFind)
	if apierrors.IsNotFound(err) {
		err = nil
	}

	return err
}

func (r *Reconciler) ignores(toReconcile *autoscalingv2.HorizontalPodAutoscaler) bool {
	return r.foundAutoscaler != nil &&
		toReconcile.GetAnnotations()[kubejects.AnnotationHash] ==
			r.foundAutoscaler.GetAnnotations()[kubejects.AnnotationHash]
}

func (r *Reconciler) create(toCreate *autoscalingv2.HorizontalPodAutoscaler) error {
	err := r.autoscalers.Create(
		r.builder.StatefulSet,
		toCreate)
	if err != nil {
		r.log.Error(
			err,
			"could not create",
			"name", toCreate.Name)
		return errors.WithStack(err)
	}

	r.log.Info("created", "name", toCreate.Name)
	return nil
}

func (r *Reconciler) update(toUpdate *autoscalingv2.HorizontalPodAutoscaler) error {
	err := r.autoscalers.Update(r.builder.StatefulSet, toUpdate)
	if err == nil {
		r.log.Info("updated", "name", toUpdate.Name)
	}

	return errors.WithStack(err)
}
