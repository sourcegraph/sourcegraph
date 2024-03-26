package pod

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/container"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestNewPodTemplate(t *testing.T) {
	t.Parallel()

	type args struct {
		name      string
		namespace string
		options   []Option
	}

	tests := []struct {
		name    string
		args    args
		want    corev1.PodTemplate
		wantErr bool
	}{
		{
			name: "default container",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
			},
			want: corev1.PodTemplate{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "sourcegraph",
						Annotations: map[string]string{
							"kubectl.kubernetes.io/default-container": "foo",
						},
						Labels: map[string]string{
							"app":    "foo",
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
			},
		},
		{
			name: "with labels",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithLabels(map[string]string{
						"app":         "bar",
						"environment": "prod",
					}),
				},
			},
			want: corev1.PodTemplate{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "sourcegraph",
						Annotations: map[string]string{
							"kubectl.kubernetes.io/default-container": "foo",
						},
						Labels: map[string]string{
							"app":         "foo",
							"deploy":      "sourcegraph",
							"environment": "prod",
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
			},
		},
		{
			name: "with annotations",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithAnnotations(map[string]string{
						"kubectl.kubernetes.io/default-container": "bar",
						"environment": "prod",
					}),
				},
			},
			want: corev1.PodTemplate{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "sourcegraph",
						Annotations: map[string]string{
							"kubectl.kubernetes.io/default-container": "foo",
							"environment": "prod",
						},
						Labels: map[string]string{
							"app":    "foo",
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
			},
		},
		{
			name: "with affinity",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithAffinity(corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution:  nil,
							PreferredDuringSchedulingIgnoredDuringExecution: nil,
						},
						PodAffinity:     nil,
						PodAntiAffinity: nil,
					}),
				},
			},
			want: corev1.PodTemplate{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "sourcegraph",
						Annotations: map[string]string{
							"kubectl.kubernetes.io/default-container": "foo",
						},
						Labels: map[string]string{
							"app":    "foo",
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
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution:  nil,
								PreferredDuringSchedulingIgnoredDuringExecution: nil,
							},
							PodAffinity:     nil,
							PodAntiAffinity: nil,
						},
					},
				},
			},
		},
		{
			name: "with volumes",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithVolumes([]corev1.Volume{
						{
							Name: "stuff",
						},
						{
							Name: "data",
						},
					}),
				},
			},
			want: corev1.PodTemplate{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "sourcegraph",
						Annotations: map[string]string{
							"kubectl.kubernetes.io/default-container": "foo",
						},
						Labels: map[string]string{
							"app":    "foo",
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
						Volumes: []corev1.Volume{
							{
								Name: "data",
							},
							{
								Name: "stuff",
							},
						},
					},
				},
			},
		},
		{
			name: "with termination grace period",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithTerminationGracePeriod(int64(10)),
				},
			},
			want: corev1.PodTemplate{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "sourcegraph",
						Annotations: map[string]string{
							"kubectl.kubernetes.io/default-container": "foo",
						},
						Labels: map[string]string{
							"app":    "foo",
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
						TerminationGracePeriodSeconds: pointers.Ptr[int64](10),
					},
				},
			},
		},
		{
			name: "with init containers",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithInitContainers(func() corev1.Container {
						c, _ := container.NewContainer("foo")
						return c
					}()),
				},
			},
			want: corev1.PodTemplate{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "sourcegraph",
						Annotations: map[string]string{
							"kubectl.kubernetes.io/default-container": "foo",
						},
						Labels: map[string]string{
							"app":    "foo",
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
						InitContainers: []corev1.Container{
							{
								Name:                     "foo",
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
							},
						},
					},
				},
			},
		},
		{
			name: "with containers",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithContainers(func() corev1.Container {
						c, _ := container.NewContainer("foo")
						return c
					}()),
				},
			},
			want: corev1.PodTemplate{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "sourcegraph",
						Annotations: map[string]string{
							"kubectl.kubernetes.io/default-container": "foo",
						},
						Labels: map[string]string{
							"app":    "foo",
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
						Containers: []corev1.Container{
							{
								Name:                     "foo",
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
							},
						},
					},
				},
			},
		},
		{
			name: "with service account",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithServiceAccount("foobar"),
				},
			},
			want: corev1.PodTemplate{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "sourcegraph",
						Annotations: map[string]string{
							"kubectl.kubernetes.io/default-container": "foo",
						},
						Labels: map[string]string{
							"app":    "foo",
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
						ServiceAccountName: "foobar",
					},
				},
			},
		},
		{
			name: "with security context",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					WithSecurityContext(corev1.PodSecurityContext{
						RunAsUser:           pointers.Ptr[int64](999),
						RunAsGroup:          pointers.Ptr[int64](101),
						FSGroup:             pointers.Ptr[int64](101),
						FSGroupChangePolicy: pointers.Ptr(corev1.FSGroupChangeAlways),
					}),
				},
			},
			want: corev1.PodTemplate{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "sourcegraph",
						Annotations: map[string]string{
							"kubectl.kubernetes.io/default-container": "foo",
						},
						Labels: map[string]string{
							"app":    "foo",
							"deploy": "sourcegraph",
						},
					},
					Spec: corev1.PodSpec{
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser:           pointers.Ptr[int64](999),
							RunAsGroup:          pointers.Ptr[int64](101),
							FSGroup:             pointers.Ptr[int64](101),
							FSGroupChangePolicy: pointers.Ptr(corev1.FSGroupChangeAlways),
						},
					},
				},
			},
		},
		{
			name: "with error",
			args: args{
				name:      "foo",
				namespace: "sourcegraph",
				options: []Option{
					func(podTemplate *corev1.PodTemplate) error {
						return errors.New("test error")
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewPodTemplate(tt.args.name, tt.args.namespace, tt.args.options...)
			if err != nil && tt.wantErr == false {
				t.Errorf("NewContainer() error: %v", err)
			}
			if err != nil && tt.wantErr == true {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("NewContainer() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
