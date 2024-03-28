package configmap

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestNewConfigMap(t *testing.T) {
	t.Parallel()

	type args struct {
		name      string
		namespace string
		options   []Option
	}

	tests := []struct {
		name string
		args args
		want corev1.ConfigMap
	}{
		{
			name: "default configmap",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
			},
			want: corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph",
					},
				},
				Immutable: pointers.Ptr(false),
			},
		},
		{
			name: "with labels",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithLabels(map[string]string{
						"foo": "bar",
					}),
				},
			},
			want: corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph",
						"foo":    "bar",
					},
				},
				Immutable: pointers.Ptr(false),
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
			want: corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph",
					},
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
				Immutable: pointers.Ptr(false),
			},
		},
		{
			name: "immutable",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					Immutable(),
				},
			},
			want: corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph",
					},
				},
				Immutable: pointers.Ptr(true),
			},
		},
		{
			name: "with data",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithData(map[string]string{
						"data": "foo",
					}),
				},
			},
			want: corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph",
					},
				},
				Immutable: pointers.Ptr(false),
				Data: map[string]string{
					"data": "foo",
				},
			},
		},
		{
			name: "with binary data",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithBinaryData(map[string][]byte{
						"data": []byte("foo"),
					}),
				},
			},
			want: corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "sourcegraph",
					Labels: map[string]string{
						"deploy": "sourcegraph",
					},
				},
				Immutable: pointers.Ptr(false),
				Data:      nil,
				BinaryData: map[string][]byte{
					"data": []byte("foo"),
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewConfigMap(tt.args.name, tt.args.namespace, tt.args.options...)
			if err != nil {
				t.Errorf("NewConfigMap() error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("NewConfigMap() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
