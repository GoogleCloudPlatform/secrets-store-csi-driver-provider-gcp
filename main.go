// Copyright 2025 Google LLC
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
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"cloud.google.com/go/compute/metadata"
	iam "cloud.google.com/go/iam/credentials/apiv1"
	parametermanager "cloud.google.com/go/parametermanager/apiv1"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/auth"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/infra"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/server"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/vars"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	logsapi "k8s.io/component-base/logs/api/v1"
	jlogs "k8s.io/component-base/logs/json"
	"k8s.io/klog/v2"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

var (
	kubeconfig            = flag.String("kubeconfig", "", "absolute path to kubeconfig file")
	logFormatJSON         = flag.Bool("log-format-json", true, "set log formatter to json")
	metricsAddr           = flag.String("metrics_addr", ":8095", "configure http listener for reporting metrics")
	enableProfile         = flag.Bool("enable-pprof", false, "enable pprof profiling")
	debugAddr             = flag.String("debug_addr", "localhost:6060", "port for pprof profiling")
	_                     = flag.Bool("write_secrets", false, "[unused]")
	smConnectionPoolSize  = flag.Int("sm_connection_pool_size", 5, "size of the connection pool for the secret manager API client")
	iamConnectionPoolSize = flag.Int("iam_connection_pool_size", 5, "size of the connection pool for the IAM API client")

	version = "dev"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	flag.Parse()

	if *logFormatJSON {
		jsonFactory := jlogs.Factory{}
		logger, _ := jsonFactory.Create(logsapi.LoggingConfiguration{Format: "json"}, logsapi.LoggingOptions{ErrorStream: os.Stderr, InfoStream: os.Stdout})
		klog.SetLogger(logger)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var err error
	uai, err := vars.UserAgentIdentifier.GetValue()
	if err != nil {
		klog.ErrorS(err, "failed to get user agent identifier")
		klog.Fatal("failed to get user agent identifier")
	}

	ua := fmt.Sprintf("%s/%s", uai, version)
	klog.InfoS(fmt.Sprintf("starting %s", ua))

	// Kubernetes Client
	var rc *rest.Config
	if *kubeconfig != "" {
		klog.V(5).InfoS("using kubeconfig", "path", *kubeconfig)
		rc, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	} else {
		klog.V(5).InfoS("using in-cluster kubeconfig")
		rc, err = rest.InClusterConfig()
	}
	if err != nil {
		klog.ErrorS(err, "failed to read kubeconfig")
		klog.Fatal("failed to read kubeconfig")
	}
	rc.ContentType = runtime.ContentTypeProtobuf

	clientset, err := kubernetes.NewForConfig(rc)
	if err != nil {
		klog.ErrorS(err, "failed to configure k8s client")
		klog.Fatal("failed to configure k8s client")
	}

	// Secret Manager client
	//
	// build without auth so that authentication can be re-added on a per-RPC
	// basis for each mount
	clientOptions := []option.ClientOption{
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
	smClientOptions := append(clientOptions, option.WithEndpoint("dns:///secretmanager.googleapis.com:443"))
	sc, err := secretmanager.NewClient(ctx, smClientOptions...)
	if err != nil {
		klog.ErrorS(err, "failed to create secretmanager client")
		klog.Fatal("failed to create secretmanager client")
	}

	pmClientOptions := append(clientOptions, option.WithEndpoint("dns:///parametermanager.googleapis.com:443"))
	pmClient, err := parametermanager.NewClient(ctx, pmClientOptions...)
	if err != nil {
		klog.ErrorS(err, "failed to create parametermanager client")
		klog.Fatal("failed to create parametermanager client")
	}

	// Used to store regional clients inside map
	regionalSmClientMap := make(map[string]*secretmanager.Client)

	// To cache the clients for parameter manager regional endpoints
	regionalPmClientMap := make(map[string]*parametermanager.Client)
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
	hc := &http.Client{
		Transport: &http.Transport{
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
		SecretClient:                    sc,
		ParameterManagerClient:          pmClient,
		AuthClient:                      c,
		RegionalSecretClients:           regionalSmClientMap,
		RegionalParameterManagerClients: regionalPmClientMap,
		ServerClientOptions:             clientOptions,
	}

	p, err := vars.ProviderName.GetValue()
	if err != nil {
		klog.ErrorS(err, "failed to get provider name")
		klog.Fatal("failed to get provider name")
	}
	socketPath := filepath.Join(os.Getenv("TARGET_DIR"), fmt.Sprintf("%s.sock", p))
	// Attempt to remove the UDS to handle cases where a previous execution was
	// killed before fully closing the socket listener and unlinking.
	_ = os.Remove(socketPath)

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		klog.ErrorS(err, "unable to listen to unix socket", "path", socketPath)
		klog.Fatalln("unable to start")
	}
	defer l.Close()

	g := grpc.NewServer(
		grpc.UnaryInterceptor(infra.LogInterceptor()),
	)
	v1alpha1.RegisterCSIDriverProviderServer(g, s)
	go g.Serve(l)

	// initialize metrics and health http server
	mux := http.NewServeMux()
	ms := http.Server{
		Addr:        *metricsAddr,
		Handler:     mux,
		ReadTimeout: 10 * time.Second,
	}
	defer ms.Shutdown(ctx)

	_, err = otelprom.New()
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
		dmux.HandleFunc("/debug/pprof/", pprof.Index)
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

	<-ctx.Done()
	klog.InfoS("terminating")
	g.GracefulStop()
}
