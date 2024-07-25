package reconciler

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	"github.com/sourcegraph/sourcegraph/internal/appliance/config"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var _ reconcile.Reconciler = &Reconciler{}

type Reconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	BeginHealthCheckLoop chan struct{}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLog := log.FromContext(ctx)
	reqLog.Info("reconciling sourcegraph appliance")

	var applianceSpec corev1.ConfigMap
	err := r.Get(ctx, req.NamespacedName, &applianceSpec)
	if apierrors.IsNotFound(err) {
		// Object not found, maybe deleted.
		return ctrl.Result{}, nil
	} else if err != nil {
		reqLog.Error(err, "failed to fetch sourcegraph appliance spec")
		return ctrl.Result{}, err
	}

	// Emit a ReconcileFinished event at the end. Currently, this is only used
	// to synchronize this reconcile loop with test code, allowing reliable
	// assertions on the state of the cluster at the time this event is emitted.
	// Perhaps this should be feature-flagged so that it is only emitted during
	// tests, if it isn't useful elsewhere.
	defer r.Recorder.Event(&applianceSpec, "Normal", "ReconcileFinished", "Reconcile finished.")

	status := applianceSpec.GetAnnotations()[config.AnnotationKeyStatus]
	if config.IsPostInstallStatus(config.Status(status)) {
		close(r.BeginHealthCheckLoop)
	}

	// TODO place holder code until we get the configmap spec'd out and working'
	data, ok := applianceSpec.Data["spec"]
	if !ok {
		return ctrl.Result{}, errors.New("failed to get sourcegraph spec from configmap")
	}

	sourcegraph := config.NewDefaultConfig()
	if err := yaml.Unmarshal([]byte(data), &sourcegraph); err != nil {
		return reconcile.Result{}, err
	}

	// config.Sourcegraph is a kubebuilder-scaffolded custom type, but we do not
	// actually ask operators to install CRDs. Therefore, we set its namespace
	// based on the actual object being reconciled, so that more deeply-nested
	// code can treat it like a CRD.
	sourcegraph.Namespace = applianceSpec.GetNamespace()

	// Similarly, we simulate a CRD status using an annotation. ConfigMaps don't
	// have Statuses, so we must use annotations to drive this.
	// This can be empty string.
	sourcegraph.Status.CurrentVersion = applianceSpec.GetAnnotations()[config.AnnotationKeyCurrentVersion]

	// Reconcile services here
	var reconcile func(ctx context.Context, sourcegraph *config.Sourcegraph, owner client.Object) error
	for _, service := range config.SourcegraphServicesToReconcile {
		switch service {
		case "blobstore":
			reconcile = r.reconcileBlobstore
		case "repo-updater":
			reconcile = r.reconcileRepoUpdater
		case "symbols":
			reconcile = r.reconcileSymbols
		case "gitserver":
			reconcile = r.reconcileGitServer
		case "redis":
			reconcile = r.reconcileRedis
		case "pgsql":
			reconcile = r.reconcilePGSQL
		case "syntect":
			reconcile = r.reconcileSyntect
		case "precise-code-intel":
			reconcile = r.reconcilePreciseCodeIntel
		case "code-insights-db":
			reconcile = r.reconcileCodeInsights
		case "code-intel-db":
			reconcile = r.reconcileCodeIntel
		case "prometheus":
			reconcile = r.reconcilePrometheus
		case "cadvisor":
			reconcile = r.reconcileCadvisor
		case "worker":
			reconcile = r.reconcileWorker
		case "frontend":
			reconcile = r.reconcileFrontend
		case "searcher":
			reconcile = r.reconcileSearcher
		case "indexed-searcher":
			reconcile = r.reconcileIndexedSearcher
		case "grafana":
			reconcile = r.reconcileGrafana
		case "jaeger":
			reconcile = r.reconcileJaeger
		case "otel":
			reconcile = r.reconcileOtel
		}

		if err := reconcile(ctx, &sourcegraph, &applianceSpec); err != nil {
			return ctrl.Result{}, errors.Newf("failed to reconcile %s: %w", service, err)
		}
	}

	// Set the current version annotation in case migration logic depends on it.
	applianceSpec.Annotations[config.AnnotationKeyCurrentVersion] = sourcegraph.Spec.RequestedVersion
	if err := r.Client.Update(ctx, &applianceSpec); err != nil {
		return ctrl.Result{}, errors.Newf("failed to update current version annotation: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	applianceAnnotationPredicate := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetAnnotations()[config.AnnotationKeyManaged] == "true"
	})

	// When updating this list of owned resources, please update the
	// corresponding code in gatherResources() in golden_test.go.
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(applianceAnnotationPredicate).
		For(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// GetObject will get an object with the given name and namespace via the K8s API. The result will be stored in the
// provided object.
func (r *Reconciler) GetObject(ctx context.Context, name, namespace string, object client.Object) error {
	return r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, object)
}

// IsObjectFound will perform a basic check that the given object exists via the K8s API. If an error occurs,
// the function will return false.
func (r *Reconciler) IsObjectFound(ctx context.Context, name, namespace string, object client.Object) bool {
	return !apierrors.IsNotFound(r.GetObject(ctx, name, namespace, object))
}
