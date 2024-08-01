package command

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/errgroup"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	k8swatch "k8s.io/apimachinery/pkg/watch"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/cmdlogger"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/files"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

const (
	// KubernetesExecutorMountPath is the path where the PersistentVolumeClaim is mounted in the Executor Pod.
	KubernetesExecutorMountPath = "/data"
	// KubernetesJobMountPath is the path where the PersistentVolumeClaim is mounted in the Job Pod.
	KubernetesJobMountPath = "/job"
)

const (
	// kubernetesJobVolumeName is the name of the PersistentVolumeClaim that is mounted in the Job Pod.
	kubernetesJobVolumeName = "sg-executor-job-volume"
	// kubernetesExecutorVolumeMountSubPath is the path where the PersistentVolumeClaim is mounted to in the Executor Pod.
	// The trailing slash is required to properly trim the specified path when creating the subpath in the Job Pod.
	kubernetesExecutorVolumeMountSubPath = "/data/"
)

// KubernetesContainerOptions contains options for the Kubernetes Job containers.
type KubernetesContainerOptions struct {
	CloneOptions          KubernetesCloneOptions
	Namespace             string
	JobAnnotations        map[string]string
	PodAnnotations        map[string]string
	NodeName              string
	NodeSelector          map[string]string
	ImagePullSecrets      []corev1.LocalObjectReference
	RequiredNodeAffinity  KubernetesNodeAffinity
	PodAffinity           []corev1.PodAffinityTerm
	PodAntiAffinity       []corev1.PodAffinityTerm
	Tolerations           []corev1.Toleration
	PersistenceVolumeName string
	ResourceLimit         KubernetesResource
	ResourceRequest       KubernetesResource
	Deadline              *int64
	KeepJobs              bool
	SecurityContext       KubernetesSecurityContext
	StepImage             string
	GitCACert             string
	JobVolume             KubernetesJobVolume
}

// KubernetesCloneOptions contains options for cloning a Git repository.
type KubernetesCloneOptions struct {
	ExecutorName   string
	EndpointURL    string
	GitServicePath string
}

// KubernetesNodeAffinity contains the Kubernetes node affinity for a Job.
type KubernetesNodeAffinity struct {
	MatchExpressions []corev1.NodeSelectorRequirement
	MatchFields      []corev1.NodeSelectorRequirement
}

// KubernetesResource contains the CPU and memory resources for a Kubernetes Job.
type KubernetesResource struct {
	CPU    resource.Quantity
	Memory resource.Quantity
}

// KubernetesSecurityContext contains the security context options for a Kubernetes Job.
type KubernetesSecurityContext struct {
	RunAsUser  *int64
	RunAsGroup *int64
	FSGroup    *int64
}

type KubernetesJobVolume struct {
	Type    KubernetesVolumeType
	Size    resource.Quantity
	Volumes []corev1.Volume
	Mounts  []corev1.VolumeMount
}

type KubernetesVolumeType string

const (
	KubernetesVolumeTypeEmptyDir KubernetesVolumeType = "emptyDir"
	KubernetesVolumeTypePVC      KubernetesVolumeType = "pvc"
)

// KubernetesCommand interacts with the Kubernetes API.
type KubernetesCommand struct {
	Logger     log.Logger
	Clientset  kubernetes.Interface
	Operations *Operations
}

// CreateJob creates a Kubernetes job with the given name and command.
func (c *KubernetesCommand) CreateJob(ctx context.Context, namespace string, job *batchv1.Job) (createdJob *batchv1.Job, err error) {
	ctx, _, endObservation := c.Operations.KubernetesCreateJob.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("name", job.Name),
	}})
	defer endObservation(1, observation.Args{})

	return c.Clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
}

// DeleteJob deletes the Kubernetes job with the given name.
func (c *KubernetesCommand) DeleteJob(ctx context.Context, namespace string, jobName string) (err error) {
	ctx, _, endObservation := c.Operations.KubernetesDeleteJob.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("name", jobName),
	}})
	defer endObservation(1, observation.Args{})

	return c.Clientset.BatchV1().Jobs(namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
}

// CreateSecrets creates Kubernetes secrets with the given name and data.
func (c *KubernetesCommand) CreateSecrets(ctx context.Context, namespace string, name string, secrets map[string]string) (JobSecret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: secrets,
	}
	if _, err := c.Clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		return JobSecret{}, err
	}
	keys := make([]string, len(secrets))
	i := 0
	for key := range secrets {
		keys[i] = key
		i++
	}
	return JobSecret{Name: name, Keys: keys}, nil
}

// DeleteSecret deletes the Kubernetes secret with the given name.
func (c *KubernetesCommand) DeleteSecret(ctx context.Context, namespace string, name string) error {
	return c.Clientset.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
}

// CreateJobPVC creates a Kubernetes PersistentVolumeClaim with the given name and size.
func (c *KubernetesCommand) CreateJobPVC(ctx context.Context, namespace string, name string, size resource.Quantity) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: size},
			},
		},
	}
	if _, err := c.Clientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}

// DeleteJobPVC deletes the Kubernetes PersistentVolumeClaim with the given name.
func (c *KubernetesCommand) DeleteJobPVC(ctx context.Context, namespace string, name string) error {
	return c.Clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
}

var propagationPolicy = metav1.DeletePropagationBackground

// WaitForPodToSucceed waits for the pod with the given job label to succeed.
func (c *KubernetesCommand) WaitForPodToSucceed(ctx context.Context, logger cmdlogger.Logger, namespace string, jobName string, specs []Spec) (p *corev1.Pod, err error) {
	ctx, _, endObservation := c.Operations.KubernetesWaitForPodToSucceed.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("jobName", jobName),
	}})
	defer endObservation(1, observation.Args{})

	watch, err := c.Clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{Watch: true, LabelSelector: "job-name=" + jobName})
	if err != nil {
		return nil, errors.Wrap(err, "watching pod")
	}
	defer watch.Stop()

	containerLoggers := make(map[string]containerLogger)
	defer func() {
		for _, loggers := range containerLoggers {
			loggers.logEntry.Close()
		}
	}()

	// No need to add a timer. If the job exceeds the deadline, it will fail.
	for event := range watch.ResultChan() {
		// Will be *corev1.Pod in all cases except for an error, which is *metav1.Status.
		if event.Type == k8swatch.Error {
			if status, ok := event.Object.(*metav1.Status); ok {
				c.Logger.Error("Watch error",
					log.String("status", status.Status),
					log.String("message", status.Message),
					log.String("reason", string(status.Reason)),
					log.Int32("code", status.Code),
				)
			} else {
				c.Logger.Error("Unexpected watch error object", log.String("object", fmt.Sprintf("%T", event.Object)))
			}
			// If we get an event for something other than a pod, log it for now and try again. We don't have enough
			// information to know if this is a problem or not. We have seen this happen in the wild, but hard to
			// replicate.
			continue
		}
		// We _should_ have a pod here, but just in case, ensure the cast succeeds.
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			// If we get an event for something other than a pod, log it for now and try again. We don't have enough
			// information to know if this is a problem or not. We have seen this happen in the wild, but hard to
			// replicate.
			c.Logger.Error(
				"Unexpected watch object",
				log.String("type", string(event.Type)),
				log.String("object", fmt.Sprintf("%T", event.Object)),
			)
			continue
		}
		c.Logger.Debug(
			"Watching pod",
			log.String("name", pod.Name),
			log.String("phase", string(pod.Status.Phase)),
			log.Time("creationTimestamp", pod.CreationTimestamp.Time),
			kubernetesTimep("deletionTimestamp", pod.DeletionTimestamp),
			kubernetesConditions("conditions", pod.Status.Conditions),
		)
		// If there are init containers, stream their logs.
		if len(pod.Status.InitContainerStatuses) > 0 {
			err = c.handleContainers(ctx, logger, namespace, pod, pod.Status.InitContainerStatuses, containerLoggers, specs)
			if err != nil {
				return pod, err
			}
		}
		// If there are containers, stream their logs.
		if len(pod.Status.ContainerStatuses) > 0 {
			err = c.handleContainers(ctx, logger, namespace, pod, pod.Status.ContainerStatuses, containerLoggers, specs)
			if err != nil {
				return pod, err
			}
		}
		switch pod.Status.Phase {
		case corev1.PodFailed:
			return pod, ErrKubernetesPodFailed
		case corev1.PodSucceeded:
			return pod, nil
		case corev1.PodPending:
			if pod.DeletionTimestamp != nil {
				return nil, ErrKubernetesPodNotScheduled
			}
		}
	}
	return nil, errors.New("unexpected end of watch")
}

func kubernetesTimep(key string, time *metav1.Time) log.Field {
	if time == nil {
		return log.Timep(key, nil)
	}
	return log.Time(key, time.Time)
}

func kubernetesConditions(key string, conditions []corev1.PodCondition) log.Field {
	if len(conditions) == 0 {
		return log.Stringp(key, nil)
	}
	fields := make([]log.Field, len(conditions))
	for i, condition := range conditions {
		fields[i] = log.Object(
			fmt.Sprintf("condition[%d]", i),
			log.String("type", string(condition.Type)),
			log.String("status", string(condition.Status)),
			log.String("reason", condition.Reason),
			log.String("message", condition.Message),
		)
	}
	if len(fields) == 0 {
		return log.Stringp(key, nil)
	}
	return log.Object(
		key,
		fields...,
	)
}

func (c *KubernetesCommand) handleContainers(
	ctx context.Context,
	logger cmdlogger.Logger,
	namespace string,
	pod *corev1.Pod,
	containerStatus []corev1.ContainerStatus,
	containerLoggers map[string]containerLogger,
	specs []Spec,
) error {
	for _, status := range containerStatus {
		// If the container is waiting, it hasn't started yet, so skip it.
		if status.State.Waiting != nil {
			continue
		}
		// If the container is not waiting, then it has either started or completed. Either way, we will want to
		// create the logEntry if it doesn't exist.
		l, ok := containerLoggers[status.Name]
		if !ok {
			// Potentially the container completed too quickly, so we may not have started the log entry yet.
			key, command := getLogMetadata(status.Name, specs)
			containerLoggers[status.Name] = containerLogger{logEntry: logger.LogEntry(key, command)}
			l = containerLoggers[status.Name]
		}
		if status.State.Terminated != nil && !l.completed {
			// Read the logs once the container has terminated. This gives us access to the exit code.
			if err := c.readLogs(ctx, namespace, pod, status.Name, containerStatus, l.logEntry); err != nil {
				return err
			}
			l.completed = true
			containerLoggers[status.Name] = l
		}
	}
	return nil
}

func getLogMetadata(key string, specs []Spec) (string, []string) {
	for _, step := range specs {
		if step.Name == key {
			return step.Key, step.Command
		}
	}
	return normalizeKey(key), nil
}

func normalizeKey(key string) string {
	// Since '.' are not allowed in container names, we need to convert the key to have '.' to make our logging
	// happy.
	return strings.ReplaceAll(key, "-", ".")
}

type containerLogger struct {
	logEntry  cmdlogger.LogEntry
	completed bool
}

// readLogs reads the logs of the given pod and writes them to the logger.
func (c *KubernetesCommand) readLogs(
	ctx context.Context,
	namespace string,
	pod *corev1.Pod,
	containerName string,
	containerStatus []corev1.ContainerStatus,
	logEntry cmdlogger.LogEntry,
) (err error) {
	ctx, _, endObservation := c.Operations.KubernetesReadLogs.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("podName", pod.Name),
		attribute.String("containerName", containerName),
	}})
	defer endObservation(1, observation.Args{})

	c.Logger.Debug(
		"Reading logs",
		log.String("podName", pod.Name),
		log.String("containerName", containerName),
	)

	// If the pod just failed to even start, then we can't get logs from it.
	if pod.Status.Phase == corev1.PodFailed && len(containerStatus) == 0 {
		logEntry.Finalize(1)
	} else {
		exitCode := 0
		for _, status := range containerStatus {
			if status.Name == containerName {
				exitCode = int(status.State.Terminated.ExitCode)
				break
			}
		}
		// Ensure we always get the exit code in case an error occurs when reading the logs.
		defer logEntry.Finalize(exitCode)

		req := c.Clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: containerName})
		stream, err := req.Stream(ctx)
		if err != nil {
			return errors.Wrapf(err, "opening log stream for pod %s", pod.Name)
		}

		pipeReaderWaitGroup := readProcessPipe(logEntry, stream)

		select {
		case <-ctx.Done():
		case err = <-watchErrGroup(pipeReaderWaitGroup):
			if err != nil {
				return errors.Wrap(err, "reading process pipes")
			}
		}
	}

	return nil
}

func readProcessPipe(w io.WriteCloser, stdout io.Reader) *errgroup.Group {
	eg := &errgroup.Group{}

	eg.Go(func() error {
		return readIntoBuffer("stdout", w, stdout)
	})

	return eg
}

// ErrKubernetesPodFailed is returned when a Kubernetes pod fails.
var ErrKubernetesPodFailed = errors.New("pod failed")

// ErrKubernetesPodNotScheduled is returned when a Kubernetes pod could not be scheduled and was deleted.
var ErrKubernetesPodNotScheduled = errors.New("deleted by scheduler: pod could not be scheduled")

// RepositoryOptions contains the options for a repository job.
type RepositoryOptions struct {
	JobID               int
	CloneURL            string
	RepositoryDirectory string
	Commit              string
}

// NewKubernetesJob creates a Kubernetes job with the given name, image, volume path, and spec.
func NewKubernetesJob(
	name string,
	specs []Spec,
	workspaceFiles []files.WorkspaceFile,
	secret JobSecret,
	volumeName string,
	repoOptions RepositoryOptions,
	options KubernetesContainerOptions,
) *batchv1.Job {
	affinity := newAffinity(options)

	resourceLimit := newResourceLimit(options)
	resourceRequest := newResourceRequest(options)

	volumes := make([]corev1.Volume, len(options.JobVolume.Volumes)+1)
	switch options.JobVolume.Type {
	case KubernetesVolumeTypePVC:
		volumes[0] = corev1.Volume{
			Name: "job-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: volumeName,
				},
			},
		}
	case KubernetesVolumeTypeEmptyDir:
		fallthrough
	default:
		volumes[0] = corev1.Volume{
			Name: "job-data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: &options.JobVolume.Size,
				},
			},
		}
	}
	for i, volume := range options.JobVolume.Volumes {
		volumes[i+1] = volume
	}

	setupEnvs := make([]corev1.EnvVar, len(secret.Keys))
	for i, key := range secret.Keys {
		setupEnvs[i] = corev1.EnvVar{
			Name: key,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: key,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret.Name,
					},
				},
			},
		}
	}

	repoDir := "."
	if repoOptions.RepositoryDirectory != "" {
		repoDir = repoOptions.RepositoryDirectory
	}

	sslCAInfo := ""
	if options.GitCACert != "" {
		sslCAInfo = fmt.Sprintf("git -C %s config --local http.sslCAInfo %s; ", repoDir, options.GitCACert)
	}

	setupArgs := []string{
		"set -e; " +
			fmt.Sprintf("mkdir -p %s; ", repoDir) +
			fmt.Sprintf("git -C %s init; ", repoDir) +
			sslCAInfo +
			fmt.Sprintf("git -C %s remote add origin %s; ", repoDir, repoOptions.CloneURL) +
			fmt.Sprintf("git -C %s config --local gc.auto 0; ", repoDir) +
			fmt.Sprintf("git -C %s "+
				"-c http.extraHeader=\"Authorization:Bearer $TOKEN\" "+
				"-c http.extraHeader=X-Sourcegraph-Actor-UID:internal "+
				"-c http.extraHeader=X-Sourcegraph-Job-ID:%d "+
				"-c http.extraHeader=X-Sourcegraph-Executor-Name:%s "+
				"-c protocol.version=2 "+
				"fetch --progress --no-recurse-submodules --no-tags --depth=1 origin %s; ", repoDir, repoOptions.JobID, options.CloneOptions.ExecutorName, repoOptions.Commit) +
			fmt.Sprintf("git -C %s checkout --progress --force %s; ", repoDir, repoOptions.Commit) +
			"mkdir -p .sourcegraph-executor; " +
			"echo '" + formatContent(nextIndexScript) + "' > nextIndex.sh; " +
			"chmod +x nextIndex.sh; ",
	}

	for _, file := range workspaceFiles {
		// Get the file path without the ending file name.
		dir := filepath.Dir(file.Path)
		setupArgs[0] += "mkdir -p " + dir + "; echo -E '" + formatContent(string(file.Content)) + "' > " + file.Path + "; chmod +x " + file.Path + "; "
		if !file.ModifiedAt.IsZero() {
			setupArgs[0] += fmt.Sprintf("touch -m -d '%s' %s; ", file.ModifiedAt.Format("200601021504.05"), file.Path)
		}
	}

	stepInitContainers := make([]corev1.Container, len(specs)+1)
	mounts := make([]corev1.VolumeMount, len(options.JobVolume.Mounts)+1)
	mounts[0] = corev1.VolumeMount{
		Name:      "job-data",
		MountPath: KubernetesJobMountPath,
	}
	for i, mount := range options.JobVolume.Mounts {
		mounts[i+1] = mount
	}

	stepInitContainers[0] = corev1.Container{
		Name:            "setup-workspace",
		Image:           options.StepImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"sh", "-c"},
		Args:            setupArgs,
		Env:             setupEnvs,
		WorkingDir:      KubernetesJobMountPath,
		Resources: corev1.ResourceRequirements{
			Limits:   resourceLimit,
			Requests: resourceRequest,
		},
		VolumeMounts: mounts,
	}

	for stepIndex, step := range specs {
		jobEnvs := newEnvVars(step.Env)
		// Single job does not need to add the git directory as safe since the user is the same across all containers.
		// This is a work around until we have a more elegant solution for dealing with the multi-job and different users.
		// e.g. Executor is run as sourcegraph user and batcheshelper is run as root.
		jobEnvs = append(jobEnvs, corev1.EnvVar{
			Name:  "EXECUTOR_ADD_SAFE",
			Value: "false",
		})

		nextIndexCommand := fmt.Sprintf("if [ \"$(%s /job/skip.json %s)\" != \"skip\" ]; then ", filepath.Join(KubernetesJobMountPath, "nextIndex.sh"), step.Key)
		stepInitContainers[stepIndex+1] = corev1.Container{
			Name:            step.Name,
			Image:           step.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"sh", "-c"},
			Args: []string{
				nextIndexCommand +
					fmt.Sprintf("%s fi", strings.Join(step.Command, "; ")+"; "),
			},
			Env:        jobEnvs,
			WorkingDir: filepath.Join(KubernetesJobMountPath, step.Dir),
			Resources: corev1.ResourceRequirements{
				Limits:   resourceLimit,
				Requests: resourceRequest,
			},
			VolumeMounts: mounts,
		}
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: options.JobAnnotations,
		},
		Spec: batchv1.JobSpec{
			// Prevent K8s from retrying. This will lead to the retried jobs always failing as the workspace will get
			// cleaned up from the first failure.
			BackoffLimit: pointers.Ptr[int32](0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: options.PodAnnotations,
				},
				Spec: corev1.PodSpec{
					NodeName:              options.NodeName,
					NodeSelector:          options.NodeSelector,
					ImagePullSecrets:      options.ImagePullSecrets,
					Affinity:              affinity,
					RestartPolicy:         corev1.RestartPolicyNever,
					Tolerations:           options.Tolerations,
					ActiveDeadlineSeconds: options.Deadline,
					InitContainers:        stepInitContainers,
					Containers: []corev1.Container{
						{
							Name:            "main",
							Image:           options.StepImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"sh", "-c"},
							Args: []string{
								"echo 'complete'",
							},
							WorkingDir: KubernetesJobMountPath,
							Resources: corev1.ResourceRequirements{
								Limits:   resourceLimit,
								Requests: resourceRequest,
							},
							VolumeMounts: mounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}

func newEnvVars(envs []string) []corev1.EnvVar {
	jobEnvs := make([]corev1.EnvVar, len(envs))
	for j, env := range envs {
		parts := strings.SplitN(env, "=", 2)
		jobEnvs[j] = corev1.EnvVar{
			Name:  parts[0],
			Value: parts[1],
		}
	}
	return jobEnvs
}

func newAffinity(options KubernetesContainerOptions) *corev1.Affinity {
	var affinity *corev1.Affinity
	if len(options.RequiredNodeAffinity.MatchExpressions) > 0 || len(options.RequiredNodeAffinity.MatchFields) > 0 {
		affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: options.RequiredNodeAffinity.MatchExpressions,
							MatchFields:      options.RequiredNodeAffinity.MatchFields,
						},
					},
				},
			},
			PodAffinity: &corev1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: nil,
						TopologyKey:   "",
					},
				},
			},
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: nil,
						TopologyKey:   "",
					},
				},
			},
		}
	}
	if len(options.PodAffinity) > 0 {
		if affinity == nil {
			affinity = &corev1.Affinity{}
		}
		affinity.PodAffinity = &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: options.PodAffinity,
		}
	}
	if len(options.PodAntiAffinity) > 0 {
		if affinity == nil {
			affinity = &corev1.Affinity{}
		}
		affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: options.PodAntiAffinity,
		}
	}
	return affinity
}

func newResourceLimit(options KubernetesContainerOptions) corev1.ResourceList {
	resourceLimit := corev1.ResourceList{
		corev1.ResourceMemory: options.ResourceLimit.Memory,
	}
	if !options.ResourceLimit.CPU.IsZero() {
		resourceLimit[corev1.ResourceCPU] = options.ResourceLimit.CPU
	}
	return resourceLimit
}

func newResourceRequest(options KubernetesContainerOptions) corev1.ResourceList {
	resourceRequest := corev1.ResourceList{
		corev1.ResourceMemory: options.ResourceRequest.Memory,
	}
	if !options.ResourceRequest.CPU.IsZero() {
		resourceRequest[corev1.ResourceCPU] = options.ResourceRequest.CPU
	}
	return resourceRequest
}

func formatContent(content string) string {
	// Having single ticks in the content mess things up real quick. Replace ' with '"'"'. This forces ' to be a string.
	return strings.ReplaceAll(content, "'", "'\"'\"'")
}

const nextIndexScript = `#!/bin/sh

file="$1"

if [ ! -f "$file" ]; then
  exit 0
fi

nextStep=$(grep -o '"nextStep":[^,]*' $file | sed 's/"nextStep"://' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e 's/"//g' -e 's/}//g')

if [ "${2%$nextStep}" = "$2" ]; then
  echo "skip"
  exit 0
fi
`

type JobSecret struct {
	Name string
	Keys []string
}
