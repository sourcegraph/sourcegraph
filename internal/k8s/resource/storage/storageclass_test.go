package storage

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestNewStorageClass(t *testing.T) {
	t.Parallel()

	type args struct {
		name      string
		namespace string
		options   []Option
	}

	tests := []struct {
		name string
		args args
		want storagev1.StorageClass
	}{
		{
			name: "default storageclass",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
			},
			want: storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph-storage",
					},
				},
				ReclaimPolicy:        pointers.Ptr(corev1.PersistentVolumeReclaimRetain),
				AllowVolumeExpansion: pointers.Ptr(true),
				VolumeBindingMode:    pointers.Ptr(storagev1.VolumeBindingWaitForFirstConsumer),
			},
		},
		{
			name: "with labels",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithLabels(map[string]string{
						"app": "bar",
					}),
				},
			},
			want: storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"app":    "bar",
						"deploy": "sourcegraph-storage",
					},
				},
				ReclaimPolicy:        pointers.Ptr(corev1.PersistentVolumeReclaimRetain),
				AllowVolumeExpansion: pointers.Ptr(true),
				VolumeBindingMode:    pointers.Ptr(storagev1.VolumeBindingWaitForFirstConsumer),
			},
		},
		{
			name: "with annotations",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithAnnotations(map[string]string{
						"foo": "bar",
					}),
				},
			},
			want: storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph-storage",
					},
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
				ReclaimPolicy:        pointers.Ptr(corev1.PersistentVolumeReclaimRetain),
				AllowVolumeExpansion: pointers.Ptr(true),
				VolumeBindingMode:    pointers.Ptr(storagev1.VolumeBindingWaitForFirstConsumer),
			},
		},
		{
			name: "with type",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithType("ssd"),
				},
			},
			want: storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph-storage",
					},
				},
				Parameters: map[string]string{
					"type": "ssd",
				},
				ReclaimPolicy:        pointers.Ptr(corev1.PersistentVolumeReclaimRetain),
				AllowVolumeExpansion: pointers.Ptr(true),
				VolumeBindingMode:    pointers.Ptr(storagev1.VolumeBindingWaitForFirstConsumer),
			},
		},
		{
			name: "with parameters",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithParameters(map[string]string{
						"type":  "ssd",
						"stuff": "here",
					}),
				},
			},
			want: storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph-storage",
					},
				},
				Parameters: map[string]string{
					"stuff": "here",
					"type":  "ssd",
				},
				ReclaimPolicy:        pointers.Ptr(corev1.PersistentVolumeReclaimRetain),
				AllowVolumeExpansion: pointers.Ptr(true),
				VolumeBindingMode:    pointers.Ptr(storagev1.VolumeBindingWaitForFirstConsumer),
			},
		},
		{
			name: "with provisioner",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithProvisioner("test"),
				},
			},
			want: storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph-storage",
					},
				},
				Provisioner:          "test",
				ReclaimPolicy:        pointers.Ptr(corev1.PersistentVolumeReclaimRetain),
				AllowVolumeExpansion: pointers.Ptr(true),
				VolumeBindingMode:    pointers.Ptr(storagev1.VolumeBindingWaitForFirstConsumer),
			},
		},
		{
			name: "with reclaim policy",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithReclaimPolicy(corev1.PersistentVolumeReclaimDelete),
				},
			},
			want: storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph-storage",
					},
				},
				ReclaimPolicy:        pointers.Ptr(corev1.PersistentVolumeReclaimDelete),
				AllowVolumeExpansion: pointers.Ptr(true),
				VolumeBindingMode:    pointers.Ptr(storagev1.VolumeBindingWaitForFirstConsumer),
			},
		},
		{
			name: "disallow volume expansion",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					DisallowVolumeExpansion(),
				},
			},
			want: storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph-storage",
					},
				},
				ReclaimPolicy:        pointers.Ptr(corev1.PersistentVolumeReclaimRetain),
				AllowVolumeExpansion: pointers.Ptr(false),
				VolumeBindingMode:    pointers.Ptr(storagev1.VolumeBindingWaitForFirstConsumer),
			},
		},
		{
			name: "with volume binding mode",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithVolumeBindingMode(storagev1.VolumeBindingImmediate),
				},
			},
			want: storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph-storage",
					},
				},
				ReclaimPolicy:        pointers.Ptr(corev1.PersistentVolumeReclaimRetain),
				AllowVolumeExpansion: pointers.Ptr(true),
				VolumeBindingMode:    pointers.Ptr(storagev1.VolumeBindingImmediate),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewStorageClass(tt.args.name, tt.args.namespace, tt.args.options...)
			if err != nil {
				t.Errorf("NewStorageClass() error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("NewStorageClass() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
