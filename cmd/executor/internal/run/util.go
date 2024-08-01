package run

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/sourcegraph/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/apiclient"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/apiclient/queue"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/config"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/util"
	apiworker "github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/runner"
	executorutil "github.com/sourcegraph/sourcegraph/internal/executor/util"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func newQueueTelemetryOptions(ctx context.Context, runner util.CmdRunner, useFirecracker bool, logger log.Logger) queue.TelemetryOptions {
	t := queue.TelemetryOptions{
		OS:              runtime.GOOS,
		Architecture:    runtime.GOARCH,
		ExecutorVersion: version.Version(),
	}

	var err error

	t.GitVersion, err = util.GetGitVersion(ctx, runner)
	if err != nil {
		logger.Error("Failed to get git version", log.Error(err))
	}

	if !config.IsKubernetes() {
		t.SrcCliVersion, err = util.GetSrcVersion(ctx, runner)
		if err != nil {
			logger.Error("Failed to get src-cli version", log.Error(err))
		}

		t.DockerVersion, err = util.GetDockerVersion(ctx, runner)
		if err != nil {
			logger.Error("Failed to get docker version", log.Error(err))
		}
	}

	if useFirecracker {
		t.IgniteVersion, err = util.GetIgniteVersion(ctx, runner)
		if err != nil {
			logger.Error("Failed to get ignite version", log.Error(err))
		}
	}

	return t
}

func apiWorkerOptions(c *config.Config, queueTelemetryOptions queue.TelemetryOptions) apiworker.Options {
	return apiworker.Options{
		VMPrefix:      c.VMPrefix,
		QueueName:     c.QueueName,
		QueueNames:    c.QueueNames,
		WorkerOptions: workerOptions(c),
		RunnerOptions: runner.Options{
			DockerOptions:      dockerOptions(c),
			FirecrackerOptions: firecrackerOptions(c),
			KubernetesOptions:  kubernetesOptions(c),
		},
		GitServicePath: "/.executors/git",
		QueueOptions:   queueOptions(c, queueTelemetryOptions),
		FilesOptions:   filesOptions(c),
		RedactedValues: map[string]string{
			// 🚨 SECURITY: Catch uses of the shared frontend token used to clone
			// git repositories that make it into commands or stdout/stderr streams.
			c.FrontendAuthorizationToken: "SECRET_REMOVED",
		},

		NodeExporterEndpoint:               c.NodeExporterURL,
		DockerRegistryNodeExporterEndpoint: c.DockerRegistryNodeExporterURL,
		EnableJobAuditLogging:              c.EnableJobAuditLogging,
	}
}

func workerOptions(c *config.Config) workerutil.WorkerOptions {
	queueStr := executorutil.FormatQueueNamesForMetrics(c.QueueName, c.QueueNames)
	return workerutil.WorkerOptions{
		Name:                 fmt.Sprintf("executor_%s_worker", queueStr),
		NumHandlers:          c.MaximumNumJobs,
		Interval:             c.QueuePollInterval,
		HeartbeatInterval:    5 * time.Second,
		Metrics:              makeWorkerMetrics(queueStr),
		NumTotalJobs:         c.NumTotalJobs,
		MaxActiveTime:        c.MaxActiveTime,
		WorkerHostname:       c.WorkerHostname,
		MaximumRuntimePerJob: c.MaximumRuntimePerJob,
	}
}

func dockerOptions(c *config.Config) command.DockerOptions {
	return command.DockerOptions{
		DockerAuthConfig: c.DockerAuthConfig,
		AddHostGateway:   c.DockerAddHostGateway,
		Resources:        resourceOptions(c),
	}
}

func firecrackerOptions(c *config.Config) runner.FirecrackerOptions {
	var dockerMirrors []string
	if len(c.DockerRegistryMirrorURL) > 0 {
		dockerMirrors = strings.Split(c.DockerRegistryMirrorURL, ",")
	}
	return runner.FirecrackerOptions{
		Enabled:                  c.UseFirecracker,
		Image:                    c.FirecrackerImage,
		KernelImage:              c.FirecrackerKernelImage,
		SandboxImage:             c.FirecrackerSandboxImage,
		VMStartupScriptPath:      c.VMStartupScriptPath,
		DockerRegistryMirrorURLs: dockerMirrors,
		DockerOptions:            dockerOptions(c),
		KeepWorkspaces:           c.KeepWorkspaces,
	}
}

func resourceOptions(c *config.Config) command.ResourceOptions {
	return command.ResourceOptions{
		NumCPUs:             c.JobNumCPUs,
		Memory:              c.JobMemory,
		DiskSpace:           c.FirecrackerDiskSpace,
		DockerHostMountPath: c.DockerHostMountPath,
		MaxIngressBandwidth: c.FirecrackerBandwidthIngress,
		MaxEgressBandwidth:  c.FirecrackerBandwidthEgress,
	}
}

func queueOptions(c *config.Config, telemetryOptions queue.TelemetryOptions) queue.Options {
	return queue.Options{
		ExecutorName:      c.WorkerHostname,
		QueueName:         c.QueueName,
		QueueNames:        c.QueueNames,
		BaseClientOptions: baseClientOptions(c, "/.executors/queue"),
		TelemetryOptions:  telemetryOptions,
		ResourceOptions: queue.ResourceOptions{
			NumCPUs:   c.JobNumCPUs,
			Memory:    c.JobMemory,
			DiskSpace: c.FirecrackerDiskSpace,
		},
	}
}

func filesOptions(c *config.Config) apiclient.BaseClientOptions {
	return apiclient.BaseClientOptions{
		ExecutorName:    c.WorkerHostname,
		EndpointOptions: endpointOptions(c, "/.executors/files"),
	}
}

func testOptions(c *config.Config) apiclient.BaseClientOptions {
	return apiclient.BaseClientOptions{
		EndpointOptions: endpointOptions(c, "/.executors/test"),
	}
}

func baseClientOptions(c *config.Config, pathPrefix string) apiclient.BaseClientOptions {
	return apiclient.BaseClientOptions{
		ExecutorName:    c.WorkerHostname,
		EndpointOptions: endpointOptions(c, pathPrefix),
	}
}

func endpointOptions(c *config.Config, pathPrefix string) apiclient.EndpointOptions {
	return apiclient.EndpointOptions{
		URL:        c.FrontendURL,
		PathPrefix: pathPrefix,
		Token:      c.FrontendAuthorizationToken,
	}
}

func kubernetesOptions(c *config.Config) runner.KubernetesOptions {
	var nodeSelector map[string]string
	if len(c.KubernetesNodeSelector) > 0 {
		nodeSelectorValues := strings.Split(c.KubernetesNodeSelector, ",")
		nodeSelector = make(map[string]string, len(nodeSelectorValues))
		for _, value := range nodeSelectorValues {
			parts := strings.Split(value, "=")
			if len(parts) == 2 {
				nodeSelector[parts[0]] = parts[1]
			}
		}
	}

	resourceLimit := command.KubernetesResource{Memory: resource.MustParse(c.KubernetesResourceLimitMemory)}
	if c.KubernetesResourceLimitCPU != "" {
		resourceLimit.CPU = resource.MustParse(c.KubernetesResourceLimitCPU)
	}

	resourceRequest := command.KubernetesResource{Memory: resource.MustParse(c.KubernetesResourceRequestMemory)}
	if c.KubernetesResourceRequestCPU != "" {
		resourceRequest.CPU = resource.MustParse(c.KubernetesResourceRequestCPU)
	}
	var runAsUser *int64
	if c.KubernetesSecurityContextRunAsUser > 0 {
		runAsUser = pointers.Ptr(int64(c.KubernetesSecurityContextRunAsUser))
	}
	var runAsGroup *int64
	if c.KubernetesSecurityContextRunAsGroup > 0 {
		runAsGroup = pointers.Ptr(int64(c.KubernetesSecurityContextRunAsGroup))
	}
	fsGroup := pointers.Ptr(int64(c.KubernetesSecurityContextFSGroup))
	deadline := pointers.Ptr(int64(c.KubernetesJobDeadline))

	var imagePullSecrets []corev1.LocalObjectReference
	if c.KubernetesImagePullSecrets != "" {
		secrets := strings.Split(c.KubernetesImagePullSecrets, ",")
		for _, secret := range secrets {
			imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: secret})
		}
	}

	return runner.KubernetesOptions{
		Enabled:    config.IsKubernetes(),
		ConfigPath: c.KubernetesConfigPath,
		ContainerOptions: command.KubernetesContainerOptions{
			CloneOptions: command.KubernetesCloneOptions{
				ExecutorName:   c.WorkerHostname,
				EndpointURL:    c.FrontendURL,
				GitServicePath: "/.executors/git",
			},
			NodeName:         c.KubernetesNodeName,
			NodeSelector:     nodeSelector,
			JobAnnotations:   c.KubernetesJobAnnotations,
			PodAnnotations:   c.KubernetesJobPodAnnotations,
			ImagePullSecrets: imagePullSecrets,
			RequiredNodeAffinity: command.KubernetesNodeAffinity{
				MatchExpressions: c.KubernetesNodeRequiredAffinityMatchExpressions,
				MatchFields:      c.KubernetesNodeRequiredAffinityMatchFields,
			},
			PodAffinity:           c.KubernetesPodAffinity,
			PodAntiAffinity:       c.KubernetesPodAntiAffinity,
			Tolerations:           c.KubernetesNodeTolerations,
			Namespace:             c.KubernetesNamespace,
			PersistenceVolumeName: c.KubernetesPersistenceVolumeName,
			ResourceLimit:         resourceLimit,
			ResourceRequest:       resourceRequest,
			Deadline:              deadline,
			KeepJobs:              c.KubernetesKeepJobs,
			SecurityContext: command.KubernetesSecurityContext{
				RunAsUser:  runAsUser,
				RunAsGroup: runAsGroup,
				FSGroup:    fsGroup,
			},
			StepImage: c.KubernetesJobStepImage,
			GitCACert: c.KubernetesGitCACert,
			JobVolume: command.KubernetesJobVolume{
				Type:    command.KubernetesVolumeType(c.KubernetesJobVolumeType),
				Size:    resource.MustParse(c.KubernetesJobVolumeSize),
				Volumes: c.KubernetesAdditionalJobVolumes,
				Mounts:  c.KubernetesAdditionalJobVolumeMounts,
			},
		},
	}
}

func makeWorkerMetrics(queueName string) workerutil.WorkerObservability {
	observationCtx := observation.NewContext(log.Scoped("executor_processor"))

	return workerutil.NewMetrics(observationCtx, "executor_processor", workerutil.WithSampler(func(job workerutil.Record) bool { return true }),
		// derived from historic data, ideally we will use spare high-res histograms once they're a reality
		// 										 30s 1m	 2.5m 5m   7.5m 10m  15m  20m	30m	  45m	1hr
		workerutil.WithDurationBuckets([]float64{30, 60, 150, 300, 450, 600, 900, 1200, 1800, 2700, 3600}),
		workerutil.WithLabels(map[string]string{
			"queue": queueName,
		}),
	)
}
