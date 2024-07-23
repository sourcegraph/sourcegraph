package reconciler

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sourcegraph/sourcegraph/internal/appliance/config"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/service"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/serviceaccount"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (r *Reconciler) reconcileOtelCollector(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	if err := r.reconcileOtelCollectorService(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling Service")
	}
	if err := r.reconcileOtelCollectorServiceAccount(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling ServiceAccount")
	}
	return nil
}

func (r *Reconciler) reconcileOtelCollectorService(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	name := "otel-collector"
	cfg := sg.Spec.OtelCollector

	svc := service.NewService(name, sg.Namespace, cfg)
	svc.Spec.Ports = []corev1.ServicePort{
		{Name: "otlp-grpc", Port: 4317, TargetPort: intstr.FromInt(4317)},
		{Name: "otlp-http", Port: 4318, TargetPort: intstr.FromInt(4318)},
		{Name: "metrics", Port: 8888},
	}
	svc.Spec.Selector = map[string]string{
		"app": name,
	}

	return reconcileObject(ctx, r, cfg, &svc, &corev1.Service{}, sg, owner)
}

func (r *Reconciler) reconcileOtelCollectorServiceAccount(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	cfg := sg.Spec.OtelCollector
	sa := serviceaccount.NewServiceAccount("otel-collector", sg.Namespace, cfg)
	return reconcileObject(ctx, r, cfg, &sa, &corev1.ServiceAccount{}, sg, owner)
}
