package reconciler

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sourcegraph/sourcegraph/internal/appliance/config"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/configmap"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/container"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/daemonset"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/pod"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/serviceaccount"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (r *Reconciler) reconcileOtelAgent(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	if err := r.reconcileOtelAgentConfigmap(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling ConfigMap")
	}
	if err := r.reconcileOtelAgentServiceAccount(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling ServiceAccount")
	}
	if err := r.reconcileOtelAgentDaemonset(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling DaemonSet")
	}
	return nil
}

func (r *Reconciler) reconcileOtelAgentConfigmap(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	name := "otel-agent"
	cfg := sg.Spec.OtelAgent
	cm := configmap.NewConfigMap(name, sg.Namespace)
	cm.Data = map[string]string{
		"config.yaml": string(config.OtelAgentConfig),
	}
	return reconcileObject(ctx, r, cfg, &cm, &corev1.ConfigMap{}, sg, owner)
}

func (r *Reconciler) reconcileOtelAgentServiceAccount(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	cfg := sg.Spec.OtelAgent
	sa := serviceaccount.NewServiceAccount("otel-agent", sg.Namespace, cfg)
	return reconcileObject(ctx, r, cfg, &sa, &corev1.ServiceAccount{}, sg, owner)
}

func (r *Reconciler) reconcileOtelAgentDaemonset(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	name := "otel-agent"
	cfg := sg.Spec.OtelAgent

	ctr := container.NewContainer(name, cfg, config.ContainerConfig{
		Image: config.GetDefaultImage(sg, "opentelemetry-collector"),
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("500Mi"),
			},
		},
	})
	ctr.Command = []string{"/bin/otelcol-sourcegraph", "--config=/etc/otel-agent/config.yaml"}

	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{Path: "/", Port: intstr.FromInt(13133)},
		},
	}
	ctr.ReadinessProbe = probe
	ctr.LivenessProbe = probe

	ctr.Ports = []corev1.ContainerPort{
		{Name: "zpages", ContainerPort: 55679, HostPort: 55679},
		{Name: "otel-grpc", ContainerPort: 4317, HostPort: 4317},
		{Name: "otel-http", ContainerPort: 4318, HostPort: 4318},
	}

	ctr.VolumeMounts = []corev1.VolumeMount{
		{Name: "config", MountPath: "/etc/otel-agent"},
	}

	template := pod.NewPodTemplate(name, cfg)
	template.Template.Spec.Containers = []corev1.Container{ctr}

	cfgVol := pod.NewVolumeFromConfigMap("config", name)
	cfgVol.VolumeSource.ConfigMap.Items = []corev1.KeyToPath{
		{Key: "config.yaml", Path: "config.yaml"},
	}
	template.Template.Spec.Volumes = []corev1.Volume{cfgVol}

	ds := daemonset.New(name, sg.Namespace, sg.Spec.RequestedVersion)
	ds.Spec.Template = template.Template

	return reconcileObject(ctx, r, cfg, &ds, &appsv1.DaemonSet{}, sg, owner)
}
