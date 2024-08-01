package config

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/homedir"
	"k8s.io/utils/strings/slices"

	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
	"github.com/sourcegraph/sourcegraph/internal/hostname"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Config struct {
	env.BaseConfig

	FrontendURL                                    string
	FrontendAuthorizationToken                     string
	EnableJobAuditLogging                          bool
	QueueName                                      string
	QueueNamesStr                                  string
	QueueNames                                     []string
	QueuePollInterval                              time.Duration
	MaximumNumJobs                                 int
	FirecrackerImage                               string
	FirecrackerKernelImage                         string
	FirecrackerSandboxImage                        string
	VMStartupScriptPath                            string
	VMPrefix                                       string
	KeepWorkspaces                                 bool
	DockerHostMountPath                            string
	UseFirecracker                                 bool
	JobNumCPUs                                     int
	JobMemory                                      string
	FirecrackerDiskSpace                           string
	FirecrackerBandwidthIngress                    int
	FirecrackerBandwidthEgress                     int
	MaximumRuntimePerJob                           time.Duration
	CleanupTaskInterval                            time.Duration
	NumTotalJobs                                   int
	MaxActiveTime                                  time.Duration
	NodeExporterURL                                string
	DockerRegistryNodeExporterURL                  string
	WorkerHostname                                 string
	DockerRegistryMirrorURL                        string
	DockerAddHostGateway                           bool
	DockerAuthConfig                               types.DockerAuthConfig
	KubernetesConfigPath                           string
	KubernetesNodeName                             string
	KubernetesNodeSelector                         string
	KubernetesNodeRequiredAffinityMatchExpressions []corev1.NodeSelectorRequirement
	KubernetesNodeRequiredAffinityMatchFields      []corev1.NodeSelectorRequirement
	KubernetesPodAffinity                          []corev1.PodAffinityTerm
	KubernetesPodAntiAffinity                      []corev1.PodAffinityTerm
	KubernetesNodeTolerations                      []corev1.Toleration
	KubernetesNamespace                            string
	KubernetesPersistenceVolumeName                string
	KubernetesResourceLimitCPU                     string
	KubernetesResourceLimitMemory                  string
	KubernetesResourceRequestCPU                   string
	KubernetesResourceRequestMemory                string
	KubernetesJobDeadline                          int
	KubernetesKeepJobs                             bool
	KubernetesSecurityContextRunAsUser             int
	KubernetesSecurityContextRunAsGroup            int
	KubernetesSecurityContextFSGroup               int
	KubernetesJobAnnotations                       map[string]string
	KubernetesJobPodAnnotations                    map[string]string
	KubernetesImagePullSecrets                     string
	KubernetesJobVolumeType                        string
	KubernetesJobVolumeSize                        string
	KubernetesAdditionalJobVolumes                 []corev1.Volume
	KubernetesAdditionalJobVolumeMounts            []corev1.VolumeMount
	KubernetesJobStepImage                         string
	// TODO remove in 5.2 if we have moved to a custom image to do the setup work.
	KubernetesGitCACert string

	dockerAuthConfigStr                                          string
	dockerAuthConfigUnmarshalError                               error
	kubernetesNodeRequiredAffinityMatchExpressions               string
	kubernetesNodeRequiredAffinityMatchExpressionsUnmarshalError error
	kubernetesNodeRequiredAffinityMatchFields                    string
	kubernetesNodeRequiredAffinityMatchFieldsUnmarshalError      error
	kubernetesPodAffinity                                        string
	kubernetesPodAffinityUnmarshalError                          error
	kubernetesPodAntiAffinity                                    string
	kubernetesPodAntiAffinityUnmarshalError                      error
	kubernetesNodeTolerations                                    string
	kubernetesNodeTolerationsUnmarshalError                      error
	kubernetesAdditionalJobVolumeMounts                          string
	kubernetesAdditionalJobVolumeMountsUnmarshalError            error
	kubernetesAdditionalJobVolumes                               string
	kubernetesAdditionalJobVolumesUnmarshalError                 error
	kubernetesJobAnnotations                                     string
	kubernetesJobAnnotationsUnmarshalError                       error
	kubernetesJobPodAnnotations                                  string
	kubernetesJobPodAnnotationsUnmarshalError                    error

	defaultFrontendPassword string
}

func (c *Config) Load() {
	c.FrontendURL = c.Get("EXECUTOR_FRONTEND_URL", "", "The external URL of the sourcegraph instance.")
	c.FrontendAuthorizationToken = c.Get("EXECUTOR_FRONTEND_PASSWORD", c.defaultFrontendPassword, "The authorization token supplied to the frontend.")
	c.EnableJobAuditLogging = c.GetBool("EXECUTOR_ENABLE_JOB_AUDIT_LOGGING", "false", "Enables logging of the job payload to the executor logs. Note that this mode might contain secret information and logs very verbosely.")
	c.QueueName = c.GetOptional("EXECUTOR_QUEUE_NAME", "The name of the queue to listen to.")
	c.QueueNamesStr = c.GetOptional("EXECUTOR_QUEUE_NAMES", "The names of multiple queues to listen to, comma-separated.")
	c.QueuePollInterval = c.GetInterval("EXECUTOR_QUEUE_POLL_INTERVAL", "1s", "Interval between dequeue requests.")
	c.MaximumNumJobs = c.GetInt("EXECUTOR_MAXIMUM_NUM_JOBS", "1", "Number of virtual machines or containers that can be running at once.")
	c.UseFirecracker = c.GetBool("EXECUTOR_USE_FIRECRACKER", strconv.FormatBool(runtime.GOOS == "linux" && !IsKubernetes()), "Whether to isolate commands in virtual machines. Requires ignite and firecracker. Linux hosts only. Kubernetes is not supported.")
	c.FirecrackerImage = c.Get("EXECUTOR_FIRECRACKER_IMAGE", DefaultFirecrackerImage, "The base image to use for virtual machines.")
	c.FirecrackerKernelImage = c.Get("EXECUTOR_FIRECRACKER_KERNEL_IMAGE", DefaultFirecrackerKernelImage, "The base image containing the kernel binary to use for virtual machines.")
	c.FirecrackerSandboxImage = c.Get("EXECUTOR_FIRECRACKER_SANDBOX_IMAGE", DefaultFirecrackerSandboxImage, "The OCI image for the ignite VM sandbox.")
	c.VMStartupScriptPath = c.GetOptional("EXECUTOR_VM_STARTUP_SCRIPT_PATH", "A path to a file on the host that is loaded into a fresh virtual machine and executed on startup.")
	c.VMPrefix = c.Get("EXECUTOR_VM_PREFIX", "executor", "A name prefix for virtual machines controlled by this instance.")
	c.KeepWorkspaces = c.GetBool("EXECUTOR_KEEP_WORKSPACES", "false", "Whether to skip deletion of workspaces after a job completes (or fails). Note that when Firecracker is enabled that the workspace is initially copied into the VM, so modifications will not be observed.")
	c.DockerHostMountPath = c.GetOptional("EXECUTOR_DOCKER_HOST_MOUNT_PATH", "The target workspace as it resides on the Docker host (used to enable Docker-in-Docker).")
	c.JobNumCPUs = c.GetInt(env.ChooseFallbackVariableName("EXECUTOR_JOB_NUM_CPUS", "EXECUTOR_FIRECRACKER_NUM_CPUS"), "4", "How many CPUs to allocate to each virtual machine or container. A value of zero sets no resource bound (in Docker, but not VMs).")
	c.JobMemory = c.Get(env.ChooseFallbackVariableName("EXECUTOR_JOB_MEMORY", "EXECUTOR_FIRECRACKER_MEMORY"), "12G", "How much memory to allocate to each virtual machine or container. A value of zero sets no resource bound (in Docker, but not VMs).")
	c.FirecrackerDiskSpace = c.Get("EXECUTOR_FIRECRACKER_DISK_SPACE", "20G", "How much disk space to allocate to each virtual machine.")
	c.FirecrackerBandwidthIngress = c.GetInt("EXECUTOR_FIRECRACKER_BANDWIDTH_INGRESS", "524288000", "How much bandwidth to allow for ingress packets to the VM in bytes/s.")
	c.FirecrackerBandwidthEgress = c.GetInt("EXECUTOR_FIRECRACKER_BANDWIDTH_EGRESS", "524288000", "How much bandwidth to allow for egress packets to the VM in bytes/s.")
	c.MaximumRuntimePerJob = c.GetInterval("EXECUTOR_MAXIMUM_RUNTIME_PER_JOB", "30m", "The maximum wall time that can be spent on a single job.")
	c.CleanupTaskInterval = c.GetInterval("EXECUTOR_CLEANUP_TASK_INTERVAL", "1m", "The frequency with which to run periodic cleanup tasks.")
	c.NumTotalJobs = c.GetInt("EXECUTOR_NUM_TOTAL_JOBS", "0", "The maximum number of jobs that will be dequeued by the worker.")
	c.NodeExporterURL = c.GetOptional("NODE_EXPORTER_URL", "The URL of the node_exporter instance, without the /metrics path.")
	c.DockerRegistryNodeExporterURL = c.GetOptional("DOCKER_REGISTRY_NODE_EXPORTER_URL", "The URL of the Docker Registry instance's node_exporter, without the /metrics path.")
	c.MaxActiveTime = c.GetInterval("EXECUTOR_MAX_ACTIVE_TIME", "0", "The maximum time that can be spent by the worker dequeueing records to be handled.")
	c.DockerRegistryMirrorURL = c.GetOptional("EXECUTOR_DOCKER_REGISTRY_MIRROR_URL", "The address of a docker registry mirror to use in firecracker VMs. Supports multiple values, separated with a comma.")
	c.KubernetesConfigPath = c.GetOptional("EXECUTOR_KUBERNETES_CONFIG_PATH", "The path to the Kubernetes config file.")
	c.KubernetesNodeName = c.GetOptional("EXECUTOR_KUBERNETES_NODE_NAME", "The name of the Kubernetes node to run executor jobs in.")
	c.KubernetesNodeSelector = c.GetOptional("EXECUTOR_KUBERNETES_NODE_SELECTOR", "A comma separated list of values to use as a node selector for Kubernetes Jobs. e.g. foo=bar,app=my-app")
	c.kubernetesNodeRequiredAffinityMatchExpressions = c.GetOptional("EXECUTOR_KUBERNETES_NODE_REQUIRED_AFFINITY_MATCH_EXPRESSIONS", "The JSON encoded required affinity match expressions for Kubernetes Jobs. e.g. [{\"key\": \"foo\", \"operator\": \"In\", \"values\": [\"bar\"]}]")
	c.kubernetesNodeRequiredAffinityMatchFields = c.GetOptional("EXECUTOR_KUBERNETES_NODE_REQUIRED_AFFINITY_MATCH_FIELDS", "The JSON encoded required affinity match fields for Kubernetes Jobs. e.g. [{\"key\": \"foo\", \"operator\": \"In\", \"values\": [\"bar\"]}]")
	c.kubernetesPodAffinity = c.GetOptional("EXECUTOR_KUBERNETES_POD_AFFINITY", "The JSON encoded pod affinity for Kubernetes Jobs. e.g. [{\"labelSelector\": {\"matchExpressions\": [{\"key\": \"foo\", \"operator\": \"In\", \"values\": [\"bar\"]}]}, \"topologyKey\": \"kubernetes.io/hostname\"}]")
	c.kubernetesPodAntiAffinity = c.GetOptional("EXECUTOR_KUBERNETES_POD_ANTI_AFFINITY", "The JSON encoded pod anti-affinity for Kubernetes Jobs. e.g. [{\"labelSelector\": {\"matchExpressions\": [{\"key\": \"foo\", \"operator\": \"In\", \"values\": [\"bar\"]}]}, \"topologyKey\": \"kubernetes.io/hostname\"}]")
	c.kubernetesNodeTolerations = c.GetOptional("EXECUTOR_KUBERNETES_NODE_TOLERATIONS", "The JSON encoded tolerations for Kubernetes Jobs. e.g. [{\"key\": \"foo\", \"operator\": \"Equal\", \"value\": \"bar\", \"effect\": \"NoSchedule\"}]")
	c.KubernetesNamespace = c.Get("EXECUTOR_KUBERNETES_NAMESPACE", "default", "The namespace to run executor jobs in.")
	c.KubernetesPersistenceVolumeName = c.Get("EXECUTOR_KUBERNETES_PERSISTENCE_VOLUME_NAME", "sg-executor-pvc", "The name of the Kubernetes persistence volume to use for executor jobs.")
	c.KubernetesResourceLimitCPU = c.GetOptional("EXECUTOR_KUBERNETES_RESOURCE_LIMIT_CPU", "The maximum CPU resource for Kubernetes Jobs.")
	c.KubernetesResourceLimitMemory = c.Get("EXECUTOR_KUBERNETES_RESOURCE_LIMIT_MEMORY", "12Gi", "The maximum memory resource for Kubernetes Jobs.")
	c.KubernetesResourceRequestCPU = c.GetOptional("EXECUTOR_KUBERNETES_RESOURCE_REQUEST_CPU", "The minimum CPU resource for Kubernetes Jobs.")
	c.KubernetesResourceRequestMemory = c.Get("EXECUTOR_KUBERNETES_RESOURCE_REQUEST_MEMORY", "12Gi", "The minimum memory resource for Kubernetes Jobs.")
	c.DockerAddHostGateway = c.GetBool("EXECUTOR_DOCKER_ADD_HOST_GATEWAY", "false", "If true, host.docker.internal will be exposed to the docker commands run by the runtime. Warn: Can be insecure. Only use this if you understand what you're doing. This is mostly used for running against a Sourcegraph on the same host.")
	c.dockerAuthConfigStr = c.GetOptional("EXECUTOR_DOCKER_AUTH_CONFIG", "The content of the docker config file including auth for services. If using firecracker, only static credentials are supported, not credential stores nor credential helpers.")
	c.KubernetesJobDeadline = c.GetInt("KUBERNETES_JOB_DEADLINE", "1200", "The number of seconds after which a Kubernetes job will be terminated.")
	c.KubernetesKeepJobs = c.GetBool("KUBERNETES_KEEP_JOBS", "false", "If true, Kubernetes jobs will not be deleted after they complete. Useful for debugging.")
	c.KubernetesSecurityContextRunAsUser = c.GetInt("KUBERNETES_RUN_AS_USER", "-1", "The user ID to run Kubernetes jobs as.")
	c.KubernetesSecurityContextRunAsGroup = c.GetInt("KUBERNETES_RUN_AS_GROUP", "-1", "The group ID to run Kubernetes jobs as.")
	c.KubernetesSecurityContextFSGroup = c.GetInt("KUBERNETES_FS_GROUP", "1000", "The group ID to run all containers in the Kubernetes jobs as. Defaults to 1000, the group ID of the docker group in the executor container.")
	c.KubernetesJobVolumeType = c.Get("KUBERNETES_JOB_VOLUME_TYPE", "emptyDir", "Determines the type of volume to use with the single job. Options are 'emptyDir' and 'pvc'.")
	c.KubernetesJobVolumeSize = c.Get("KUBERNETES_JOB_VOLUME_SIZE", "5Gi", "Determines the size of the job volume.")
	c.kubernetesAdditionalJobVolumes = c.GetOptional("KUBERNETES_ADDITIONAL_JOB_VOLUMES", "Additional volumes to associate with the Jobs. e.g. [{\"name\": \"my-volume\", \"configMap\": {\"name\": \"cluster-volume\"}}]")
	c.kubernetesAdditionalJobVolumeMounts = c.GetOptional("KUBERNETES_ADDITIONAL_JOB_VOLUME_MOUNTS", "Volumes to mount to the Jobs. e.g. [{\"name\":\"my-volume\", \"mountPath\":\"/foo/bar\"}]")
	c.KubernetesJobStepImage = c.Get("KUBERNETES_JOB_STEP_IMAGE", c.Get("KUBERNETES_SINGLE_JOB_STEP_IMAGE", "sourcegraph/batcheshelper:insiders", ""), "The image to use for intermediate steps in the single job. Defaults to sourcegraph/batcheshelper:latest.")
	c.KubernetesGitCACert = c.GetOptional("KUBERNETES_GIT_CA_CERT", "The CA certificate to use for git operations. If not set, the system CA bundle will be used. e.g. /path/to/ca.crt")
	c.kubernetesJobAnnotations = c.GetOptional("KUBERNETES_JOB_ANNOTATIONS", "The JSON encoded annotations to add to the Kubernetes Jobs. e.g. {\"foo\": \"bar\"}")
	c.kubernetesJobPodAnnotations = c.GetOptional("KUBERNETES_JOB_POD_ANNOTATIONS", "The JSON encoded annotations to add to the Kubernetes Job Pods. e.g. {\"foo\": \"bar\"}")
	c.KubernetesImagePullSecrets = c.GetOptional("KUBERNETES_IMAGE_PULL_SECRETS", "The names of Kubernetes image pull secrets to use for pulling images. e.g. my-secret,my-other-secret")

	if c.QueueNamesStr != "" {
		c.QueueNames = strings.Split(c.QueueNamesStr, ",")
	}

	if c.dockerAuthConfigStr != "" {
		c.dockerAuthConfigUnmarshalError = json.Unmarshal([]byte(c.dockerAuthConfigStr), &c.DockerAuthConfig)
	}

	if c.kubernetesNodeRequiredAffinityMatchExpressions != "" {
		c.kubernetesNodeRequiredAffinityMatchExpressionsUnmarshalError = json.Unmarshal([]byte(c.kubernetesNodeRequiredAffinityMatchExpressions), &c.KubernetesNodeRequiredAffinityMatchExpressions)
	}
	if c.kubernetesNodeRequiredAffinityMatchFields != "" {
		c.kubernetesNodeRequiredAffinityMatchFieldsUnmarshalError = json.Unmarshal([]byte(c.kubernetesNodeRequiredAffinityMatchFields), &c.KubernetesNodeRequiredAffinityMatchFields)
	}
	if c.kubernetesPodAffinity != "" {
		c.kubernetesPodAffinityUnmarshalError = json.Unmarshal([]byte(c.kubernetesPodAffinity), &c.KubernetesPodAffinity)
	}
	if c.kubernetesPodAntiAffinity != "" {
		c.kubernetesPodAntiAffinityUnmarshalError = json.Unmarshal([]byte(c.kubernetesPodAntiAffinity), &c.KubernetesPodAntiAffinity)
	}
	if c.kubernetesNodeTolerations != "" {
		c.kubernetesNodeTolerationsUnmarshalError = json.Unmarshal([]byte(c.kubernetesNodeTolerations), &c.KubernetesNodeTolerations)
	}
	if c.kubernetesAdditionalJobVolumes != "" {
		c.kubernetesAdditionalJobVolumesUnmarshalError = json.Unmarshal([]byte(c.kubernetesAdditionalJobVolumes), &c.KubernetesAdditionalJobVolumes)
	}
	if c.kubernetesAdditionalJobVolumeMounts != "" {
		c.kubernetesAdditionalJobVolumeMountsUnmarshalError = json.Unmarshal([]byte(c.kubernetesAdditionalJobVolumeMounts), &c.KubernetesAdditionalJobVolumeMounts)
	}
	if c.kubernetesJobAnnotations != "" {
		c.kubernetesJobAnnotationsUnmarshalError = json.Unmarshal([]byte(c.kubernetesJobAnnotations), &c.KubernetesJobAnnotations)
	}
	if c.kubernetesJobPodAnnotations != "" {
		c.kubernetesJobPodAnnotationsUnmarshalError = json.Unmarshal([]byte(c.kubernetesJobPodAnnotations), &c.KubernetesJobPodAnnotations)
	}

	if c.KubernetesConfigPath == "" {
		c.KubernetesConfigPath = getKubeConfigPath()
	}

	hn := hostname.Get()
	// Be unique but also descriptive.
	c.WorkerHostname = hn + "-" + uuid.New().String()
}

func getKubeConfigPath() string {
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}

func (c *Config) Validate() error {
	if c.QueueName == "" && c.QueueNamesStr == "" {
		c.AddError(errors.New("neither EXECUTOR_QUEUE_NAME or EXECUTOR_QUEUE_NAMES is set"))
	} else if c.QueueName != "" && c.QueueNamesStr != "" {
		c.AddError(errors.New("both EXECUTOR_QUEUE_NAME and EXECUTOR_QUEUE_NAMES are set"))
	} else if c.QueueName != "" && !slices.Contains(types.ValidQueueNames, c.QueueName) {
		c.AddError(errors.Newf("EXECUTOR_QUEUE_NAME must be set to one of '%v'", strings.Join(types.ValidQueueNames, ", ")))
	} else {
		for _, queueName := range c.QueueNames {
			if !slices.Contains(types.ValidQueueNames, queueName) {
				c.AddError(errors.Newf("EXECUTOR_QUEUE_NAMES contains invalid queue name '%s', valid names are '%v' and should be comma-separated",
					queueName,
					strings.Join(types.ValidQueueNames, ", "),
				))
			}
		}
	}

	u, err := url.Parse(c.FrontendURL)
	if err != nil {
		c.AddError(errors.Wrap(err, "failed to parse EXECUTOR_FRONTEND_URL"))
	}
	if u.Scheme == "" || u.Host == "" {
		c.AddError(errors.New("EXECUTOR_FRONTEND_URL must be in the format scheme://host (and optionally :port)"))
	}
	if u.Hostname() == "host.docker.internal" && !c.DockerAddHostGateway {
		c.AddError(errors.New("Making the executor talk to host.docker.internal but not allowing host gateway access using EXECUTOR_DOCKER_ADD_HOST_GATEWAY can cause connectivity problems"))
	}

	if c.dockerAuthConfigUnmarshalError != nil {
		c.AddError(errors.Wrap(c.dockerAuthConfigUnmarshalError, "invalid EXECUTOR_DOCKER_AUTH_CONFIG, failed to parse"))
	}

	if c.kubernetesNodeRequiredAffinityMatchExpressionsUnmarshalError != nil {
		c.AddError(errors.Wrap(c.kubernetesNodeRequiredAffinityMatchExpressionsUnmarshalError, "invalid EXECUTOR_KUBERNETES_NODE_REQUIRED_AFFINITY_MATCH_EXPRESSIONS, failed to parse"))
	}

	if c.kubernetesNodeRequiredAffinityMatchFieldsUnmarshalError != nil {
		c.AddError(errors.Wrap(c.kubernetesNodeRequiredAffinityMatchFieldsUnmarshalError, "invalid EXECUTOR_KUBERNETES_NODE_REQUIRED_AFFINITY_MATCH_FIELDS, failed to parse"))
	}

	if c.kubernetesPodAffinityUnmarshalError != nil {
		c.AddError(errors.Wrap(c.kubernetesPodAffinityUnmarshalError, "invalid EXECUTOR_KUBERNETES_POD_AFFINITY, failed to parse"))
	}

	if c.kubernetesAdditionalJobVolumeMountsUnmarshalError != nil {
		c.AddError(errors.Wrap(c.kubernetesAdditionalJobVolumeMountsUnmarshalError, "invalid KUBERNETES_JOB_MOUNTS, failed to parse"))
	}

	if c.kubernetesAdditionalJobVolumesUnmarshalError != nil {
		c.AddError(errors.Wrap(c.kubernetesAdditionalJobVolumesUnmarshalError, "invalid KUBERNETES_JOB_VOLUMES, failed to parse"))
	}

	if c.kubernetesJobAnnotationsUnmarshalError != nil {
		c.AddError(errors.Wrap(c.kubernetesJobAnnotationsUnmarshalError, "invalid KUBERNETES_JOB_ANNOTATIONS, failed to parse"))
	}

	if c.kubernetesJobPodAnnotationsUnmarshalError != nil {
		c.AddError(errors.Wrap(c.kubernetesJobPodAnnotationsUnmarshalError, "invalid KUBERNETES_JOB_POD_ANNOTATIONS, failed to parse"))
	}

	if c.KubernetesJobVolumeType != "emptyDir" && c.KubernetesJobVolumeType != "pvc" {
		c.AddError(errors.New("invalid KUBERNETES_JOB_VOLUME_TYPE, valid values are 'emptyDir' and 'pvc'"))
	}

	if len(c.KubernetesPodAffinity) > 0 {
		for _, podAffinity := range c.KubernetesPodAffinity {
			if len(podAffinity.TopologyKey) == 0 {
				c.AddError(errors.New("EXECUTOR_KUBERNETES_POD_AFFINITY must contain a topologyKey"))
			}
		}
	}

	if c.kubernetesPodAntiAffinityUnmarshalError != nil {
		c.AddError(errors.Wrap(c.kubernetesPodAntiAffinityUnmarshalError, "invalid EXECUTOR_KUBERNETES_POD_ANTI_AFFINITY, failed to parse"))
	}

	if len(c.KubernetesPodAntiAffinity) > 0 {
		for _, podAntiAffinity := range c.KubernetesPodAntiAffinity {
			if len(podAntiAffinity.TopologyKey) == 0 {
				c.AddError(errors.New("EXECUTOR_KUBERNETES_POD_ANTI_AFFINITY must contain a topologyKey"))
			}
		}
	}

	if c.kubernetesNodeTolerationsUnmarshalError != nil {
		c.AddError(errors.Wrap(c.kubernetesNodeTolerationsUnmarshalError, "invalid EXECUTOR_KUBERNETES_NODE_TOLERATIONS, failed to parse"))
	}

	if c.UseFirecracker {
		// Validate that firecracker can work on this host.
		if runtime.GOOS != "linux" {
			c.AddError(errors.New("EXECUTOR_USE_FIRECRACKER is only supported on linux hosts."))
		}
		if runtime.GOARCH != "amd64" {
			c.AddError(errors.New("EXECUTOR_USE_FIRECRACKER is only supported on amd64 hosts."))
		}

		// Required by Firecracker: The vCPU number can only be 1 or an even number when hyperthreading is enabled.
		if c.JobNumCPUs != 1 && c.JobNumCPUs%2 != 0 {
			c.AddError(errors.New("EXECUTOR_JOB_NUM_CPUS must be 1 or an even number"))
		}

		// Make sure disk space is a valid datasize string.
		_, err := datasize.ParseString(c.FirecrackerDiskSpace)
		if err != nil {
			c.AddError(errors.Wrapf(err, "invalid disk size provided for EXECUTOR_FIRECRACKER_DISK_SPACE: %q", c.FirecrackerDiskSpace))
		}
	}

	if len(c.KubernetesNodeSelector) > 0 {
		nodeSelectorValues := strings.Split(c.KubernetesNodeSelector, ",")
		for _, value := range nodeSelectorValues {
			parts := strings.Split(value, "=")
			if len(parts) != 2 {
				c.AddError(errors.New("EXECUTOR_KUBERNETES_NODE_SELECTOR must be a comma separated list of key=value pairs"))
			}
		}
	}

	return c.BaseConfig.Validate()
}
