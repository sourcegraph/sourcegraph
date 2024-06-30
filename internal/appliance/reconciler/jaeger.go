package reconciler

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sourcegraph/sourcegraph/internal/appliance/config"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/container"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/deployment"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/pod"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/service"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/serviceaccount"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (r *Reconciler) reconcileJaeger(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	if err := r.reconcileJaegerDeployment(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling Deployment")
	}
	if err := r.reconcileJaegerQueryService(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling Service")
	}
	if err := r.reconcileJaegerCollectorService(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling Service")
	}
	if err := r.reconcileJaegerServiceAccount(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling ServiceAccount")
	}

	return nil
}

func (r *Reconciler) reconcileJaegerDeployment(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	name := "jaeger"
	cfg := sg.Spec.Jaeger

	defaultImage := config.GetDefaultImage(sg, name)
	ctr := container.NewContainer(name, cfg, config.ContainerConfig{
		Image: defaultImage,
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("500M"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1G"),
			},
		},
	})

	ctr.Ports = []corev1.ContainerPort{
		{ContainerPort: 5775, Protocol: corev1.ProtocolUDP},
		{ContainerPort: 6831, Protocol: corev1.ProtocolUDP},
		{ContainerPort: 6832, Protocol: corev1.ProtocolUDP},
		{ContainerPort: 5778, Protocol: corev1.ProtocolTCP},
		{ContainerPort: 16686, Protocol: corev1.ProtocolTCP},
		{ContainerPort: 14250, Protocol: corev1.ProtocolTCP},
	}
	ctr.ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/",
				Port: intstr.FromInt(16686),
			},
		},
		InitialDelaySeconds: 10,
	}

	podTemplate := pod.NewPodTemplate(name, cfg)
	podTemplate.Template.Spec.Containers = []corev1.Container{ctr}
	podTemplate.Template.Spec.ServiceAccountName = name

	dep := deployment.NewDeployment(name, sg.Namespace, sg.Spec.RequestedVersion)
	dep.Spec.Replicas = &cfg.Replicas
	dep.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType
	dep.Spec.Template = podTemplate.Template

	return reconcileObject(ctx, r, cfg, &dep, &appsv1.Deployment{}, sg, owner)
}

func (r *Reconciler) reconcileJaegerQueryService(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	name := "jaeger-query"
	cfg := sg.Spec.Jaeger
	svc := service.NewService(name, sg.Namespace, cfg)
	svc.Spec.Ports = []corev1.ServicePort{
		{Name: "query-http", Port: 16686, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(16686)},
	}
	svc.Spec.Selector = map[string]string{
		"app":                         "jaeger",
		"app.kubernetes.io/component": "all-in-one",
	}
	return reconcileObject(ctx, r, cfg, &svc, &corev1.Service{}, sg, owner)
}

func (r *Reconciler) reconcileJaegerCollectorService(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	name := "jaeger-collector"
	cfg := sg.Spec.Jaeger
	svc := service.NewService(name, sg.Namespace, cfg)
	svc.Spec.Ports = []corev1.ServicePort{
		{Name: "jaeger-collector-tchannel", Port: 14267, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(14267)},
		{Name: "jaeger-collector-http", Port: 14268, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(14268)},
		{Name: "jaeger-collector-grpc", Port: 14250, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt(14250)},
	}
	svc.Spec.Selector = map[string]string{
		"app":                         "jaeger",
		"app.kubernetes.io/component": "all-in-one",
	}
	return reconcileObject(ctx, r, cfg, &svc, &corev1.Service{}, sg, owner)
}

func (r *Reconciler) reconcileJaegerServiceAccount(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	cfg := sg.Spec.Jaeger
	sa := serviceaccount.NewServiceAccount("jaeger", sg.Namespace, cfg)
	return reconcileObject(ctx, r, cfg, &sa, &corev1.ServiceAccount{}, sg, owner)
}
