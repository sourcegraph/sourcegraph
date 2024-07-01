package reconciler

import (
	"bytes"
	"context"
	"text/template"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sourcegraph/sourcegraph/internal/appliance/config"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/configmap"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/container"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/pod"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/pvc"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/service"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/serviceaccount"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/statefulset"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func (r *Reconciler) reconcileGrafana(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	if err := r.reconcileGrafanaStatefulSet(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling StatefulSet")
	}
	if err := r.reconcileGrafanaService(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling Service")
	}
	if err := r.reconcileGrafanaServiceAccount(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling ServiceAccount")
	}
	if err := r.reconcileGrafanaConfigMap(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling ConfigMap")
	}
	return nil
}

func (r *Reconciler) reconcileGrafanaStatefulSet(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	name := "grafana"
	cfg := sg.Spec.Grafana

	defaultImage := config.GetDefaultImage(sg, name)
	ctr := container.NewContainer(name, cfg, config.ContainerConfig{
		Image: defaultImage,
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
	})
	ctr.TerminationMessagePolicy = corev1.TerminationMessageFallbackToLogsOnError

	ctr.Ports = []corev1.ContainerPort{
		{Name: "http", ContainerPort: 3370},
	}

	ctr.VolumeMounts = []corev1.VolumeMount{
		{Name: "grafana-data", MountPath: "/var/lib/grafana"},
		{Name: "config", MountPath: "/sg_config_grafana/provisioning/datasources"},
	}

	ctr.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: pointers.Ptr(false),
		RunAsUser:                pointers.Ptr(int64(472)),
		RunAsGroup:               pointers.Ptr(int64(472)),
		ReadOnlyRootFilesystem:   pointers.Ptr(true),
	}

	podTemplate := pod.NewPodTemplate(name, cfg)
	podTemplate.Template.Spec.Containers = []corev1.Container{ctr}
	podTemplate.Template.Spec.ServiceAccountName = name
	podTemplate.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
		RunAsUser:           pointers.Ptr(int64(472)),
		RunAsGroup:          pointers.Ptr(int64(472)),
		FSGroup:             pointers.Ptr(int64(472)),
		FSGroupChangePolicy: pointers.Ptr(corev1.FSGroupChangeOnRootMismatch),
	}

	cfgMapName := name
	if cfg.ExistingConfigMap != "" {
		cfgMapName = cfg.ExistingConfigMap
	}
	podTemplate.Template.Spec.Volumes = []corev1.Volume{
		pod.NewVolumeFromConfigMap("config", cfgMapName),
	}

	pvc, err := pvc.NewPersistentVolumeClaim("grafana-data", sg.Namespace, sg.Spec.Grafana)
	if err != nil {
		return err
	}

	sset := statefulset.NewStatefulSet(name, sg.Namespace, sg.Spec.RequestedVersion)
	sset.Spec.Template = podTemplate.Template
	sset.Spec.Replicas = &cfg.Replicas
	sset.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{pvc}

	return reconcileObject(ctx, r, cfg, &sset, &appsv1.StatefulSet{}, sg, owner)
}

func (r *Reconciler) reconcileGrafanaService(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	name := "grafana"
	cfg := sg.Spec.Grafana
	svc := service.NewService(name, sg.Namespace, cfg)
	svc.Spec.Ports = []corev1.ServicePort{
		{Name: "http", TargetPort: intstr.FromString("http"), Port: 3181},
		{Name: "debug", TargetPort: intstr.FromString("debug"), Port: 6060},
	}
	svc.Spec.Selector = map[string]string{
		"app": name,
	}
	return reconcileObject(ctx, r, cfg, &svc, &corev1.Service{}, sg, owner)
}

func (r *Reconciler) reconcileGrafanaServiceAccount(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	cfg := sg.Spec.Grafana
	sa := serviceaccount.NewServiceAccount("grafana", sg.Namespace, cfg)
	return reconcileObject(ctx, r, cfg, &sa, &corev1.ServiceAccount{}, sg, owner)
}

func (r *Reconciler) reconcileGrafanaConfigMap(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	cfg := sg.Spec.Grafana
	if cfg.ExistingConfigMap != "" {
		return nil
	}

	tmpl, err := template.New("grafana-config").Parse(string(config.GrafanaDefaultConfigTemplate))
	if err != nil {
		return errors.Wrap(err, "parsing default grafana config template")
	}
	var defaultConfig bytes.Buffer
	if err := tmpl.Execute(&defaultConfig, sg); err != nil {
		return errors.Wrap(err, "rendering default grafana config template")
	}

	name := "grafana"
	cm := configmap.NewConfigMap(name, sg.Namespace)
	cm.Data = map[string]string{
		"grafana.yml":     defaultConfig.String(),
		"extra_rules.yml": "",
	}

	return reconcileObject(ctx, r, cfg, &cm, &corev1.ConfigMap{}, sg, owner)
}
