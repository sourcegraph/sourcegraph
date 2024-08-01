package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/config"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestConfig_Load(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetMockGetter(func(name, defaultValue, description string) string {
		switch name {
		case "EXECUTOR_QUEUE_POLL_INTERVAL":
			return "10s"
		case "EXECUTOR_MAXIMUM_NUM_JOBS":
			return "10"
		case "EXECUTOR_USE_FIRECRACKER":
			return "true"
		case "EXECUTOR_KEEP_WORKSPACES":
			return "true"
		case "EXECUTOR_JOB_NUM_CPUS":
			return "8"
		case "EXECUTOR_FIRECRACKER_BANDWIDTH_INGRESS":
			return "100"
		case "EXECUTOR_FIRECRACKER_BANDWIDTH_EGRESS":
			return "100"
		case "EXECUTOR_MAXIMUM_RUNTIME_PER_JOB":
			return "1m"
		case "EXECUTOR_CLEANUP_TASK_INTERVAL":
			return "10m"
		case "EXECUTOR_NUM_TOTAL_JOBS":
			return "10"
		case "EXECUTOR_MAX_ACTIVE_TIME":
			return "1h"
		case "EXECUTOR_ENABLE_JOB_AUDIT_LOGGING":
			return "true"
		case "EXECUTOR_KUBERNETES_CONFIG_PATH":
			return "/foo/bar"
		case "EXECUTOR_KUBERNETES_NODE_NAME":
			return "my-node"
		case "EXECUTOR_KUBERNETES_NODE_SELECTOR":
			return "app=my-app,zone=west"
		case "EXECUTOR_KUBERNETES_NODE_REQUIRED_AFFINITY_MATCH_EXPRESSIONS":
			return `[{"key": "foo", "operator": "In", "values": ["bar"]}]`
		case "EXECUTOR_KUBERNETES_NODE_REQUIRED_AFFINITY_MATCH_FIELDS":
			return `[{"key": "faz", "operator": "In", "values": ["baz"]}]`
		case "EXECUTOR_KUBERNETES_POD_AFFINITY":
			return `[{"labelSelector": {"matchExpressions": [{"key": "foo", "operator": "In", "values": ["bar"]}]}, "topologyKey": "kubernetes.io/hostname"}]`
		case "EXECUTOR_KUBERNETES_POD_ANTI_AFFINITY":
			return `[{"labelSelector": {"matchExpressions": [{"key": "foo", "operator": "In", "values": ["bar"]}]}, "topologyKey": "kubernetes.io/hostname"}]`
		case "EXECUTOR_KUBERNETES_NODE_TOLERATIONS":
			return `[{"key": "foo", "operator": "Equal", "value": "bar", "effect": "NoSchedule"}]`
		case "KUBERNETES_JOB_VOLUME_TYPE":
			return "pvc"
		case "KUBERNETES_JOB_VOLUME_SIZE":
			return "10Gi"
		case "KUBERNETES_ADDITIONAL_JOB_VOLUMES":
			return `[{"name": "foo", "configMap": {"name": "bar"}}]`
		case "KUBERNETES_ADDITIONAL_JOB_VOLUME_MOUNTS":
			return `[{"name": "foo", "mountPath": "/foo"}]`
		case "KUBERNETES_JOB_STEP_IMAGE":
			return "sourcegraph/step-image:latest"
		case "KUBERNETES_JOB_ANNOTATIONS":
			return `{"foo": "bar", "faz": "baz"}`
		case "KUBERNETES_JOB_POD_ANNOTATIONS":
			return `{"foo": "bar", "faz": "baz"}`
		case "KUBERNETES_IMAGE_PULL_SECRETS":
			return "foo,bar"
		default:
			return name
		}
	})
	cfg.Load()

	assert.Equal(t, "EXECUTOR_FRONTEND_URL", cfg.FrontendURL)
	assert.Equal(t, "EXECUTOR_FRONTEND_PASSWORD", cfg.FrontendAuthorizationToken)
	assert.True(t, cfg.EnableJobAuditLogging)
	assert.Equal(t, "EXECUTOR_QUEUE_NAME", cfg.QueueName)
	assert.Equal(t, "EXECUTOR_QUEUE_NAMES", cfg.QueueNamesStr)
	assert.Equal(t, 10*time.Second, cfg.QueuePollInterval)
	assert.Equal(t, 10, cfg.MaximumNumJobs)
	assert.True(t, cfg.UseFirecracker)
	assert.Equal(t, "EXECUTOR_FIRECRACKER_IMAGE", cfg.FirecrackerImage)
	assert.Equal(t, "EXECUTOR_FIRECRACKER_KERNEL_IMAGE", cfg.FirecrackerKernelImage)
	assert.Equal(t, "EXECUTOR_FIRECRACKER_SANDBOX_IMAGE", cfg.FirecrackerSandboxImage)
	assert.Equal(t, "EXECUTOR_VM_STARTUP_SCRIPT_PATH", cfg.VMStartupScriptPath)
	assert.Equal(t, "EXECUTOR_VM_PREFIX", cfg.VMPrefix)
	assert.True(t, cfg.KeepWorkspaces)
	assert.Equal(t, "EXECUTOR_DOCKER_HOST_MOUNT_PATH", cfg.DockerHostMountPath)
	assert.Equal(t, 8, cfg.JobNumCPUs)
	assert.Equal(t, "EXECUTOR_JOB_MEMORY", cfg.JobMemory)
	assert.Equal(t, "EXECUTOR_FIRECRACKER_DISK_SPACE", cfg.FirecrackerDiskSpace)
	assert.Equal(t, 100, cfg.FirecrackerBandwidthIngress)
	assert.Equal(t, 100, cfg.FirecrackerBandwidthEgress)
	assert.Equal(t, 1*time.Minute, cfg.MaximumRuntimePerJob)
	assert.Equal(t, 10*time.Minute, cfg.CleanupTaskInterval)
	assert.Equal(t, 10, cfg.NumTotalJobs)
	assert.Equal(t, "NODE_EXPORTER_URL", cfg.NodeExporterURL)
	assert.Equal(t, "DOCKER_REGISTRY_NODE_EXPORTER_URL", cfg.DockerRegistryNodeExporterURL)
	assert.Equal(t, time.Hour, cfg.MaxActiveTime)
	assert.Equal(t, "EXECUTOR_DOCKER_REGISTRY_MIRROR_URL", cfg.DockerRegistryMirrorURL)
	assert.Equal(t, "/foo/bar", cfg.KubernetesConfigPath)
	assert.Equal(t, "my-node", cfg.KubernetesNodeName)
	assert.Equal(t, "app=my-app,zone=west", cfg.KubernetesNodeSelector)
	assert.Equal(
		t,
		[]corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpIn, Values: []string{"bar"}}},
		cfg.KubernetesNodeRequiredAffinityMatchExpressions,
	)
	assert.Equal(
		t,
		[]corev1.NodeSelectorRequirement{{Key: "faz", Operator: corev1.NodeSelectorOpIn, Values: []string{"baz"}}},
		cfg.KubernetesNodeRequiredAffinityMatchFields,
	)
	assert.Equal(
		t,
		[]corev1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "foo",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"bar"},
						},
					},
				},
				TopologyKey: "kubernetes.io/hostname",
			},
		},
		cfg.KubernetesPodAffinity,
	)
	assert.Equal(
		t,
		[]corev1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "foo",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"bar"},
						},
					},
				},
				TopologyKey: "kubernetes.io/hostname",
			},
		},
		cfg.KubernetesPodAntiAffinity,
	)
	assert.Equal(
		t,
		[]corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpEqual, Value: "bar", Effect: corev1.TaintEffectNoSchedule}},
		cfg.KubernetesNodeTolerations,
	)
	assert.Equal(t, "pvc", cfg.KubernetesJobVolumeType)
	assert.Equal(t, "10Gi", cfg.KubernetesJobVolumeSize)
	assert.Equal(
		t,
		[]corev1.Volume{{Name: "foo", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "bar"}}}}},
		cfg.KubernetesAdditionalJobVolumes,
	)
	assert.Equal(
		t,
		[]corev1.VolumeMount{{Name: "foo", MountPath: "/foo"}},
		cfg.KubernetesAdditionalJobVolumeMounts,
	)
	assert.Equal(t, "sourcegraph/step-image:latest", cfg.KubernetesJobStepImage)

	assert.Len(t, cfg.KubernetesJobAnnotations, 2)
	assert.Equal(t, "bar", cfg.KubernetesJobAnnotations["foo"])
	assert.Equal(t, "baz", cfg.KubernetesJobAnnotations["faz"])

	assert.Len(t, cfg.KubernetesJobPodAnnotations, 2)
	assert.Equal(t, "bar", cfg.KubernetesJobPodAnnotations["foo"])
	assert.Equal(t, "baz", cfg.KubernetesJobPodAnnotations["faz"])

	assert.Equal(t, "foo,bar", cfg.KubernetesImagePullSecrets)
}

func TestConfig_Load_Defaults(t *testing.T) {
	cfg := &config.Config{}
	cfg.Load()

	assert.Empty(t, cfg.FrontendURL)
	assert.Empty(t, cfg.FrontendAuthorizationToken)
	assert.False(t, cfg.EnableJobAuditLogging)
	assert.Empty(t, cfg.QueueName)
	assert.Empty(t, cfg.QueueNamesStr)
	assert.Equal(t, time.Second, cfg.QueuePollInterval)
	assert.Equal(t, 1, cfg.MaximumNumJobs)
	assert.Equal(t, "sourcegraph/executor-vm:insiders", cfg.FirecrackerImage)
	assert.Equal(t, "sourcegraph/ignite-kernel:5.10.135-amd64", cfg.FirecrackerKernelImage)
	assert.Equal(t, "sourcegraph/ignite:v0.10.5", cfg.FirecrackerSandboxImage)
	assert.Empty(t, cfg.VMStartupScriptPath)
	assert.Equal(t, "executor", cfg.VMPrefix)
	assert.False(t, cfg.KeepWorkspaces)
	assert.Empty(t, cfg.DockerHostMountPath)
	assert.Equal(t, 4, cfg.JobNumCPUs)
	assert.Equal(t, "12G", cfg.JobMemory)
	assert.Equal(t, "20G", cfg.FirecrackerDiskSpace)
	assert.Equal(t, 524288000, cfg.FirecrackerBandwidthIngress)
	assert.Equal(t, 524288000, cfg.FirecrackerBandwidthEgress)
	assert.Equal(t, 30*time.Minute, cfg.MaximumRuntimePerJob)
	assert.Equal(t, 1*time.Minute, cfg.CleanupTaskInterval)
	assert.Zero(t, cfg.NumTotalJobs)
	assert.Empty(t, cfg.NodeExporterURL)
	assert.Empty(t, cfg.DockerRegistryNodeExporterURL)
	assert.Zero(t, cfg.MaxActiveTime)
	assert.Empty(t, cfg.DockerRegistryMirrorURL)
	assert.Empty(t, cfg.KubernetesNodeName)
	assert.Empty(t, cfg.KubernetesNodeSelector)
	assert.Nil(t, cfg.KubernetesNodeRequiredAffinityMatchExpressions)
	assert.Nil(t, cfg.KubernetesNodeRequiredAffinityMatchFields)
	assert.Equal(t, "default", cfg.KubernetesNamespace)
	assert.Equal(t, "sg-executor-pvc", cfg.KubernetesPersistenceVolumeName)
	assert.Empty(t, cfg.KubernetesResourceLimitCPU)
	assert.Equal(t, "12Gi", cfg.KubernetesResourceLimitMemory)
	assert.Empty(t, cfg.KubernetesResourceRequestCPU)
	assert.Equal(t, "12Gi", cfg.KubernetesResourceRequestMemory)
	assert.Equal(t, 1200, cfg.KubernetesJobDeadline)
	assert.False(t, cfg.KubernetesKeepJobs)
	assert.Equal(t, -1, cfg.KubernetesSecurityContextRunAsUser)
	assert.Equal(t, -1, cfg.KubernetesSecurityContextRunAsGroup)
	assert.Equal(t, 1000, cfg.KubernetesSecurityContextFSGroup)
	assert.Equal(t, "emptyDir", cfg.KubernetesJobVolumeType)
	assert.Equal(t, "5Gi", cfg.KubernetesJobVolumeSize)
	assert.Empty(t, cfg.KubernetesAdditionalJobVolumes)
	assert.Empty(t, cfg.KubernetesAdditionalJobVolumeMounts)
	assert.Equal(t, "sourcegraph/batcheshelper:insiders", cfg.KubernetesJobStepImage)
	assert.Nil(t, cfg.KubernetesJobAnnotations)
	assert.Nil(t, cfg.KubernetesJobPodAnnotations)
	assert.Empty(t, cfg.KubernetesImagePullSecrets)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		getterFunc  env.GetterFunc
		expectedErr error
	}{
		{
			name: "Valid config",
			getterFunc: func(name string, defaultValue, description string) string {
				switch name {
				case "EXECUTOR_QUEUE_NAME":
					return "batches"
				case "EXECUTOR_FRONTEND_URL":
					return "http://some-url.com"
				case "EXECUTOR_FRONTEND_PASSWORD":
					return "some-password"
				default:
					return defaultValue
				}
			},
		},
		{
			name:        "Default config",
			expectedErr: errors.New("4 errors occurred:\n\t* invalid value \"\" for EXECUTOR_FRONTEND_URL: no value supplied\n\t* invalid value \"\" for EXECUTOR_FRONTEND_PASSWORD: no value supplied\n\t* neither EXECUTOR_QUEUE_NAME or EXECUTOR_QUEUE_NAMES is set\n\t* EXECUTOR_FRONTEND_URL must be in the format scheme://host (and optionally :port)"),
		},
		{
			name: "Invalid EXECUTOR_DOCKER_AUTH_CONFIG",
			getterFunc: func(name string, defaultValue, description string) string {
				switch name {
				case "EXECUTOR_QUEUE_NAME":
					return "batches"
				case "EXECUTOR_FRONTEND_URL":
					return "http://some-url.com"
				case "EXECUTOR_FRONTEND_PASSWORD":
					return "some-password"
				case "EXECUTOR_DOCKER_AUTH_CONFIG":
					return `{"foo": "bar"`
				default:
					return defaultValue
				}
			},
			expectedErr: errors.New("invalid EXECUTOR_DOCKER_AUTH_CONFIG, failed to parse: unexpected end of JSON input"),
		},
		{
			name: "Invalid frontend URL",
			getterFunc: func(name string, defaultValue, description string) string {
				switch name {
				case "EXECUTOR_QUEUE_NAME":
					return "batches"
				case "EXECUTOR_FRONTEND_URL":
					return "sourcegraph.example.com"
				case "EXECUTOR_FRONTEND_PASSWORD":
					return "some-password"
				default:
					return defaultValue
				}
			},
			expectedErr: errors.New("EXECUTOR_FRONTEND_URL must be in the format scheme://host (and optionally :port)"),
		},
		{
			name: "EXECUTOR_QUEUE_NAME and EXECUTOR_QUEUE_NAMES both defined",
			getterFunc: func(name string, defaultValue, description string) string {
				switch name {
				case "EXECUTOR_QUEUE_NAME":
					return "batches"
				case "EXECUTOR_QUEUE_NAMES":
					return "batches,codeintel"
				case "EXECUTOR_FRONTEND_URL":
					return "http://some-url.com"
				case "EXECUTOR_FRONTEND_PASSWORD":
					return "some-password"
				default:
					return defaultValue
				}
			},
			expectedErr: errors.New("both EXECUTOR_QUEUE_NAME and EXECUTOR_QUEUE_NAMES are set"),
		},
		{
			name: "Neither EXECUTOR_QUEUE_NAME or EXECUTOR_QUEUE_NAMES defined",
			getterFunc: func(name string, defaultValue, description string) string {
				switch name {
				case "EXECUTOR_FRONTEND_URL":
					return "http://some-url.com"
				case "EXECUTOR_FRONTEND_PASSWORD":
					return "some-password"
				default:
					return defaultValue
				}
			},
			expectedErr: errors.New("neither EXECUTOR_QUEUE_NAME or EXECUTOR_QUEUE_NAMES is set"),
		},
		{
			name: "EXECUTOR_QUEUE_NAMES using incorrect separator",
			getterFunc: func(name string, defaultValue, description string) string {
				switch name {
				case "EXECUTOR_FRONTEND_URL":
					return "http://some-url.com"
				case "EXECUTOR_QUEUE_NAMES":
					return "batches;codeintel"
				case "EXECUTOR_FRONTEND_PASSWORD":
					return "some-password"
				default:
					return defaultValue
				}
			},
			expectedErr: errors.New("EXECUTOR_QUEUE_NAMES contains invalid queue name 'batches;codeintel', valid names are 'batches, codeintel' and should be comma-separated"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.SetMockGetter(test.getterFunc)
			cfg.Load()

			err := cfg.Validate()
			if test.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
