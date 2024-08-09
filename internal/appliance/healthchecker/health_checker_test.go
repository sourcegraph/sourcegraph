package healthchecker

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logr"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/appliance/k8senvtest"
	"github.com/sourcegraph/sourcegraph/internal/k8s/resource/service"
)

var (
	// set once, before suite runs. See TestMain
	ctx       context.Context
	k8sClient client.Client
)

func TestMain(m *testing.M) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	if !k8senvtest.IsSetupEnvTestInstalled() {
		fmt.Println("setup-envtest is not installed, skipping Appliance healthchecker tests")
		os.Exit(0)
	}

	logger := log.Scoped("appliance-healthchecker-tests")
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

// A bit of a lengthy scenario-style test
func TestManageIngressFacingService(t *testing.T) {
	ns, err := k8senvtest.NewRandomNamespace("test-appliance-self-update")
	require.NoError(t, err)
	err = k8sClient.Create(ctx, ns)
	require.NoError(t, err)

	serviceName := types.NamespacedName{Namespace: ns.GetName(), Name: "sourcegraph-frontend"}
	checker := &HealthChecker{
		Probe:     &PodProbe{K8sClient: k8sClient},
		K8sClient: k8sClient,
		Logger:    logtest.Scoped(t),

		ServiceName: serviceName,
		Graceperiod: 0,
	}

	// Simulate helm having created the service, but no frontend pods have been
	// created yet
	svc := service.NewService("sourcegraph-frontend", ns.GetName(), nil)
	svc.Spec.Ports = []corev1.ServicePort{
		{Name: "http", Port: 30080, TargetPort: intstr.FromString("http")},
	}
	svc.Spec.Selector = map[string]string{
		"app": "sourcegraph-appliance-frontend",
	}
	err = k8sClient.Create(ctx, &svc)
	require.NoError(t, err)
	runHealthCheckAndAssertSelector(t, checker, serviceName, ns.GetName(), "sourcegraph-appliance-frontend")

	// Simulate some frontend pods existing but with no readiness conditions.
	pod1 := mkPod("pod1", ns.GetName())
	err = k8sClient.Create(ctx, pod1)
	require.NoError(t, err)
	pod2 := mkPod("pod2", ns.GetName())
	err = k8sClient.Create(ctx, pod2)
	require.NoError(t, err)
	runHealthCheckAndAssertSelector(t, checker, serviceName, ns.GetName(), "sourcegraph-appliance-frontend")

	// Simulate one pod becoming ready to receive traffic
	pod1.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		},
	}
	err = k8sClient.Status().Update(ctx, pod1)
	require.NoError(t, err)
	pod2.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionFalse,
		},
	}
	err = k8sClient.Status().Update(ctx, pod2)
	require.NoError(t, err)
	runHealthCheckAndAssertSelector(t, checker, serviceName, ns.GetName(), "sourcegraph-frontend")

	// test idempotency of the monitor
	runHealthCheckAndAssertSelector(t, checker, serviceName, ns.GetName(), "sourcegraph-frontend")

	// Simulate pods becoming unready
	pod1.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionFalse,
		},
	}
	err = k8sClient.Status().Update(ctx, pod1)
	require.NoError(t, err)
	runHealthCheckAndAssertSelector(t, checker, serviceName, ns.GetName(), "sourcegraph-appliance-frontend")
}

func runHealthCheckAndAssertSelector(t *testing.T, checker *HealthChecker, serviceName types.NamespacedName, namespace, expectedSelectorValue string) {
	err := checker.maybeFlipServiceOnce(ctx, "app=sourcegraph-frontend", namespace)
	require.NoError(t, err)

	var svc corev1.Service
	err = k8sClient.Get(ctx, serviceName, &svc)
	require.NoError(t, err)

	require.Equal(t, expectedSelectorValue, svc.Spec.Selector["app"])
}

func mkPod(name, namespace string) *corev1.Pod {
	ctr := corev1.Container{
		Name:    "frontend",
		Image:   "foo:bar",
		Command: []string{"doitnow"},
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"app": "sourcegraph-frontend"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{ctr},
		},
	}
}
