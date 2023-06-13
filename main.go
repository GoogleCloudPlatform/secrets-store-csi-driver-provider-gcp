// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Binary secrets-store-csi-driver-provider-gcp is a plugin for the
// secrets-store-csi-driver for fetching secrets from Google Cloud's Secret
// Manager API.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"      //	For more details see: https://pkg.go.dev/net
	"net/http" //	Package http provides HTTP client and server implementations. For details see: https://pkg.go.dev/net/http

	/*
		pprof is a tool for visualization and analysis of profiling data.
		For more details see: https://pkg.go.dev/net/http/pprof
	*/
	"net/http/pprof"
	"os"
	"os/signal" //	For details see: https://pkg.go.dev/os/signal
	"path/filepath"
	"syscall"
	"time"

	/*
		This is a utility library for communicating with Google Cloud metadata service on Google Cloud.
		For more details see: https://pkg.go.dev/cloud.google.com/go/compute/metadata#section-readme
	*/
	"cloud.google.com/go/compute/metadata"

	/*
		Credentials package creates short-lived, limited-privilege credentials for IAM service accounts.
		For more details see: https://pkg.go.dev/cloud.google.com/go/iam/credentials/apiv1
	*/
	iam "cloud.google.com/go/iam/credentials/apiv1"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"

	/*
		Package auth includes obtains auth tokens for workload identity.
		For more details see: https://pkg.go.dev/github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/auth
	*/

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/auth"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/infra"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/server"
	"github.com/prometheus/client_golang/prometheus/promhttp" // For more details see: https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp

	/*
		Prometheus is an open-source technology designed to provide monitoring and alerting functionality for cloud-native environments, including Kubernetes.
		For more details see: https://pkg.go.dev/go.opentelemetry.io/otel/exporters/prometheus
	*/
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"google.golang.org/api/option" // For more details see: https://pkg.go.dev/google.golang.org/api/option
	"google.golang.org/grpc"       // RPC = Remote Procedural Call. For more details see: https://pkg.go.dev/google.golang.org/grpc
	"google.golang.org/grpc/credentials"

	/*
		Package kubernetes holds packages which implement a clientset for Kubernetes APIs.
		For more details see: https://pkg.go.dev/k8s.io/client-go/kubernetes
	*/
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"            // For more details see: https://pkg.go.dev/k8s.io/client-go/rest
	"k8s.io/client-go/tools/clientcmd" // For more details see: https://pkg.go.dev/k8s.io/client-go/tools/clientcmd
	logsapi "k8s.io/component-base/logs/api/v1"

	/*
		For more details check: https://pkg.go.dev/k8s.io/component-base@v0.27.2/logs/json
		TODO: Is package import depriciated and needs to be changed?
	*/
	jlogs "k8s.io/component-base/logs/json"

	/*
		Klog. klog is the Kubernetes logging library.
		klog generates log messages for the Kubernetes system components.
	*/
	"k8s.io/klog/v2"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

/*
Creates flags of different data types. 3 Arguments are:
1) Name of flag (analogical to variable name)
2) default value
3) usage string to define the purpose the flag is created for
*/
var (
	kubeconfig            = flag.String("kubeconfig", "", "absolute path to kubeconfig file")
	logFormatJSON         = flag.Bool("log-format-json", true, "set log formatter to json")
	metricsAddr           = flag.String("metrics_addr", ":8095", "configure http listener for reporting metrics")
	enableProfile         = flag.Bool("enable-pprof", false, "enable pprof profiling")
	debugAddr             = flag.String("debug_addr", "localhost:6060", "port for pprof profiling")
	_                     = flag.Bool("write_secrets", false, "[unused]") // TODO: Can this be deleted?
	smConnectionPoolSize  = flag.Int("sm_connection_pool_size", 5, "size of the connection pool for the secret manager API client")
	iamConnectionPoolSize = flag.Int("iam_connection_pool_size", 5, "size of the connection pool for the IAM API client")

	version = "dev"
)

func main() {
	klog.InitFlags(nil) //  explicitly for initializing global flags
	defer klog.Flush()  //	delay flushing log I/O that is to be written
	// function calls with defer before them means it executes towards after all other lines in the function are executed i.e. towards the end of the function.

	flag.Parse() /*	Parse parses the command-line flags from os.Args[1:].
		Must be called after all flags are defined and before flags are accessed by the program. */

	if *logFormatJSON { // will execute as when set to default, the flag value is true
		jsonFactory := jlogs.Factory{}                                                //Factory produces JSON logger instances. Struct variable
		logger, _ := jsonFactory.Create(logsapi.LoggingConfiguration{Format: "json"}) // all logs are created in json format
		klog.SetLogger(logger)
	}

	/*
		Context is essentially the configuration that you use to access a particular cluster & namespace with a user account.
		context.Background returns a non-nil empty context having no variables, no deadlines and cannot be cancelled.
		NotifyContext returns a copy of the parent context that is marked done (its Done channel is closed) when one of the listed signals arrives, when the returned stop function is called, or when the parent context's Done channel is closed, whichever happens first.
		For more details see: https://pkg.go.dev/os/signal#NotifyContext
		SIGINT: Ctrl+C to terminate program abruptly
		SIGTETM: Default Kill Signal
	*/
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ua := fmt.Sprintf("secrets-store-csi-driver-provider-gcp/%s", version)
	klog.InfoS(fmt.Sprintf("starting %s", ua)) // Stmt added to log

	// Kubernetes Client
	var rc *rest.Config // crate rc variable of type Config which is a struct. See: https://pkg.go.dev/k8s.io/client-go/rest#Config
	var err error
	if *kubeconfig != "" {
		klog.V(5).InfoS("using kubeconfig", "path", *kubeconfig)
		rc, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		klog.V(5).InfoS("using in-cluster kubeconfig")
		/*
			InClusterConfig returns a config object which uses the service account kubernetes gives to pods. It's intended for clients that expect to be running inside a pod running on kubernetes.
		*/
		rc, err = rest.InClusterConfig()
	}
	if err != nil {
		klog.ErrorS(err, "failed to read kubeconfig")
		klog.Fatal("failed to read kubeconfig")
	}

	clientset, err := kubernetes.NewForConfig(rc) //	NewForConfig creates a new Clientset for the given config. Clientset is a struct.
	if err != nil {
		klog.ErrorS(err, "failed to configure k8s client")
		klog.Fatal("failed to configure k8s client")
	}

	// Secret Manager client
	//
	// build without auth so that authentication can be re-added on a per-RPC
	// basis for each mount
	smOpts := []option.ClientOption{ // create a slice variable of ClientOption interface type
		option.WithUserAgent(ua),
		// tell the secretmanager library to not add transport-level ADC since
		// we need to override on a per call basis
		option.WithoutAuthentication(),
		// grpc oauth TokenSource credentials require transport security, so
		// this must be set explicitly even though TLS is used
		option.WithGRPCDialOption(grpc.WithTransportCredentials(credentials.NewTLS(nil))),
		// establish a pool of underlying connections to the Secret Manager API
		// to decrease blocking since same client will be used across concurrent
		// requests. Note that this is implemented in
		// google.golang.org/api/option and not grpc itself.
		option.WithGRPCConnectionPool(*smConnectionPoolSize),
	}

	sc, err := secretmanager.NewClient(ctx, smOpts...)
	if err != nil {
		klog.ErrorS(err, "failed to create secretmanager client")
		klog.Fatal("failed to create secretmanager client")
	}

	// IAM client
	//
	// build without auth so that authentication can be re-added on a per-RPC
	// basis for each mount
	iamOpts := []option.ClientOption{
		option.WithUserAgent(ua),
		// tell the secretmanager library to not add transport-level ADC since
		// we need to override on a per call basis
		option.WithoutAuthentication(),
		// grpc oauth TokenSource credentials require transport security, so
		// this must be set explicitly even though TLS is used
		option.WithGRPCDialOption(grpc.WithTransportCredentials(credentials.NewTLS(nil))),
		// establish a pool of underlying connections to the Secret Manager API
		// to decrease blocking since same client will be used across concurrent
		// requests. Note that this is implemented in
		// google.golang.org/api/option and not grpc itself.
		option.WithGRPCConnectionPool(*iamConnectionPoolSize),
	}

	iamc, err := iam.NewIamCredentialsClient(ctx, iamOpts...)
	if err != nil {
		klog.ErrorS(err, "failed to create iam client")
		klog.Fatal("failed to create iam client")
	}

	// HTTP client
	hc := &http.Client{ // struct of type Client
		Transport: &http.Transport{ //	struct of type Transport
			Dial: (&net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
		},
		Timeout: 60 * time.Second,
	}

	c := &auth.Client{
		KubeClient:     clientset,
		IAMClient:      iamc,
		MetadataClient: metadata.NewClient(hc),
		HTTPClient:     hc,
	}

	// setup provider grpc server
	s := &server.Server{
		SecretClient: sc, // Client for interacting with SM API
		AuthClient:   c,
	}

	socketPath := filepath.Join(os.Getenv("TARGET_DIR"), "gcp.sock")
	// Attempt to remove the UDS to handle cases where a previous execution was
	// killed before fully closing the socket listener and unlinking.
	_ = os.Remove(socketPath)

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		klog.ErrorS(err, "unable to listen to unix socket", "path", socketPath)
		klog.Fatalln("unable to start")
	}
	defer l.Close()

	g := grpc.NewServer( //	NewServer creates a gRPC server which has no service registered and has not started to accept requests yet.
		grpc.UnaryInterceptor(infra.LogInterceptor()),
	)
	v1alpha1.RegisterCSIDriverProviderServer(g, s)
	go g.Serve(l)

	// initialize metrics and health http server
	mux := http.NewServeMux() //	ServeMux is an HTTP request multiplexer. It matches the URL of each incoming request against a list of registered patterns and calls the handler for the pattern that most closely matches the URL.
	ms := http.Server{
		Addr:        *metricsAddr,
		Handler:     mux,
		ReadTimeout: 10 * time.Second,
	}
	defer ms.Shutdown(ctx)

	_, err = otelprom.New() // returns a new prometheus exporter. Used to collect, aggregate and send metrics to the backend platform
	if err != nil {
		klog.ErrorS(err, "unable to initialize prometheus registry")
		klog.Fatalln("unable to initialize prometheus registry")
	}

	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	go func() {
		if err := ms.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			klog.ErrorS(err, "metrics http server error")
		}
	}()
	klog.InfoS("health server listening", "addr", *metricsAddr)

	if *enableProfile {
		dmux := http.NewServeMux()
		dmux.HandleFunc("/debug/pprof/", pprof.Index) //	Index responds to a request for "/debug/pprof" with an HTML page listing the available profiles.
		dmux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		dmux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		dmux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		dmux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		ds := http.Server{
			Addr:        *debugAddr,
			Handler:     dmux,
			ReadTimeout: 10 * time.Second,
		}
		defer ds.Shutdown(ctx)
		go func() {
			if err := ds.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				klog.ErrorS(err, "debug http server error")
			}
		}()
		klog.InfoS("debug server listening", "addr", *debugAddr)
	}

	<-ctx.Done() //	The <- operator represents the idea of passing a value from a channel to a reference
	klog.InfoS("terminating")
	g.GracefulStop()
}
