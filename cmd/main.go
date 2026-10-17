package main

import (
	"flag"
	"os"

	certificatesv1 "k8s.io/api/certificates/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/weristdominik/spiffe-pod-certificate-controller/internal/controller"
	"github.com/weristdominik/spiffe-pod-certificate-controller/internal/signer"
)

const signerName = "spiffe.weristdominik.de/workload"

func main() {
	var (
		debug                bool
		enableLeaderElection bool
	)

	flag.BoolVar(&debug, "debug", false, "Enable the development CA")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	scheme := runtime.NewScheme()
	must(certificatesv1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "spiffe-pod-certificate-controller.weristdominik.de",
	})
	must(err)

	var issuer signer.Issuer
	var ca *signer.DevCA

	if debug {
		ca, err = signer.NewDevCA()
		must(err)

		issuer = ca
	}

	pcrReconciler := &controller.PodCertificateRequestReconciler{
		Client:     mgr.GetClient(),
		SignerName: signerName,
		Issuer:     issuer,
	}
	must(pcrReconciler.SetupWithManager(mgr))

	if ca != nil {
		trustBundleReconciler := &controller.ClusterTrustBundleReconciler{
			Client:      mgr.GetClient(),
			SignerName:  signerName,
			BundleName:  controller.ClusterTrustBundleName,
			TrustBundle: string(ca.TrustBundlePEM()),
		}
		must(trustBundleReconciler.SetupWithManager(mgr))
	}

	must(mgr.Start(ctrl.SetupSignalHandler()))
}

func must(err error) {
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
