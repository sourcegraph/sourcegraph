package container

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// NewContainer creates a new k8s Container with default values.
//
// Default values include:
//
//   - ImagePullPolicy: PullIfNotPresent
//   - TerminationMessagePolicy: TerminationMessageFallbackToLogsOnError
//   - CPU/memory resource limits and requests.
//   - SecurityContext with defaults.
//
// Additional options can be passed to modify the default values.
func NewContainer(name string, options ...Option) (corev1.Container, error) {
	container := corev1.Container{
		Name:                     name,
		ImagePullPolicy:          corev1.PullIfNotPresent,
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("500Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                pointers.Ptr[int64](100),
			RunAsGroup:               pointers.Ptr[int64](101),
			AllowPrivilegeEscalation: pointers.Ptr(false),
			ReadOnlyRootFilesystem:   pointers.Ptr(true),
		},
	}

	// apply any given options
	for _, opt := range options {
		err := opt(&container)
		if err != nil {
			return corev1.Container{}, err
		}
	}

	return container, nil
}

// Option sets an option for a Container.
type Option func(container *corev1.Container) error

// WithImage sets the image field on the Container.
func WithImage(image string) Option {
	return func(container *corev1.Container) error {
		container.Image = image
		return nil
	}
}

// WithCommand sets the command field on the Container to the provided
// command and arguments.
func WithCommand(command ...string) Option {
	return func(container *corev1.Container) error {
		container.Command = command
		return nil
	}
}

// WithArgs sets the args field on the Container to the provided arguments.
func WithArgs(args ...string) Option {
	return func(container *corev1.Container) error {
		container.Args = args
		return nil
	}
}

// WithWorkingDir sets the working directory on the Container.
func WithWorkingDir(dir string) Option {
	return func(container *corev1.Container) error {
		container.WorkingDir = dir
		return nil
	}
}

// WithPorts adds the provided ContainerPorts to the Container if they do not already exist.
// It also sorts the ports by name for accurate comparison of resources.
func WithPorts(ports ...corev1.ContainerPort) Option {
	return func(container *corev1.Container) error {
		for _, p := range ports {
			if !portExists(p.Name, container) {
				container.Ports = append(container.Ports, p)
			}
		}

		// sort ports by names
		sort.SliceStable(container.Ports, func(i, j int) bool {
			return container.Ports[i].Name < container.Ports[j].Name
		})
		return nil
	}
}

func portExists(name string, container *corev1.Container) bool {
	for _, p := range container.Ports {
		if p.Name == name {
			return true
		}
	}
	return false
}

// WithEnv adds the provided EnvVars to the Container if they do not already exist.
func WithEnv(vars ...corev1.EnvVar) Option {
	return func(container *corev1.Container) error {
		for _, v := range vars {
			if envExists(v.Name, container) {
				continue
			}
			container.Env = append(container.Env, v)
		}
		return nil
	}
}

func envExists(name string, container *corev1.Container) bool {
	for _, env := range container.Env {
		if env.Name == name {
			return true
		}
	}
	return false
}

// WithResources sets the given ResourceRequirements on the Container.
func WithResources(resources corev1.ResourceRequirements) Option {
	return func(container *corev1.Container) error {
		container.Resources = resources
		return nil
	}
}

// WithVolumeMounts adds the provided VolumeMounts to the Container if they do not already exist.
// It also sorts the VolumeMounts by name for accurate comparison of resources.
func WithVolumeMounts(volumeMounts []corev1.VolumeMount) Option {
	return func(container *corev1.Container) error {
		for _, v := range volumeMounts {
			if !volumeMountExists(v, container) {
				container.VolumeMounts = append(container.VolumeMounts, v)
			}
		}

		sort.SliceStable(container.VolumeMounts, func(i, j int) bool {
			return container.VolumeMounts[i].Name < container.VolumeMounts[j].Name
		})
		return nil
	}
}

func volumeMountExists(volumeMount corev1.VolumeMount, container *corev1.Container) bool {
	for _, volMount := range container.VolumeMounts {
		if volMount.Name == volumeMount.Name || volMount.MountPath == volumeMount.MountPath {
			return true
		}
	}
	return false
}

// WithLivenessProbe sets the given *corev1.Probe as the LivenessProbe on the Container.
func WithLivenessProbe(livenessProbe *corev1.Probe) Option {
	return func(container *corev1.Container) error {
		container.LivenessProbe = livenessProbe
		return nil
	}
}

// WithDefaultLivenessProbe sets a default LivenessProbe on the Container
// that is commonly used for Sourcegraph services.
//
// The default probe is:
//
//	&corev1.Probe{
//		ProbeHandler: corev1.ProbeHandler{
//			HTTPGet: &corev1.HTTPGetAction{
//				Path:   "/",
//				Port:   intstr.FromString(container.Name),
//				Scheme: corev1.URISchemeHTTP,
//			},
//		},
//		InitialDelaySeconds: 60,
//		TimeoutSeconds:      5,
//	}
func WithDefaultLivenessProbe() Option {
	return func(container *corev1.Container) error {
		container.LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/",
					Port:   intstr.FromString(container.Name),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 60,
			TimeoutSeconds:      5,
		}
		return nil
	}
}

// WithReadinessProbe sets the given *corev1.Probe as the ReadinessProbe on the Container.
func WithReadinessProbe(readinessProbe *corev1.Probe) Option {
	return func(container *corev1.Container) error {
		container.ReadinessProbe = readinessProbe
		return nil
	}
}

// WithDefaultReadinessProbe sets a default ReadinessProbe on the Container
// that is commonly used for Sourcegraph services.
//
// The default probe is:
//
//	&corev1.Probe{
//		ProbeHandler: corev1.ProbeHandler{
//			HTTPGet: &corev1.HTTPGetAction{
//				Path:   "/",
//				Port:   intstr.FromString(container.Name),
//				Scheme: corev1.URISchemeHTTP,
//			},
//		},
//		PeriodSeconds:  5,
//		TimeoutSeconds: 5,
//	}
func WithDefaultReadinessProbe() Option {
	return func(container *corev1.Container) error {
		container.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/",
					Port:   intstr.FromString(container.Name),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			PeriodSeconds:  5,
			TimeoutSeconds: 5,
		}
		return nil
	}
}

// WithSecurityContext sets the given corev1.SecurityContext on the Container.
func WithSecurityContext(securityContext corev1.SecurityContext) Option {
	return func(container *corev1.Container) error {
		container.SecurityContext = &securityContext
		return nil
	}
}
