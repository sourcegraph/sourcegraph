package k8senvtest

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type KubernetesController interface {
	SetupWithManager(mgr ctrl.Manager) error
}

type NewController func(mgr ctrl.Manager) KubernetesController

func SetupEnvtest(ctx context.Context, logger logr.Logger, newController NewController) (*rest.Config, func() error, error) {
	ctx, cancel := context.WithCancel(ctx)

	kubeBuilderAssets, err := kubebuilderAssetPath()
	if err != nil {
		cancel()
		return nil, nil, err
	}
	testEnv := &envtest.Environment{
		AttachControlPlaneOutput: true,
		BinaryAssetsDirectory:    kubeBuilderAssets,
	}
	apiServerCfg := testEnv.ControlPlane.GetAPIServer()
	apiServerCfg.Configure().Set("bind-address", "127.0.0.1")
	apiServerCfg.Configure().Set("advertise-address", "127.0.0.1")
	cfg, err := testEnv.Start()
	if err != nil {
		cancel()
		return nil, nil, err
	}

	// If we had CRDs, this is where we'd add them to the scheme
	ctrlMgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Logger: logger,
		Scheme: scheme.Scheme,

		// On macos with the built-in firewall enabled, every test run will
		// cause a firewall dialog box to pop. That's incredibly annoying. This
		// can be solved by binding the metrics server to localhost, but we
		// don't need it, so we disable it altogether.
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	if err != nil {
		cancel()
		return nil, nil, err
	}

	controller := newController(ctrlMgr)
	if err := controller.SetupWithManager(ctrlMgr); err != nil {
		cancel()
		return nil, nil, err
	}

	// Start controller manager async. We'll stop it with context cancellation
	// later.
	ctrlMgrDone := make(chan error, 1)
	go func() {
		if err := ctrlMgr.Start(ctx); err != nil {
			ctrlMgrDone <- err
			return
		}
		close(ctrlMgrDone)
	}()

	cleanup := func() error {
		cancel()
		if err := testEnv.Stop(); err != nil {
			return err
		}
		return <-ctrlMgrDone
	}

	return cfg, cleanup, nil
}

func kubebuilderAssetPath() (string, error) {
	if os.Getenv("BAZEL_TEST") == "" {
		return kubebuilderAssetPathLocalDev()
	}

	assetPaths := strings.Split(os.Getenv("KUBEBUILDER_ASSET_PATHS"), " ")
	if len(assetPaths) == 0 {
		return "", errors.New("expected KUBEBUILDER_ASSET_PATHS to not be empty")
	}
	arbAssetPath, err := runfiles.Rlocation(assetPaths[0])
	if err != nil {
		return "", err
	}
	return filepath.Dir(arbAssetPath), nil
}

// In the hermetic bazel environment, we skip setup-envtest, handling the assets
// directly in a hermetic and cachable way.
// If we're using `go test`, which can be convenient for local dev, we fall back
// on expecting setup-envtest to be present on the developer machine.
func kubebuilderAssetPathLocalDev() (string, error) {
	setupEnvTestCmd := exec.Command("setup-envtest", "use", "1.28.0", "--bin-dir", "/tmp/envtest", "-p", "path")
	var envtestOut bytes.Buffer
	setupEnvTestCmd.Stdout = &envtestOut
	if err := setupEnvTestCmd.Run(); err != nil {
		return "", errors.Wrap(err, "error running setup-envtest - did you remember to `go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest`?")
	}
	return strings.TrimSpace(envtestOut.String()), nil
}

func NewNoopReconciler(mgr ctrl.Manager) KubernetesController {
	return noopReconciler{}
}

type noopReconciler struct{}

func (noopReconciler) SetupWithManager(_ ctrl.Manager) error { return nil }

func IsSetupEnvTestInstalled() bool {
	_, err := exec.LookPath("setup-envtest")
	if err != nil {
		return false
	}
	return true
}
