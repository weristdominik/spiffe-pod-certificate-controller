package controller

import (
	"context"
	"fmt"

	certificatesv1 "k8s.io/api/certificates/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ClusterTrustBundleName = "spiffe.weristdominik.de:workload:current"
)

// +kubebuilder:rbac:groups=certificates.k8s.io,resources=clustertrustbundles,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=signers,resourceNames=spiffe.weristdominik.de/workload,verbs=attest

type ClusterTrustBundleReconciler struct {
	client.Client

	SignerName  string
	BundleName  string
	TrustBundle string
}

func (r *ClusterTrustBundleReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// This reconciler manages exactly one bundle.
	if req.Name != r.BundleName {
		return ctrl.Result{}, nil
	}

	var current certificatesv1.ClusterTrustBundle
	err := r.Get(ctx, client.ObjectKey{Name: r.BundleName}, &current)
	if apierrors.IsNotFound(err) {
		bundle := r.desiredBundle()
		if err := r.Create(ctx, bundle); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, fmt.Errorf("create ClusterTrustBundle: %w", err)
		}

		logger.Info("Created ClusterTrustBundle", "name", r.BundleName)
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get ClusterTrustBundle: %w", err)
	}

	// Do not silently take over an object associated with another signer.
	if current.Spec.SignerName != r.SignerName {
		return ctrl.Result{}, fmt.Errorf(
			"ClusterTrustBundle %q has signerName %q, expected %q",
			r.BundleName,
			current.Spec.SignerName,
			r.SignerName,
		)
	}

	if current.Spec.TrustBundle == r.TrustBundle {
		return ctrl.Result{}, nil
	}

	// Preserve metadata and update only the trust bundle contents.
	current.Spec.TrustBundle = r.TrustBundle
	if err := r.Update(ctx, &current); err != nil {
		return ctrl.Result{}, fmt.Errorf("update ClusterTrustBundle: %w", err)
	}

	logger.Info("Updated ClusterTrustBundle", "name", r.BundleName)
	return ctrl.Result{}, nil
}

func (r *ClusterTrustBundleReconciler) desiredBundle() *certificatesv1.ClusterTrustBundle {
	return &certificatesv1.ClusterTrustBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.BundleName,
			Labels: map[string]string{
				"spiffe.weristdominik.de/trust-domain": "cluster.local",
				"spiffe.weristdominik.de/canarying":    "development",
			},
		},
		Spec: certificatesv1.ClusterTrustBundleSpec{
			SignerName:  r.SignerName,
			TrustBundle: r.TrustBundle,
		},
	}
}

func (r *ClusterTrustBundleReconciler) SetupWithManager(
	mgr ctrl.Manager,
) error {
	// Create the initial object so the controller has something to watch.
	// Reconcile will subsequently keep its trust bundle contents in sync.
	initial := r.desiredBundle()
	err := r.Create(context.Background(), initial)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("initialize ClusterTrustBundle: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&certificatesv1.ClusterTrustBundle{}).
		WithOptions(controller.Options{}).
		Complete(r)
}
