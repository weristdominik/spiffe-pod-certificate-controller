# spiffe-checker

## Build and run

`docker build -t spiffe-checker:local .`

If using kind as your cluster please remember to load image `kind load docker-image spiffe-checker:local`.

Deploy manifest to cluster - this will use the currently configured kubeconfig: `kubectl apply -f deployment.yaml`

Inspect pod logs:

```bash
# spiffe-checker-server
kubectl -n spiffe-checker-server logs -l app=spiffe-checker-server
# TLS credentials/trust bundle loaded or reloaded
# 2026/10/26 18:18:17 HTTPS server listening on :8443
# 2026/10/26 18:18:27 trusted client: spiffe://cluster.local/ns/spiffe-checker-client/sa/checker-client

# spiffe-checker-client
kubectl -n spiffe-checker-client logs -l app=spiffe-checker-client
# TLS credentials/trust bundle loaded or reloaded
# 2026/10/26 18:18:27 server response: 200 OK
# 2026/10/26 18:18:27 authenticated request successful
```