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
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/server"

	"google.golang.org/grpc"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "absolute path to kubeconfig file")

	version = "dev"
)

func main() {
	flag.Parse()
	ctx := withShutdownSignal(context.Background())

	ua := fmt.Sprintf("secrets-store-csi-driver-provider-gcp/%s", version)
	log.Printf("starting %s", ua)

	s := &server.Server{
		UA:         ua,
		Kubeconfig: *kubeconfig,
	}

	l, err := net.Listen("unix", path.Join(os.Getenv("TARGET_DIR"), "gcp.sock"))
	if err != nil {
		log.Fatalf("Unable to listen to unix socket: %s", err)
	}
	defer l.Close()

	g := grpc.NewServer()
	v1alpha1.RegisterCSIDriverProviderServer(g, s)
	go g.Serve(l)

	<-ctx.Done()
	log.Printf("terminating")
	g.GracefulStop()
}

// withShutdownSignal returns a copy of the parent context that will close if
// the process receives termination signals.
func withShutdownSignal(ctx context.Context) context.Context {
	nctx, cancel := context.WithCancel(ctx)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	go func() {
		sig := <-sigs
		log.Println("signal:", sig)
		cancel()
	}()
	return nctx
}
