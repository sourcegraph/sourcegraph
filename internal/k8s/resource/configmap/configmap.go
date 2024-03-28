package configmap

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sourcegraph/sourcegraph/internal/maps"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// NewConfigMap creates a new k8s ConfigMap with default values.
//
// Default values include:
//
//   - Immutable set to false.
//
// Additional options can be passed to modify the default values.
func NewConfigMap(name, namespace string, options ...Option) (corev1.ConfigMap, error) {
	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"deploy": "sourcegraph",
			},
		},
		Immutable: pointers.Ptr(false),
	}

	// apply any given options
	for _, opt := range options {
		err := opt(&configMap)
		if err != nil {
			return corev1.ConfigMap{}, err
		}
	}
	return configMap, nil
}

// Option sets an option for a ConfigMap.
type Option func(configMap *corev1.ConfigMap) error

// WithLabels set ConfigMap labels without overriding existing labels.
func WithLabels(labels map[string]string) Option {
	return func(configMap *corev1.ConfigMap) error {
		configMap.Labels = maps.MergePreservingExistingKeys(configMap.Labels, labels)
		return nil
	}
}

// WithAnnotations set ConfigMap annotations without overriding existing annotations.
func WithAnnotations(annotations map[string]string) Option {
	return func(configMap *corev1.ConfigMap) error {
		configMap.Annotations = maps.MergePreservingExistingKeys(configMap.Annotations, annotations)
		return nil
	}
}

// Immutable sets a ConfigMap to be immutable.
func Immutable() Option {
	return func(configMap *corev1.ConfigMap) error {
		configMap.Immutable = pointers.Ptr(true)
		return nil
	}
}

// WithData sets the given string data to a ConfigMap.
func WithData(data map[string]string) Option {
	return func(configMap *corev1.ConfigMap) error {
		configMap.Data = data
		return nil
	}
}

// WithBinaryData sets the given binary data to a ConfigMap.
func WithBinaryData(data map[string][]byte) Option {
	return func(configMap *corev1.ConfigMap) error {
		configMap.BinaryData = data
		return nil
	}
}
