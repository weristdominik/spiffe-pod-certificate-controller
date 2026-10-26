package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	mode := flag.String("mode", "", "Mode: server or client")
	credential := flag.String("credential", "/var/run/spiffe/credential.pem", "Workload credential PEM")
	trustBundle := flag.String("trust-bundle", "/var/run/spiffe/trust-bundle.pem", "Trusted CA bundle PEM")
	reloadInterval := flag.Duration("reload-interval", 2*time.Second, "TLS material reload interval")
	minTLS := flag.String("min-tls-version", "1.2", "Minimum TLS version: 1.2 or 1.3")

	listen := flag.String("listen", ":8443", "Server listen address")
	expectedClient := flag.String("expected-client", "", "Expected client SPIFFE ID")

	target := flag.String("target", "", "HTTPS server URL")
	expectedServer := flag.String("expected-server", "", "Expected server SPIFFE ID")
	requestInterval := flag.Duration("request-interval", 10*time.Second, "Client request interval")

	flag.Parse()

	if *mode != "server" && *mode != "client" {
		log.Fatal("--mode must be server or client")
	}
	if *reloadInterval <= 0 {
		log.Fatal("--reload-interval must be positive")
	}

	tlsVersion, err := parseTLSVersion(*minTLS)
	if err != nil {
		log.Fatal(err)
	}

	cfg := Config{
		CredentialFile:  *credential,
		TrustBundleFile: *trustBundle,
		ReloadInterval:  *reloadInterval,
		MinTLSVersion:   tlsVersion,
		ListenAddress:   *listen,
		ExpectedClient:  *expectedClient,
		TargetURL:       *target,
		ExpectedServer:  *expectedServer,
		RequestInterval: *requestInterval,
	}

	if err := cfg.Validate(*mode); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := NewChecker(cfg)
	if err != nil {
		log.Fatal(err)
	}
	go app.Watch(ctx)

	switch *mode {
	case "server":
		err = app.RunServer(ctx)
	case "client":
		err = app.RunClient(ctx)
	}
	if err != nil {
		log.Fatal(err)
	}
}
