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
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/infra"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/server"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/exporters/metric/prometheus"
	"google.golang.org/grpc"
	jlogs "k8s.io/component-base/logs/json"
	"k8s.io/klog/v2"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

var (
	kubeconfig    = flag.String("kubeconfig", "", "absolute path to kubeconfig file")
	logFormatJSON = flag.Bool("log-format-json", true, "set log formatter to json")
	metricsAddr   = flag.String("metrics_addr", ":8095", "configure http listener for reporting metrics")
	enableProfile = flag.Bool("enable-pprof", false, "enable pprof profiling")
	debugAddr     = flag.String("debug_addr", "localhost:6060", "port for pprof profiling")

	version = "dev"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	if *logFormatJSON {
		klog.SetLogger(jlogs.JSONLogger)
	}

	flag.Parse()
	ctx := withShutdownSignal(context.Background())

	ua := fmt.Sprintf("secrets-store-csi-driver-provider-gcp/%s", version)
	klog.InfoS(fmt.Sprintf("starting %s", ua))

	// setup provider grpc server
	s := &server.Server{
		UA:         ua,
		Kubeconfig: *kubeconfig,
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

	g := grpc.NewServer(
		grpc.UnaryInterceptor(infra.LogInterceptor()),
	)
	v1alpha1.RegisterCSIDriverProviderServer(g, s)
	go g.Serve(l)

	// initialize metrics and health http server
	mux := http.NewServeMux()
	ms := http.Server{
		Addr:    *metricsAddr,
		Handler: mux,
	}
	defer ms.Shutdown(ctx)

	ex, err := prometheus.InstallNewPipeline(prometheus.Config{})
	if err != nil {
		klog.ErrorS(err, "unable to initialize prometheus exporter")
		klog.Fatalln("unable to initialize prometheus exporter")
	}
	if err := runtime.Start(runtime.WithMeterProvider(ex.MeterProvider())); err != nil {
		klog.ErrorS(err, "unable to start runtime metrics monitoring")
	}
	mux.HandleFunc("/metrics", ex.ServeHTTP)
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
			Addr:    *debugAddr,
			Handler: dmux,
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

// withShutdownSignal returns a copy of the parent context that will close if
// the process receives termination signals.
func withShutdownSignal(ctx context.Context) context.Context {
	nctx, cancel := context.WithCancel(ctx)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		klog.InfoS("received shutdown signal", "signal", sig)
		cancel()
	}()
	return nctx
}
