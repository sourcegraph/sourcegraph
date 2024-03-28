package storage

import (
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sourcegraph/sourcegraph/internal/maps"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// NewStorageClass creates a new k8s StorageClass with default values.
//
// Default values include:
//
//   - AllowVolumeExpansion: true
//   - ReclaimPolicy: PersistentVolumeReclaimRetain
//   - VolumeBindingMode: VolumeBindingWaitForFirstConsumer
//
// Additional options can be passed to modify the default values.
func NewStorageClass(name, namespace string, options ...Option) (storagev1.StorageClass, error) {
	storageClass := storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"deploy": "sourcegraph-storage",
			},
		},
		AllowVolumeExpansion: pointers.Ptr(true),
		ReclaimPolicy:        pointers.Ptr(corev1.PersistentVolumeReclaimRetain),
		VolumeBindingMode:    pointers.Ptr(storagev1.VolumeBindingWaitForFirstConsumer),
	}

	// apply any given options
	for _, opt := range options {
		err := opt(&storageClass)
		if err != nil {
			return storagev1.StorageClass{}, err
		}
	}

	return storageClass, nil
}

// Option sets an option for a StorageClass.
type Option func(storageClass *storagev1.StorageClass) error

// WithLabels sets StorageClass labels without overriding existing labels.
func WithLabels(labels map[string]string) Option {
	return func(storageClass *storagev1.StorageClass) error {
		storageClass.Labels = maps.MergePreservingExistingKeys(storageClass.Labels, labels)
		return nil
	}
}

// WithAnnotations sets StorageClass annotations without overriding existing annotations.
func WithAnnotations(annotations map[string]string) Option {
	return func(storageClass *storagev1.StorageClass) error {
		storageClass.Annotations = maps.MergePreservingExistingKeys(storageClass.Annotations, annotations)
		return nil
	}
}

// WithType sets the given type on the StorageClass.
func WithType(typeParam string) Option {
	return func(storageClass *storagev1.StorageClass) error {
		storageClass.Parameters = maps.Merge(storageClass.Parameters, map[string]string{
			"type": typeParam,
		})
		return nil
	}
}

// WithParameters sets the given parameters on the StorageClass.
func WithParameters(params map[string]string) Option {
	return func(storageClass *storagev1.StorageClass) error {
		storageClass.Parameters = maps.MergePreservingExistingKeys(storageClass.Parameters, params)
		return nil
	}
}

// WithProvisioner sets the given provisioner on the StorageClass.
func WithProvisioner(provisioner string) Option {
	return func(storageClass *storagev1.StorageClass) error {
		storageClass.Provisioner = provisioner
		return nil
	}
}

// WithReclaimPolicy sets the given PersistentVolumeReclaimPolicy on the StorageClass.
func WithReclaimPolicy(reclaimPolicy corev1.PersistentVolumeReclaimPolicy) Option {
	return func(storageClass *storagev1.StorageClass) error {
		storageClass.ReclaimPolicy = &reclaimPolicy
		return nil
	}
}

// AllowVolumeExpansion allows volume expansion on the StorageClass.
func AllowVolumeExpansion() Option {
	return func(storageClass *storagev1.StorageClass) error {
		storageClass.AllowVolumeExpansion = pointers.Ptr(true)
		return nil
	}
}

// DisallowVolumeExpansion disallows volume expansion on the StorageClass.
func DisallowVolumeExpansion() Option {
	return func(storageClass *storagev1.StorageClass) error {
		storageClass.AllowVolumeExpansion = pointers.Ptr(false)
		return nil
	}
}

// WithVolumeBindingMode sets the given VolumeBindingMode on the StorageClasss.
func WithVolumeBindingMode(volumeBindingMode storagev1.VolumeBindingMode) Option {
	return func(storageClass *storagev1.StorageClass) error {
		storageClass.VolumeBindingMode = &volumeBindingMode
		return nil
	}
}
