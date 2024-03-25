package container

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestNewContainer(t *testing.T) {
	t.Parallel()

	type args struct {
		name    string
		options []Option
	}

	tests := []struct {
		name string
		args args
		want corev1.Container
	}{
		{
			name: "default container",
			args: args{
				name: "foo",
			},
			want: corev1.Container{
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
		{
			name: "with image",
			args: args{
				name: "foo",
				options: []Option{
					WithImage("ghcr.io/sourcegraph/service"),
				},
			},
			want: corev1.Container{
				Name:                     "foo",
				ImagePullPolicy:          corev1.PullIfNotPresent,
				Image:                    "ghcr.io/sourcegraph/service",
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
		{
			name: "with command",
			args: args{
				name: "foo",
				options: []Option{
					WithCommand("ls", "-a"),
				},
			},
			want: corev1.Container{
				Name:                     "foo",
				ImagePullPolicy:          corev1.PullIfNotPresent,
				TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				Command: []string{
					"ls",
					"-a",
				},
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
		{
			name: "with args",
			args: args{
				name: "foo",
				options: []Option{
					WithArgs("foo", "bar"),
				},
			},
			want: corev1.Container{
				Name:                     "foo",
				ImagePullPolicy:          corev1.PullIfNotPresent,
				TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				Args: []string{
					"foo",
					"bar",
				},
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
		{
			name: "with working dir",
			args: args{
				name: "foo",
				options: []Option{
					WithWorkingDir("/home/foo"),
				},
			},
			want: corev1.Container{
				Name:                     "foo",
				ImagePullPolicy:          corev1.PullIfNotPresent,
				TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				WorkingDir:               "/home/foo",
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
		{
			name: "with ports",
			args: args{
				name: "foo",
				options: []Option{
					WithPorts([]corev1.ContainerPort{
						{
							Name:          "prometheus",
							HostPort:      9000,
							ContainerPort: 9000,
							Protocol:      "TCP",
						},
						{
							// try to add duplicate port
							Name:          "prometheus",
							HostPort:      9000,
							ContainerPort: 9000,
							Protocol:      "TCP",
						},
						{
							// ports should be sorted by function
							Name:          "http",
							HostPort:      80,
							ContainerPort: 80,
							Protocol:      "TCP",
						},
					}...),
				},
			},
			want: corev1.Container{
				Name:                     "foo",
				ImagePullPolicy:          corev1.PullIfNotPresent,
				TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				Ports: []corev1.ContainerPort{
					{
						Name:          "http",
						HostPort:      80,
						ContainerPort: 80,
						Protocol:      "TCP",
					},
					{
						Name:          "prometheus",
						HostPort:      9000,
						ContainerPort: 9000,
						Protocol:      "TCP",
					},
				},
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
		{
			name: "with env",
			args: args{
				name: "foo",
				options: []Option{
					WithEnv(corev1.EnvVar{
						Name:      "DEBUG",
						Value:     "TRUE",
						ValueFrom: nil,
					}),
					WithEnv(corev1.EnvVar{
						Name:      "DEBUG",
						Value:     "FALSE",
						ValueFrom: nil,
					}),
				},
			},
			want: corev1.Container{
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
				Env: []corev1.EnvVar{
					{
						Name:      "DEBUG",
						Value:     "TRUE",
						ValueFrom: nil,
					},
				},
			},
		},
		{
			name: "with resources",
			args: args{
				name: "foo",
				options: []Option{
					WithResources(corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10"),
							corev1.ResourceMemory: resource.MustParse("5G"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10"),
							corev1.ResourceMemory: resource.MustParse("5G"),
						},
					}),
				},
			},
			want: corev1.Container{
				Name:                     "foo",
				ImagePullPolicy:          corev1.PullIfNotPresent,
				TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("5G"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10"),
						corev1.ResourceMemory: resource.MustParse("5G"),
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
		{
			name: "with volume mounts",
			args: args{
				name: "foo",
				options: []Option{
					WithVolumeMounts([]corev1.VolumeMount{
						{
							Name:      "stuff",
							ReadOnly:  true,
							MountPath: "/data",
						},
						{
							Name:      "other stuff",
							ReadOnly:  false,
							MountPath: "/tmp",
						},
						{
							Name:      "other stuff",
							ReadOnly:  false,
							MountPath: "/var/log",
						},
					}),
				},
			},
			want: corev1.Container{
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
				VolumeMounts: []corev1.VolumeMount{
					// Notice the order here, volumes should be sorted
					// and won't overwrite an existing volume
					{
						Name:      "other stuff",
						ReadOnly:  false,
						MountPath: "/tmp",
					},
					{
						Name:      "stuff",
						ReadOnly:  true,
						MountPath: "/data",
					},
				},
			},
		},
		{
			name: "with liveness probe",
			args: args{
				name: "foo",
				options: []Option{
					WithLivenessProbe(&corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/ready",
								Port:   intstr.FromString("foo"),
								Scheme: corev1.URISchemeHTTP,
							},
						},
						PeriodSeconds:                 5,
						TerminationGracePeriodSeconds: pointers.Ptr[int64](10),
					}),
				},
			},
			want: corev1.Container{
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
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/ready",
							Port:   intstr.FromString("foo"),
							Scheme: corev1.URISchemeHTTP,
						},
					},
					PeriodSeconds:                 5,
					TerminationGracePeriodSeconds: pointer.Int64(10),
				},
			},
		},
		{
			name: "with default liveness probe",
			args: args{
				name: "foo",
				options: []Option{
					WithDefaultLivenessProbe(),
				},
			},
			want: corev1.Container{
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
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/",
							Port:   intstr.FromString("foo"),
							Scheme: corev1.URISchemeHTTP,
						},
					},
					InitialDelaySeconds: 60,
					TimeoutSeconds:      5,
				},
			},
		},
		{
			name: "with readiness probe",
			args: args{
				name: "foo",
				options: []Option{
					WithReadinessProbe(&corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/ready",
								Port:   intstr.FromString("foo"),
								Scheme: corev1.URISchemeHTTP,
							},
						},
						InitialDelaySeconds:           6,
						TimeoutSeconds:                10,
						PeriodSeconds:                 10,
						SuccessThreshold:              3,
						FailureThreshold:              10,
						TerminationGracePeriodSeconds: pointer.Int64(10),
					}),
				},
			},
			want: corev1.Container{
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
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/ready",
							Port:   intstr.FromString("foo"),
							Scheme: corev1.URISchemeHTTP,
						},
					},
					InitialDelaySeconds:           6,
					TimeoutSeconds:                10,
					PeriodSeconds:                 10,
					SuccessThreshold:              3,
					FailureThreshold:              10,
					TerminationGracePeriodSeconds: pointer.Int64(10),
				},
			},
		},
		{
			name: "with default readiness probe",
			args: args{
				name: "foo",
				options: []Option{
					WithDefaultReadinessProbe(),
				},
			},
			want: corev1.Container{
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
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/",
							Port:   intstr.FromString("foo"),
							Scheme: corev1.URISchemeHTTP,
						},
					},
					TimeoutSeconds: 5,
					PeriodSeconds:  5,
				},
			},
		},
		{
			name: "with security context",
			args: args{
				name: "foo",
				options: []Option{
					WithSecurityContext(corev1.SecurityContext{
						Privileged:               pointers.Ptr(true),
						RunAsUser:                pointers.Ptr[int64](5000),
						RunAsGroup:               pointers.Ptr[int64](9000),
						ReadOnlyRootFilesystem:   pointers.Ptr(true),
						AllowPrivilegeEscalation: pointers.Ptr(false),
					}),
				},
			},
			want: corev1.Container{
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
					Privileged:               pointers.Ptr(true),
					RunAsUser:                pointers.Ptr[int64](5000),
					RunAsGroup:               pointers.Ptr[int64](9000),
					ReadOnlyRootFilesystem:   pointers.Ptr(true),
					AllowPrivilegeEscalation: pointers.Ptr(false),
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewContainer(tt.args.name, tt.args.options...)
			if err != nil {
				t.Errorf("NewContainer() error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("NewContainer() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
