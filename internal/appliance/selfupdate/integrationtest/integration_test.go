package integrationtest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logr"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/appliance/config"
	"github.com/sourcegraph/sourcegraph/internal/appliance/k8senvtest"
	"github.com/sourcegraph/sourcegraph/internal/appliance/selfupdate"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/container"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/deployment"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/pod"
	"github.com/sourcegraph/sourcegraph/internal/releaseregistry"
	"github.com/sourcegraph/sourcegraph/internal/releaseregistry/mocks"
)

// Separate envtest-using things here, that have expensive setup and teardown,
// from the faster unit tests.

var (
	// set once, before suite runs. See TestMain
	ctx       context.Context
	k8sClient client.Client
)

func TestMain(m *testing.M) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	logger := log.Scoped("selfupdate-tests")
	k8sConfig, cleanup, err := k8senvtest.SetupEnvtest(ctx, logr.New(logger), k8senvtest.NewNoopReconciler)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer func() {
		if err := cleanup(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}()

	k8sClient, err = client.New(k8sConfig, client.Options{})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	rc := m.Run()

	// Our earlier defer won't run after we call os.Exit() below
	if err := cleanup(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	os.Exit(rc)
}

func TestSelfUpdateLoop(t *testing.T) {
	ns, err := k8senvtest.NewRandomNamespace("test-appliance-self-update")
	require.NoError(t, err)
	err = k8sClient.Create(ctx, ns)
	require.NoError(t, err)
	nsName := ns.GetName()

	// provision example appliance deployment
	dep1 := buildTestDeployment("appliance", nsName)
	err = k8sClient.Create(ctx, dep1)
	require.NoError(t, err)

	dep2 := buildTestDeployment("appliance-frontend", nsName)
	err = k8sClient.Create(ctx, dep2)
	require.NoError(t, err)

	cfgMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigmapName,
			Namespace: nsName,
			Annotations: map[string]string{
				config.AnnotationKeyCurrentVersion: "4.3.1",
			},
		},
	}
	err = k8sClient.Create(ctx, cfgMap)
	require.NoError(t, err)

	relregClient := mocks.NewMockReleaseRegistryClient()
	relregClient.ListVersionsFunc.SetDefaultReturn([]releaseregistry.ReleaseInfo{
		{Version: "4.5.7", Public: true},
	}, nil)
	selfUpdater := &selfupdate.SelfUpdate{
		Interval:        time.Second,
		Logger:          logtest.Scoped(t),
		RelregClient:    relregClient,
		K8sClient:       k8sClient,
		DeploymentNames: "appliance,appliance-frontend",
		Namespace:       nsName,
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	loopDone := make(chan struct{})
	go func() {
		err := selfUpdater.Loop(loopCtx)
		if !errors.Is(err, context.Canceled) {
			require.NoError(t, err)
		}
		close(loopDone)
	}()

	for _, depName := range []string{"appliance", "appliance-frontend"} {
		require.Eventually(t, func() bool {
			var dep appsv1.Deployment
			depName := types.NamespacedName{Name: depName, Namespace: nsName}
			require.NoError(t, k8sClient.Get(ctx, depName, &dep))
			return strings.HasSuffix(dep.Spec.Template.Spec.Containers[0].Image, "4.5.7")
		}, time.Second*10, time.Second)
	}

	cancel()
	<-loopDone
}

func buildTestDeployment(name, namespace string) *appsv1.Deployment {
	defaultContainer := container.NewContainer(name, nil, config.ContainerConfig{
		Image:     "index.docker.io/sourcegraph/appliance:4.3.1",
		Resources: &corev1.ResourceRequirements{},
	})
	podTemplate := pod.NewPodTemplate(name, nil)
	podTemplate.Template.Spec.Containers = []corev1.Container{defaultContainer}
	defaultDeployment := deployment.NewDeployment(name, namespace, "4.3.1")
	defaultDeployment.Spec.Template = podTemplate.Template
	return &defaultDeployment
}
