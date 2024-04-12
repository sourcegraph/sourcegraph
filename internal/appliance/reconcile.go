package appliance

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/sourcegraph/internal/appliance/hash"
)

var _ reconcile.Reconciler = &Reconciler{}

type Reconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
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

	applianceSpec.Labels = hash.SetTemplateHashLabel(applianceSpec.Labels, applianceSpec.Data)

	if applianceSpec.GetDeletionTimestamp() != nil {
		r.Recorder.Event(&applianceSpec, "Warning", "Deleting", fmt.Sprintf("ConfigMap %s is being deleted from the namespace %s", req.Name, req.Namespace))
		err = r.Delete(ctx, &applianceSpec, client.Preconditions{
			UID:             &applianceSpec.UID,
			ResourceVersion: &applianceSpec.ResourceVersion,
		})

		if err != nil && client.IgnoreNotFound(err) != nil {
			reqLog.Error(err, "failed to delete sourcegraph appliance spec")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// TODO place holder code until we get the configmap spec'd out and working'
	data, ok := applianceSpec.Data["spec"]
	if !ok {
		return ctrl.Result{}, errors.New("failed to get sourcegraph spec from configmap")
	}

	var sourcegraph Sourcegraph
	if err := yaml.Unmarshal([]byte(data), &sourcegraph); err != nil {
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	applianceAnnotationPredicate := predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetAnnotations()["appliance.sourcegraph.com/managed"] == "true"
	})

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(applianceAnnotationPredicate).
		For(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
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
