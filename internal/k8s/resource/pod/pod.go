package pod

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sourcegraph/sourcegraph/internal/maps"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// NewPodTemplate creates a new k8s PodTemplate with default values.
//
// Default values include:
//
//   - Default container annotations
//   - Default Sourcegraph `app` and `deploy` labels
//   - SecurityContext with defaults.
//
// Additional options can be passed to modify the default values.
func NewPodTemplate(name, namespace string, options ...Option) (corev1.PodTemplate, error) {
	podTemplate := corev1.PodTemplate{
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Annotations: map[string]string{
					"kubectl.kubernetes.io/default-container": name,
				},
				Labels: map[string]string{
					"app":    name,
					"deploy": "sourcegraph",
				},
			},
			Spec: corev1.PodSpec{
				SecurityContext: &corev1.PodSecurityContext{
					RunAsUser:           pointers.Ptr[int64](100),
					RunAsGroup:          pointers.Ptr[int64](101),
					FSGroup:             pointers.Ptr[int64](101),
					FSGroupChangePolicy: pointers.Ptr(corev1.FSGroupChangeOnRootMismatch),
				},
			},
		},
	}

	// apply any given options
	for _, opt := range options {
		err := opt(&podTemplate)
		if err != nil {
			return corev1.PodTemplate{}, err
		}
	}

	return podTemplate, nil
}

// Option sets an option for a PodTemplate.
type Option func(podTemplate *corev1.PodTemplate) error

// WithLabels sets PodTemplate labels without overriding existing labels.
func WithLabels(labels map[string]string) Option {
	return func(podTemplate *corev1.PodTemplate) error {
		podTemplate.Template.Labels = maps.MergePreservingExistingKeys(podTemplate.Template.Labels, labels)
		return nil
	}
}

// WithAnnotations sets PodTemplate annotations without overriding existing annotations.
func WithAnnotations(annotations map[string]string) Option {
	return func(podTemplate *corev1.PodTemplate) error {
		podTemplate.Template.Annotations = maps.MergePreservingExistingKeys(podTemplate.Template.Annotations, annotations)
		return nil
	}
}

// WithAffinity sets a default affinity on the PodTemplate.
func WithAffinity(affinity corev1.Affinity) Option {
	return func(podTemplate *corev1.PodTemplate) error {
		podTemplate.Template.Spec.Affinity = &affinity
		return nil
	}
}

// WithVolumes appends the given volumes to the PodSpec without overriding existing volumes.
func WithVolumes(volumes []corev1.Volume) Option {
	return func(podTemplate *corev1.PodTemplate) error {
		for _, v := range volumes {
			if !volumeExists(v.Name, podTemplate) {
				podTemplate.Template.Spec.Volumes = append(podTemplate.Template.Spec.Volumes, v)
			}
		}

		sort.SliceStable(podTemplate.Template.Spec.Volumes, func(i, j int) bool {
			return podTemplate.Template.Spec.Volumes[i].Name < podTemplate.Template.Spec.Volumes[j].Name
		})
		return nil
	}
}

func volumeExists(name string, podTemplate *corev1.PodTemplate) bool {
	for _, v := range podTemplate.Template.Spec.Volumes {
		if v.Name == name {
			return true
		}
	}
	return false
}

// WithTerminationGracePeriod sets the given termination grace period.
func WithTerminationGracePeriod(period int64) Option {
	return func(podTemplate *corev1.PodTemplate) error {
		podTemplate.Template.Spec.TerminationGracePeriodSeconds = &period
		return nil
	}
}

// WithInitContainers appends the given InitContainers to the Pod without overriding existing containers.
func WithInitContainers(initContainers ...corev1.Container) Option {
	return func(podTemplate *corev1.PodTemplate) error {
		for _, c := range initContainers {
			if !initContainerExists(c.Name, podTemplate) {
				podTemplate.Template.Spec.InitContainers = append(podTemplate.Template.Spec.InitContainers, c)
			}
		}
		return nil
	}
}

func initContainerExists(name string, podTemplate *corev1.PodTemplate) bool {
	for _, c := range podTemplate.Template.Spec.InitContainers {
		if c.Name == name {
			return true
		}
	}
	return false
}

// WithContainers appends the given containers to the Pod without overriding existing containers.
func WithContainers(containers ...corev1.Container) Option {
	return func(podTemplate *corev1.PodTemplate) error {
		for _, c := range containers {
			if !containerExists(c.Name, podTemplate) {
				podTemplate.Template.Spec.Containers = append(podTemplate.Template.Spec.Containers, c)
			}
		}
		return nil
	}
}

func containerExists(name string, podTemplate *corev1.PodTemplate) bool {
	for _, c := range podTemplate.Template.Spec.Containers {
		if c.Name == name {
			return true
		}
	}
	return false
}

// WithServiceAccount sets the given Service Account on the pod.
func WithServiceAccount(serviceAccount string) Option {
	return func(podTemplate *corev1.PodTemplate) error {
		podTemplate.Template.Spec.ServiceAccountName = serviceAccount
		return nil
	}
}

// WithSecurityContext sets the given Pod Security Context on the pod.
func WithSecurityContext(securityContext corev1.PodSecurityContext) Option {
	return func(podTemplate *corev1.PodTemplate) error {
		podTemplate.Template.Spec.SecurityContext = &securityContext
		return nil
	}
}
