# Spiffe-Pod-Certificate-Controller

A Controller to issue [PodCertificateRequests](https://kubernetes.io/docs/reference/kubernetes-api/certificates/pod-certificate-request-v1/) and populate [ClusterTrustBundles](https://kubernetes.io/docs/reference/kubernetes-api/certificates/cluster-trust-bundle-v1/)

## General

Starting with [Kubernetes v1.37.x](https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/) the usage of Pod certificates and Cluster Trust Bundeles became stable. ([kubernetes-v1-37-release-notes](https://kubernetes.io/blog/2026/08/26/kubernetes-v1-37-release/#pod-certificates-and-clustertrustbundles)).

## Run

```bash
# kubectl -k apply -f config/default
make deploy
```

Or run the controller locally `make run` on your machine - this will use the currently configured kubeconfig in your shell. If you need an local kubernetes cluster you can use `kind create cluster --image=kindest/node:v1.37.0` to create one

## Usage

Spiffe-Pod-Certificate-Controller implements two independent controllers `clustertrustbundle` and `podcertificaterequest`. If an Pod is deployed with the [prodcertificate syntax](./config/demo/pod.yaml), the PodCertificateRequest will be signed/ issued by the selected ca and mounted into the Pod filesystem.
