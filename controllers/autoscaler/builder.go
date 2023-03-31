package autoscaler

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/romr-of-dt/syn-hpa-controller/controllers/kubejects"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type builder struct {
	*appsv1.StatefulSet
}

const syntheticUtilizationDynaQuery = `dsfm:synthetic.engine_utilization:filter(eq("dt.entity.synthetic_location","%s")):merge("host.name","dt.active_gate.working_mode","dt.active_gate.id","location.name"):fold(avg)`

var (
	externalMetricTargetValueMetricQuantity = kubejects.NewQuantity("80")

	behaviorScaleUpStabilizationWindowSeconds   = int32(0)
	behaviorScaleDownStabilizationWindowSeconds = int32(300)

	hpaScalingPolicies = []autoscalingv2.HPAScalingPolicy{
		{
			Type:          autoscalingv2.PodsScalingPolicy,
			Value:         1,
			PeriodSeconds: 600,
		},
	}

	defaultMinReplicas = int32(1)
	defaultMaxReplicas = int32(2)
)

func newBuilder(statefulSet *appsv1.StatefulSet) *builder {
	return &builder{
		StatefulSet: statefulSet,
	}
}

func (b *builder) newAutoscaler() (*autoscalingv2.HorizontalPodAutoscaler, error) {
	minChangePolicySelect := autoscalingv2.MinChangePolicySelect

	autoscaler := autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.StatefulSet.Name,
			Namespace: b.StatefulSet.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/component":  "synthetic",
				"app.kubernetes.io/created-by": b.StatefulSet.Name,
				"app.kubernetes.io/managed-by": "synthetic-controller",
				"app.kubernetes.io/name":       b.StatefulSet.Name,
			},
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind:       "StatefulSet",
				Name:       b.StatefulSet.Name,
				APIVersion: "apps/v1",
			},
			MinReplicas: &defaultMinReplicas,
			MaxReplicas: defaultMaxReplicas,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ExternalMetricSourceType,
					External: &autoscalingv2.ExternalMetricSource{
						Metric: autoscalingv2.MetricIdentifier{
							Name: b.buildSyntheticUtilizationDynaQuery(),
						},
						Target: autoscalingv2.MetricTarget{
							Type:  autoscalingv2.ValueMetricType,
							Value: externalMetricTargetValueMetricQuantity,
						},
					},
				},
			},
			Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleUp: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: &behaviorScaleUpStabilizationWindowSeconds,
					SelectPolicy:               &minChangePolicySelect,
					Policies:                   hpaScalingPolicies,
				},
				ScaleDown: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: &behaviorScaleDownStabilizationWindowSeconds,
					SelectPolicy:               &minChangePolicySelect,
					Policies:                   hpaScalingPolicies,
				},
			},
		},
	}

	hash, err := kubejects.GenerateHash(autoscaler)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	autoscaler.ObjectMeta.Annotations = map[string]string{
		kubejects.AnnotationHash: hash,
	}

	return &autoscaler, nil
}

func (b *builder) buildSyntheticUtilizationDynaQuery() string {
	loc := "unknown"
	for _, e := range b.StatefulSet.Spec.Template.Spec.Containers[0].Env {
		if e.Name == "DT_LOCATION_ID" {
			loc = e.Value
			break
		}
	}

	return fmt.Sprintf(syntheticUtilizationDynaQuery, loc)
}
