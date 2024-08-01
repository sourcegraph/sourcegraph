package command_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/files"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestKubernetesCommand_CreateJob(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	cmd := &command.KubernetesCommand{
		Logger:     logtest.Scoped(t),
		Clientset:  clientset,
		Operations: command.NewOperations(observation.TestContextTB(t)),
	}

	job := &batchv1.Job{}

	_, err := cmd.CreateJob(context.Background(), "my-namespace", job)
	require.NoError(t, err)

	require.Len(t, clientset.Actions(), 1)
	require.Equal(t, "create", clientset.Actions()[0].GetVerb())
	require.Equal(t, "jobs", clientset.Actions()[0].GetResource().Resource)
	require.Equal(t, "my-namespace", clientset.Actions()[0].GetNamespace())
}

func TestKubernetesCommand_DeleteJob(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	cmd := &command.KubernetesCommand{
		Logger:     logtest.Scoped(t),
		Clientset:  clientset,
		Operations: command.NewOperations(observation.TestContextTB(t)),
	}

	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "my-job"}}
	_, err := cmd.CreateJob(context.Background(), "my-namespace", job)
	require.NoError(t, err)

	err = cmd.DeleteJob(context.Background(), "my-namespace", "my-job")
	require.NoError(t, err)

	require.Len(t, clientset.Actions(), 2)
	require.Equal(t, "delete", clientset.Actions()[1].GetVerb())
	require.Equal(t, "jobs", clientset.Actions()[1].GetResource().Resource)
	assert.Equal(t, "my-namespace", clientset.Actions()[1].GetNamespace())
	assert.Equal(t, "my-job", clientset.Actions()[1].(k8stesting.DeleteAction).GetName())
}

func TestKubernetesCommand_CreateSecrets(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	cmd := &command.KubernetesCommand{
		Logger:     logtest.Scoped(t),
		Clientset:  clientset,
		Operations: command.NewOperations(observation.TestContextTB(t)),
	}

	secrets := map[string]string{
		"foo": "bar",
		"baz": "qux",
	}
	createSecrets, err := cmd.CreateSecrets(context.Background(), "my-namespace", "my-secret", secrets)
	require.NoError(t, err)

	require.Len(t, clientset.Actions(), 1)
	require.Equal(t, "create", clientset.Actions()[0].GetVerb())
	require.Equal(t, "secrets", clientset.Actions()[0].GetResource().Resource)
	require.Equal(t, "my-namespace", clientset.Actions()[0].GetNamespace())

	assert.Equal(t, "my-secret", createSecrets.Name)
	assert.Len(t, createSecrets.Keys, 2)
	assert.ElementsMatch(t, []string{"foo", "baz"}, createSecrets.Keys)
}

func TestKubernetesCommand_DeleteSecret(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	cmd := &command.KubernetesCommand{
		Logger:     logtest.Scoped(t),
		Clientset:  clientset,
		Operations: command.NewOperations(observation.TestContextTB(t)),
	}

	secrets := map[string]string{
		"foo": "bar",
		"baz": "qux",
	}
	_, err := cmd.CreateSecrets(context.Background(), "my-namespace", "my-secret", secrets)
	require.NoError(t, err)

	err = cmd.DeleteSecret(context.Background(), "my-namespace", "my-secret")
	require.NoError(t, err)

	require.Len(t, clientset.Actions(), 2)
	require.Equal(t, "delete", clientset.Actions()[1].GetVerb())
	require.Equal(t, "secrets", clientset.Actions()[1].GetResource().Resource)
	assert.Equal(t, "my-namespace", clientset.Actions()[1].GetNamespace())
}

func TestKubernetesCommand_CreateJobPVC(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	cmd := &command.KubernetesCommand{
		Logger:     logtest.Scoped(t),
		Clientset:  clientset,
		Operations: command.NewOperations(observation.TestContextTB(t)),
	}

	err := cmd.CreateJobPVC(context.Background(), "my-namespace", "my-pvc", resource.MustParse("1Gi"))
	require.NoError(t, err)

	require.Len(t, clientset.Actions(), 1)
	require.Equal(t, "create", clientset.Actions()[0].GetVerb())
	require.Equal(t, "persistentvolumeclaims", clientset.Actions()[0].GetResource().Resource)
	require.Equal(t, "my-namespace", clientset.Actions()[0].GetNamespace())
}

func TestKubernetesCommand_DeleteJobPVC(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	cmd := &command.KubernetesCommand{
		Logger:     logtest.Scoped(t),
		Clientset:  clientset,
		Operations: command.NewOperations(observation.TestContextTB(t)),
	}

	err := cmd.CreateJobPVC(context.Background(), "my-namespace", "my-pvc", resource.MustParse("1Gi"))
	require.NoError(t, err)

	err = cmd.DeleteJobPVC(context.Background(), "my-namespace", "my-pvc")
	require.NoError(t, err)

	require.Len(t, clientset.Actions(), 2)
	require.Equal(t, "delete", clientset.Actions()[1].GetVerb())
	require.Equal(t, "persistentvolumeclaims", clientset.Actions()[1].GetResource().Resource)
	assert.Equal(t, "my-namespace", clientset.Actions()[1].GetNamespace())
}

func TestKubernetesCommand_WaitForPodToSucceed(t *testing.T) {
	tests := []struct {
		name           string
		specs          []command.Spec
		mockFunc       func(clientset *fake.Clientset)
		mockAssertFunc func(t *testing.T, actions []k8stesting.Action, logger *command.MockLogger)
		expectedErr    error
	}{
		{
			name: "Pod succeeded",
			specs: []command.Spec{
				{
					Key:  "my.container",
					Name: "my-container",
					Command: []string{
						"echo",
						"hello world",
					},
				},
			},
			mockFunc: func(clientset *fake.Clientset) {
				watcher := watch.NewFakeWithChanSize(10, false)
				watcher.Add(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-pod",
						Labels: map[string]string{
							"job-name": "my-job",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name: "my-container",
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode: 0,
									},
								},
							},
						},
					},
				})
				clientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(watcher, nil))
			},
			mockAssertFunc: func(t *testing.T, actions []k8stesting.Action, logger *command.MockLogger) {
				require.Len(t, actions, 2)
				assert.Equal(t, "watch", actions[0].GetVerb())
				assert.Equal(t, "pods", actions[0].GetResource().Resource)
				assert.Equal(t, "job-name=my-job", actions[0].(k8stesting.WatchActionImpl).GetWatchRestrictions().Labels.String())
				assert.Equal(t, "get", actions[1].GetVerb())
				assert.Equal(t, "pods", actions[1].GetResource().Resource)
				assert.Equal(t, "log", actions[1].GetSubresource())

				require.Len(t, logger.LogEntryFunc.History(), 1)
				assert.Equal(t, "my.container", logger.LogEntryFunc.History()[0].Arg0)
				assert.Equal(t, []string{"echo", "hello world"}, logger.LogEntryFunc.History()[0].Arg1)
				logEntry := logger.LogEntryFunc.History()[0].Result0.(*command.MockLogEntry)
				require.Len(t, logEntry.WriteFunc.History(), 1)
				assert.Equal(t, "stdout: fake logs\n", string(logEntry.WriteFunc.History()[0].Arg0))
				require.Len(t, logEntry.FinalizeFunc.History(), 1)
				assert.Equal(t, 0, logEntry.FinalizeFunc.History()[0].Arg0)
				require.Len(t, logEntry.CloseFunc.History(), 1)
			},
		},
		{
			name: "Pod succeeded single job",
			specs: []command.Spec{
				{
					Key:  "setup.0",
					Name: "setup-0",
					Command: []string{
						"echo",
						"hello",
					},
				},
				{
					Key:  "setup.1",
					Name: "setup-1",
					Command: []string{
						"echo",
						"world",
					},
				},
			},
			mockFunc: func(clientset *fake.Clientset) {
				watcher := watch.NewFakeWithChanSize(10, false)
				watcher.Add(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-pod",
						Labels: map[string]string{
							"job-name": "my-job",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
						InitContainerStatuses: []corev1.ContainerStatus{
							{
								Name: "setup.0",
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode: 0,
									},
								},
							},
							{
								Name: "setup.1",
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode: 0,
									},
								},
							},
						},
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name: "my-container",
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode: 0,
									},
								},
							},
						},
					},
				})
				clientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(watcher, nil))
			},
			mockAssertFunc: func(t *testing.T, actions []k8stesting.Action, logger *command.MockLogger) {
				require.Len(t, actions, 4)
				assert.Equal(t, "watch", actions[0].GetVerb())
				assert.Equal(t, "pods", actions[0].GetResource().Resource)
				assert.Equal(t, "job-name=my-job", actions[0].(k8stesting.WatchActionImpl).GetWatchRestrictions().Labels.String())
				assert.Equal(t, "get", actions[1].GetVerb())
				assert.Equal(t, "pods", actions[1].GetResource().Resource)
				assert.Equal(t, "log", actions[1].GetSubresource())
				assert.Equal(t, "get", actions[2].GetVerb())
				assert.Equal(t, "pods", actions[2].GetResource().Resource)
				assert.Equal(t, "log", actions[2].GetSubresource())
				assert.Equal(t, "get", actions[3].GetVerb())
				assert.Equal(t, "pods", actions[3].GetResource().Resource)
				assert.Equal(t, "log", actions[3].GetSubresource())

				require.Len(t, logger.LogEntryFunc.History(), 3)
			},
		},
		{
			name: "Pod failed",
			mockFunc: func(clientset *fake.Clientset) {
				watcher := watch.NewFakeWithChanSize(10, false)
				watcher.Add(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-pod",
						Labels: map[string]string{
							"job-name": "my-job",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodFailed,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name: "my-container",
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode: 1,
									},
								},
							},
						},
					},
				})
				clientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(watcher, nil))
			},
			mockAssertFunc: func(t *testing.T, actions []k8stesting.Action, logger *command.MockLogger) {
				require.Len(t, logger.LogEntryFunc.History(), 1)
				logEntry := logger.LogEntryFunc.History()[0].Result0.(*command.MockLogEntry)
				require.Len(t, logEntry.FinalizeFunc.History(), 1)
			},
			expectedErr: errors.New("pod failed"),
		},
		{
			name: "Error occurred",
			mockFunc: func(clientset *fake.Clientset) {
				clientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(nil, errors.New("failed")))
			},
			expectedErr: errors.New("watching pod: failed"),
		},
		{
			name: "Pod succeeded second try",
			mockFunc: func(clientset *fake.Clientset) {
				watcher := watch.NewFakeWithChanSize(10, false)
				watcher.Add(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-pod",
						Labels: map[string]string{
							"job-name": "my-job",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				})
				watcher.Add(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-pod",
						Labels: map[string]string{
							"job-name": "my-job",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
					},
				})
				clientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(watcher, nil))
			},
			mockAssertFunc: func(t *testing.T, actions []k8stesting.Action, logger *command.MockLogger) {
				require.Len(t, actions, 1)
			},
		},
		{
			name: "Pod deleted by scheduler",
			mockFunc: func(clientset *fake.Clientset) {
				watcher := watch.NewFakeWithChanSize(10, false)
				watcher.Add(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-pod",
						Labels: map[string]string{
							"job-name": "my-job",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
					},
				})
				watcher.Add(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-pod",
						Labels: map[string]string{
							"job-name": "my-job",
						},
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
					},
				})
				clientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(watcher, nil))
			},
			mockAssertFunc: func(t *testing.T, actions []k8stesting.Action, logger *command.MockLogger) {
				require.Len(t, actions, 1)
			},
			expectedErr: errors.New("deleted by scheduler: pod could not be scheduled"),
		},
		{
			name: "Watch Error",
			mockFunc: func(clientset *fake.Clientset) {
				watcher := watch.NewFakeWithChanSize(10, false)
				watcher.Error(&metav1.Status{
					Status:  metav1.StatusFailure,
					Message: "failed",
					Reason:  "InternalError",
					Code:    1,
				})
				watcher.Add(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-pod",
						Labels: map[string]string{
							"job-name": "my-job",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
					},
				})
				clientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(watcher, nil))
			},
			mockAssertFunc: func(t *testing.T, actions []k8stesting.Action, logger *command.MockLogger) {
				require.Len(t, actions, 1)
				assert.Equal(t, "watch", actions[0].GetVerb())
				assert.Equal(t, "pods", actions[0].GetResource().Resource)
				assert.Equal(t, "job-name=my-job", actions[0].(k8stesting.WatchActionImpl).GetWatchRestrictions().Labels.String())
			},
		},
		{
			name: "Unexpected Watch Error",
			mockFunc: func(clientset *fake.Clientset) {
				watcher := watch.NewFakeWithChanSize(10, false)
				watcher.Error(&corev1.Pod{})
				watcher.Add(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-pod",
						Labels: map[string]string{
							"job-name": "my-job",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
					},
				})
				clientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(watcher, nil))
			},
			mockAssertFunc: func(t *testing.T, actions []k8stesting.Action, logger *command.MockLogger) {
				require.Len(t, actions, 1)
				assert.Equal(t, "watch", actions[0].GetVerb())
				assert.Equal(t, "pods", actions[0].GetResource().Resource)
				assert.Equal(t, "job-name=my-job", actions[0].(k8stesting.WatchActionImpl).GetWatchRestrictions().Labels.String())
			},
		},
		{
			name: "Unexpected Watch Object",
			mockFunc: func(clientset *fake.Clientset) {
				watcher := watch.NewFakeWithChanSize(10, false)
				watcher.Add(&metav1.Status{
					Status:  metav1.StatusFailure,
					Message: "failed",
					Reason:  "InternalError",
					Code:    1,
				})
				watcher.Add(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-pod",
						Labels: map[string]string{
							"job-name": "my-job",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
					},
				})
				clientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(watcher, nil))
			},
			mockAssertFunc: func(t *testing.T, actions []k8stesting.Action, logger *command.MockLogger) {
				require.Len(t, actions, 1)
				assert.Equal(t, "watch", actions[0].GetVerb())
				assert.Equal(t, "pods", actions[0].GetResource().Resource)
				assert.Equal(t, "job-name=my-job", actions[0].(k8stesting.WatchActionImpl).GetWatchRestrictions().Labels.String())
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			logger := command.NewMockLogger()
			logger.LogEntryFunc.SetDefaultReturn(command.NewMockLogEntry())

			if test.mockFunc != nil {
				test.mockFunc(clientset)
			}

			cmd := &command.KubernetesCommand{
				Logger:     logtest.Scoped(t),
				Clientset:  clientset,
				Operations: command.NewOperations(observation.TestContextTB(t)),
			}

			pod, err := cmd.WaitForPodToSucceed(
				context.Background(),
				logger,
				"my-namespace",
				"my-job",
				test.specs,
			)
			if test.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.NotNil(t, pod)
			}

			if test.mockAssertFunc != nil {
				test.mockAssertFunc(t, clientset.Actions(), logger)
			}
		})
	}
}

func TestNewKubernetesJob(t *testing.T) {
	err := os.Setenv("KUBERNETES_SERVICE_HOST", "http://localhost")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
	})

	specs := []command.Spec{
		{
			Key:     "my.container.0",
			Name:    "my-container-0",
			Command: []string{"echo", "hello"},
			Env:     []string{"FOO=bar"},
			Dir:     "repository",
			Image:   "my-image:latest",
		},
		{
			Key:     "my.container.1",
			Name:    "my-container-1",
			Command: []string{"echo", "world"},
			Env:     []string{"FOO=baz"},
			Dir:     "repository",
			Image:   "my-image:latest",
		},
	}
	workspaceFiles := []files.WorkspaceFile{
		{
			Path:    "/my/path/script1.sh",
			Content: []byte("echo hello"),
		},
		{
			Path:    "/my/path/script2.sh",
			Content: []byte("echo world"),
		},
	}
	secret := command.JobSecret{
		Name: "my-secret",
		Keys: []string{"TOKEN"},
	}
	repoOptions := command.RepositoryOptions{
		JobID:               42,
		CloneURL:            "http://my-frontend/.executor/git/my-repo",
		RepositoryDirectory: "repository",
		Commit:              "deadbeef",
	}
	options := command.KubernetesContainerOptions{
		CloneOptions: command.KubernetesCloneOptions{
			ExecutorName: "my-executor",
		},
		Namespace:      "default",
		NodeName:       "my-node",
		JobAnnotations: map[string]string{"foo": "bar"},
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "my-secret"},
		},
		PersistenceVolumeName: "my-pvc",
		ResourceLimit: command.KubernetesResource{
			CPU:    resource.MustParse("10"),
			Memory: resource.MustParse("10Gi"),
		},
		ResourceRequest: command.KubernetesResource{
			CPU:    resource.MustParse("1"),
			Memory: resource.MustParse("1Gi"),
		},
		SecurityContext: command.KubernetesSecurityContext{
			FSGroup: pointer.Int64(1000),
		},
		StepImage: "step-image:latest",
	}
	job := command.NewKubernetesJob(
		"my-job",
		specs,
		workspaceFiles,
		secret,
		"my-volume",
		repoOptions,
		options,
	)

	assert.Equal(t, "my-job", job.Name)
	assert.Equal(t, map[string]string{"foo": "bar"}, job.Annotations)
	assert.Equal(t, int32(0), *job.Spec.BackoffLimit)

	assert.Equal(t, "my-node", job.Spec.Template.Spec.NodeName)
	assert.Equal(t, corev1.RestartPolicyNever, job.Spec.Template.Spec.RestartPolicy)

	require.Len(t, job.Spec.Template.Spec.InitContainers, 3)
	assert.Equal(t, "my-secret", job.Spec.Template.Spec.ImagePullSecrets[0].Name)

	assert.Equal(t, "setup-workspace", job.Spec.Template.Spec.InitContainers[0].Name)
	assert.Equal(t, "step-image:latest", job.Spec.Template.Spec.InitContainers[0].Image)
	assert.Equal(t, "/job", job.Spec.Template.Spec.InitContainers[0].WorkingDir)
	require.Len(t, job.Spec.Template.Spec.InitContainers[0].Command, 2)
	assert.Equal(t, []string{"sh", "-c"}, job.Spec.Template.Spec.InitContainers[0].Command)
	require.Len(t, job.Spec.Template.Spec.InitContainers[0].Args, 1)
	assert.Equal(
		t,
		"set -e; "+
			"mkdir -p repository; "+
			"git -C repository init; "+
			"git -C repository remote add origin http://my-frontend/.executor/git/my-repo; "+
			"git -C repository config --local gc.auto 0; "+
			"git -C repository "+
			"-c http.extraHeader=\"Authorization:Bearer $TOKEN\" "+
			"-c http.extraHeader=X-Sourcegraph-Actor-UID:internal "+
			"-c http.extraHeader=X-Sourcegraph-Job-ID:42 "+
			"-c http.extraHeader=X-Sourcegraph-Executor-Name:my-executor "+
			"-c protocol.version=2 fetch --progress --no-recurse-submodules --no-tags --depth=1 origin deadbeef; "+
			"git -C repository checkout --progress --force deadbeef; "+
			"mkdir -p .sourcegraph-executor; "+
			"echo '#!/bin/sh\n\nfile=\"$1\"\n\nif [ ! -f \"$file\" ]; then\n  exit 0\nfi\n\nnextStep=$(grep -o '\"'\"'\"nextStep\":[^,]*'\"'\"' $file | sed '\"'\"'s/\"nextStep\"://'\"'\"' | sed -e '\"'\"'s/^[[:space:]]*//'\"'\"' -e '\"'\"'s/[[:space:]]*$//'\"'\"' -e '\"'\"'s/\"//g'\"'\"' -e '\"'\"'s/}//g'\"'\"')\n\nif [ \"${2%$nextStep}\" = \"$2\" ]; then\n  echo \"skip\"\n  exit 0\nfi\n' > nextIndex.sh; "+
			"chmod +x nextIndex.sh; "+
			"mkdir -p /my/path; "+
			"echo -E 'echo hello' > /my/path/script1.sh; "+
			"chmod +x /my/path/script1.sh; "+
			"mkdir -p /my/path; "+
			"echo -E 'echo world' > /my/path/script2.sh; "+
			"chmod +x /my/path/script2.sh; ",
		job.Spec.Template.Spec.InitContainers[0].Args[0],
	)
	require.Len(t, job.Spec.Template.Spec.InitContainers[0].Env, 1)
	assert.Equal(t, "TOKEN", job.Spec.Template.Spec.InitContainers[0].Env[0].Name)
	assert.Equal(t, &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			Key:                  "TOKEN",
			LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
		},
	}, job.Spec.Template.Spec.InitContainers[0].Env[0].ValueFrom)
	require.Len(t, job.Spec.Template.Spec.InitContainers[0].VolumeMounts, 1)
	assert.Equal(t, "job-data", job.Spec.Template.Spec.InitContainers[0].VolumeMounts[0].Name)
	assert.Equal(t, "/job", job.Spec.Template.Spec.InitContainers[0].VolumeMounts[0].MountPath)

	assert.Equal(t, "my-container-0", job.Spec.Template.Spec.InitContainers[1].Name)
	assert.Equal(t, "my-image:latest", job.Spec.Template.Spec.InitContainers[1].Image)
	assert.Equal(t, "/job/repository", job.Spec.Template.Spec.InitContainers[1].WorkingDir)
	require.Len(t, job.Spec.Template.Spec.InitContainers[1].Command, 2)
	assert.Equal(t, []string{"sh", "-c"}, job.Spec.Template.Spec.InitContainers[1].Command)
	require.Len(t, job.Spec.Template.Spec.InitContainers[1].Args, 1)
	assert.Equal(
		t,
		"if [ \"$(/job/nextIndex.sh /job/skip.json my.container.0)\" != \"skip\" ]; then echo; hello;  fi",
		job.Spec.Template.Spec.InitContainers[1].Args[0],
	)
	require.Len(t, job.Spec.Template.Spec.InitContainers[1].Env, 2)
	assert.Equal(t, "FOO", job.Spec.Template.Spec.InitContainers[1].Env[0].Name)
	assert.Equal(t, "bar", job.Spec.Template.Spec.InitContainers[1].Env[0].Value)
	assert.Equal(t, "EXECUTOR_ADD_SAFE", job.Spec.Template.Spec.InitContainers[1].Env[1].Name)
	assert.Equal(t, "false", job.Spec.Template.Spec.InitContainers[1].Env[1].Value)
	require.Len(t, job.Spec.Template.Spec.InitContainers[1].VolumeMounts, 1)
	assert.Equal(t, "job-data", job.Spec.Template.Spec.InitContainers[1].VolumeMounts[0].Name)
	assert.Equal(t, "/job", job.Spec.Template.Spec.InitContainers[1].VolumeMounts[0].MountPath)

	assert.Equal(t, "my-container-1", job.Spec.Template.Spec.InitContainers[2].Name)
	assert.Equal(t, "my-image:latest", job.Spec.Template.Spec.InitContainers[2].Image)
	assert.Equal(t, "/job/repository", job.Spec.Template.Spec.InitContainers[2].WorkingDir)
	require.Len(t, job.Spec.Template.Spec.InitContainers[2].Command, 2)
	assert.Equal(t, []string{"sh", "-c"}, job.Spec.Template.Spec.InitContainers[2].Command)
	require.Len(t, job.Spec.Template.Spec.InitContainers[2].Args, 1)
	assert.Equal(
		t,
		"if [ \"$(/job/nextIndex.sh /job/skip.json my.container.1)\" != \"skip\" ]; then echo; world;  fi",
		job.Spec.Template.Spec.InitContainers[2].Args[0],
	)
	require.Len(t, job.Spec.Template.Spec.InitContainers[2].Env, 2)
	assert.Equal(t, "FOO", job.Spec.Template.Spec.InitContainers[2].Env[0].Name)
	assert.Equal(t, "baz", job.Spec.Template.Spec.InitContainers[2].Env[0].Value)
	assert.Equal(t, "EXECUTOR_ADD_SAFE", job.Spec.Template.Spec.InitContainers[1].Env[1].Name)
	assert.Equal(t, "false", job.Spec.Template.Spec.InitContainers[1].Env[1].Value)
	require.Len(t, job.Spec.Template.Spec.InitContainers[2].VolumeMounts, 1)
	assert.Equal(t, "job-data", job.Spec.Template.Spec.InitContainers[2].VolumeMounts[0].Name)
	assert.Equal(t, "/job", job.Spec.Template.Spec.InitContainers[2].VolumeMounts[0].MountPath)

	require.Len(t, job.Spec.Template.Spec.Containers, 1)
	assert.Equal(t, "main", job.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, "step-image:latest", job.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, []string{"sh", "-c"}, job.Spec.Template.Spec.Containers[0].Command)
	require.Len(t, job.Spec.Template.Spec.Containers[0].Args, 1)
	assert.Equal(t, "echo 'complete'", job.Spec.Template.Spec.Containers[0].Args[0])
	assert.Equal(t, "/job", job.Spec.Template.Spec.Containers[0].WorkingDir)

	assert.Equal(t, resource.MustParse("10"), *job.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu())
	assert.Equal(t, resource.MustParse("10Gi"), *job.Spec.Template.Spec.Containers[0].Resources.Limits.Memory())
	assert.Equal(t, resource.MustParse("1"), *job.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu())
	assert.Equal(t, resource.MustParse("1Gi"), *job.Spec.Template.Spec.Containers[0].Resources.Requests.Memory())
}
