package controller

import (
	"context"
	"fmt"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/weristdominik/spiffe-pod-certificate-controller/internal/signer"
)

// +kubebuilder:rbac:groups=certificates.k8s.io,resources=podcertificaterequests,verbs=get;list;watch
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=podcertificaterequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=signers,resourceNames=spiffe.weristdominik.de/workload,verbs=sign

type PodCertificateRequestReconciler struct {
	client.Client
	SignerName string
	Issuer     signer.Issuer
}

func (r *PodCertificateRequestReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var pcr certificatesv1.PodCertificateRequest
	if err := r.Get(ctx, req.NamespacedName, &pcr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if pcr.Spec.SignerName != r.SignerName {
		return ctrl.Result{}, nil
	}

	logger.Info(
		"Reconciling PodCertificateRequest",
		"request", req.NamespacedName,
		"pod", pcr.Spec.PodName,
		"podUID", pcr.Spec.PodUID,
		"serviceAccount", pcr.Spec.ServiceAccountName,
		"node", pcr.Spec.NodeName,
	)

	// Don't issue a request more than once.
	if hasTerminalCondition(&pcr) {
		return ctrl.Result{}, nil
	}

	// TODO implement logic so custom CA can be used
	// No issuer is configured unless debug mode or another issuer is enabled.
	if r.Issuer == nil {
		logger.Info(
			"No issuer configured; leaving request pending",
			"request", pcr.Name,
		)
		return ctrl.Result{}, nil
	}

	result, err := r.Issuer.Issue(&pcr)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf(
			"issue certificate for request %s: %w",
			pcr.Name,
			err,
		)
	}

	// Populate the status fields expected by the native Pod Certificate API.
	pcr.Status.CertificateChain = string(result.CertificateChain)
	pcr.Status.NotBefore = ptrToMetaTime(result.NotBefore)
	pcr.Status.NotAfter = ptrToMetaTime(result.NotAfter)
	pcr.Status.BeginRefreshAt = ptrToMetaTime(result.BeginRefreshAt)
	pcr.Status.Conditions = append(pcr.Status.Conditions, metav1.Condition{
		Type:               "Issued",
		Status:             metav1.ConditionTrue,
		Reason:             "Issued",
		Message:            "Certificate issued by the configured signer",
		ObservedGeneration: pcr.Generation,
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, &pcr); err != nil {
		return ctrl.Result{}, fmt.Errorf(
			"update PodCertificateRequest status: %w",
			err,
		)
	}

	logger.Info("Certificate issued", "request", pcr.Name)
	return ctrl.Result{}, nil
}

func ptrToMetaTime(t time.Time) *metav1.Time {
	mt := metav1.NewTime(t)
	return &mt
}

func hasTerminalCondition(pcr *certificatesv1.PodCertificateRequest) bool {
	for _, condition := range pcr.Status.Conditions {
		switch condition.Type {
		case "Issued", "Denied", "Failed":
			return true
		}
	}
	return false
}

func (r *PodCertificateRequestReconciler) SetupWithManager(
	mgr ctrl.Manager,
) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&certificatesv1.PodCertificateRequest{}).
		WithOptions(controller.Options{}).
		Complete(r)
}
