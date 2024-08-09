package reconciler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/sourcegraph/log/logr"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/appliance/config"
	"github.com/sourcegraph/sourcegraph/internal/appliance/k8senvtest"
)

// Test helpers

type ApplianceTestSuite struct {
	suite.Suite

	ctx            context.Context
	k8sClient      *kubernetes.Clientset
	envtestCleanup func() error
}

func TestApplianceTestSuite(t *testing.T) {
	if !k8senvtest.IsSetupEnvTestInstalled() {
		t.Skip("setup-envtest is not installed, skipping ApplianceTestSuite")
	}
	suite.Run(t, new(ApplianceTestSuite))
}

func (suite *ApplianceTestSuite) SetupSuite() {
	suite.ctx = context.Background()

	var k8sConfig *rest.Config
	var err error
	logger := logr.New(logtest.Scoped(suite.T()))
	k8sConfig, suite.envtestCleanup, err = k8senvtest.SetupEnvtest(suite.ctx, logger, newReconciler)
	suite.Require().NoError(err)
	suite.k8sClient, err = kubernetes.NewForConfig(k8sConfig)
	suite.Require().NoError(err)
}

func newReconciler(ctrlMgr ctrl.Manager) k8senvtest.KubernetesController {
	return &Reconciler{
		Client:   ctrlMgr.GetClient(),
		Scheme:   ctrlMgr.GetScheme(),
		Recorder: ctrlMgr.GetEventRecorderFor("appliance"),
	}
}

func (suite *ApplianceTestSuite) TearDownSuite() {
	suite.Require().NoError(suite.envtestCleanup())
}

func (suite *ApplianceTestSuite) createConfigMapAndAwaitReconciliation(fixtureFileName string) string {
	// Create a random namespace for each test
	namespace, err := k8senvtest.NewRandomNamespace("test-appliance")
	suite.Require().NoError(err)
	_, err = suite.k8sClient.CoreV1().Namespaces().Create(suite.ctx, namespace, metav1.CreateOptions{})
	suite.Require().NoError(err)

	cfgMap := suite.newConfigMap(namespace.GetName(), fixtureFileName)
	suite.awaitReconciliation(namespace.GetName(), func() {
		_, err := suite.k8sClient.CoreV1().ConfigMaps(namespace.GetName()).Create(suite.ctx, cfgMap, metav1.CreateOptions{})
		suite.Require().NoError(err)
	})
	return namespace.GetName()
}

func (suite *ApplianceTestSuite) updateConfigMapAndAwaitReconciliation(namespace, fixtureFileName string) {
	cfgMap := suite.newConfigMap(namespace, fixtureFileName)
	suite.awaitReconciliation(namespace, func() {
		_, err := suite.k8sClient.CoreV1().ConfigMaps(namespace).Update(suite.ctx, cfgMap, metav1.UpdateOptions{})
		suite.Require().NoError(err)
	})
}

// Synchronize test and controller code by counting ReconcileFinished events. We
// expect exactly 2 from one initial creation or update of an SG ConfigMap. This
// is because we update the ConfigMap at the end of the reconcile loop with
// annotations. This triggers another reconcile loop. This all ends when the
// changes are no-ops.
func (suite *ApplianceTestSuite) awaitReconciliation(namespace string, op func()) {
	events := suite.getConfigMapReconcileEventCount(namespace)
	op()
	suite.Require().Eventually(func() bool {
		return suite.getConfigMapReconcileEventCount(namespace) >= events+2
	}, time.Second*10, time.Millisecond*200)
}

func (suite *ApplianceTestSuite) getConfigMapReconcileEventCount(namespace string) int32 {
	t := suite.T()
	events, err := suite.k8sClient.CoreV1().Events(namespace).List(suite.ctx, metav1.ListOptions{
		FieldSelector: "involvedObject.name=sg,reason=ReconcileFinished",
		TypeMeta:      metav1.TypeMeta{Kind: "ConfigMap"},
	})
	require.NoError(t, err)
	if len(events.Items) == 0 {
		return 0
	}
	event := events.Items[0]
	return event.Count
}

func (suite *ApplianceTestSuite) newConfigMap(namespace, fixtureFileName string) *corev1.ConfigMap {
	t := suite.T()
	cfgBytes, err := os.ReadFile(filepath.Join("testdata", "sg", fixtureFileName+".yaml"))
	require.NoError(t, err)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sg",
			Namespace: namespace,
			Annotations: map[string]string{
				config.AnnotationKeyManaged: "true",
			},
		},
		Data: map[string]string{"spec": string(cfgBytes)},
	}
}
