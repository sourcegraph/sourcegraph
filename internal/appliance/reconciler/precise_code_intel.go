package reconciler

import (
	"context"
	"fmt"

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
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func (r *Reconciler) reconcilePreciseCodeIntel(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	if err := r.reconcilePreciseCodeIntelDeployment(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling Deployment")
	}
	if err := r.reconcilePreciseCodeIntelService(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling Service")
	}
	if err := r.reconcilePreciseCodeIntelServiceAccount(ctx, sg, owner); err != nil {
		return errors.Wrap(err, "reconciling ServiceAccount")
	}
	return nil
}

func (r *Reconciler) reconcilePreciseCodeIntelDeployment(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	name := "precise-code-intel-worker"
	cfg := sg.Spec.PreciseCodeIntel

	defaultImage := config.GetDefaultImage(sg, name)
	ctr := container.NewContainer(name, cfg, config.ContainerConfig{
		Image: defaultImage,
		Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("2G"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("4G"),
			},
		},
	})

	ctr.Env = append(
		ctr.Env,

		// This doesn't appear to be used in this repository, but it is included
		// in order for parity with Helm. It might be read from a library
		// dependency.
		corev1.EnvVar{Name: "NUM_WORKERS", Value: fmt.Sprintf("%d", cfg.NumWorkers)},

		container.NewEnvVarFieldRef("POD_NAME", "metadata.name"),
	)

	ctr.Env = addPreciseCodeIntelBlobstoreVars(ctr.Env, sg)

	ctr.Env = append(ctr.Env, container.EnvVarsOtel()...)

	ctr.Ports = []corev1.ContainerPort{
		{Name: "http", ContainerPort: 3188},
		{Name: "debug", ContainerPort: 6060},
	}
	ctr.LivenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.FromString("debug"),
			},
		},
		InitialDelaySeconds: 60,
		TimeoutSeconds:      5,
	}
	ctr.ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/ready",
				Port: intstr.FromString("debug"),
			},
		},
		PeriodSeconds:  5,
		TimeoutSeconds: 5,
	}
	ctr.VolumeMounts = []corev1.VolumeMount{
		{Name: "tmpdir", MountPath: "/tmp"},
	}

	podTemplate := pod.NewPodTemplate(name, cfg)
	podTemplate.Template.Spec.Containers = []corev1.Container{ctr}
	podTemplate.Template.Spec.Volumes = []corev1.Volume{
		pod.NewVolumeEmptyDir("tmpdir"),
	}

	dep := deployment.NewDeployment(name, sg.Namespace, sg.Spec.RequestedVersion)
	dep.Spec.Replicas = pointers.Ptr(cfg.Replicas)
	dep.Spec.Strategy.RollingUpdate = &appsv1.RollingUpdateDeployment{
		MaxSurge:       pointers.Ptr(intstr.FromInt(1)),
		MaxUnavailable: pointers.Ptr(intstr.FromInt(1)),
	}
	dep.Spec.Template = podTemplate.Template

	return reconcileObject(ctx, r, cfg, &dep, &appsv1.Deployment{}, sg, owner)
}

func (r *Reconciler) reconcilePreciseCodeIntelService(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	name := "precise-code-intel-worker"
	cfg := sg.Spec.PreciseCodeIntel

	svc := service.NewService(name, sg.Namespace, cfg)
	svc.Spec.Ports = []corev1.ServicePort{
		{Name: "http", Port: 3188, TargetPort: intstr.FromString("http")},
		{Name: "debug", Port: 6060, TargetPort: intstr.FromString("debug")},
	}
	svc.Spec.Selector = map[string]string{
		"app": "precise-code-intel-worker",
	}

	return reconcileObject(ctx, r, cfg, &svc, &corev1.Service{}, sg, owner)
}

func (r *Reconciler) reconcilePreciseCodeIntelServiceAccount(ctx context.Context, sg *config.Sourcegraph, owner client.Object) error {
	cfg := sg.Spec.PreciseCodeIntel
	sa := serviceaccount.NewServiceAccount("precise-code-intel-worker", sg.Namespace, cfg)
	return reconcileObject(ctx, r, cfg, &sa, &corev1.ServiceAccount{}, sg, owner)
}

func addPreciseCodeIntelBlobstoreVars(env []corev1.EnvVar, sg *config.Sourcegraph) []corev1.EnvVar {
	// Only set these when the internal blobstore is enabled. Otherwise, callers
	// can supply env vars for external blobstores via ContainerConfig.
	if !sg.Spec.Blobstore.IsDisabled() {
		env = append(
			env,
			corev1.EnvVar{Name: "PRECISE_CODE_INTEL_UPLOAD_BACKEND", Value: "blobstore"},
			corev1.EnvVar{Name: "PRECISE_CODE_INTEL_UPLOAD_AWS_ENDPOINT", Value: "http://blobstore:9000"},
		)
	}
	return env
}
