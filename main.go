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
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path"

	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/server"

	"google.golang.org/grpc"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

var (
	daemonset  = flag.Bool("daemonset", false, "Controls whether the plugin executes in the DaemonSet mode, copying itself to TARGET_DIR")
	kubeconfig = flag.String("kubeconfig", "", "absolute path to kubeconfig file")

	version = "dev"
)

func main() {
	flag.Parse()

	ua := fmt.Sprintf("secrets-store-csi-driver-provider-gcp/%s", version)
	log.Printf("starting %s", ua)

	if !*daemonset {
		// no-op if exec'd without -daemonset
		os.Exit(0)
	}

	s := &server.Server{
		UA:         ua,
		Kubeconfig: *kubeconfig,
	}

	l, err := net.Listen("unix", path.Join(os.Getenv("TARGET_DIR"), "gcp.sock"))
	defer l.Close()

	if err != nil {
		log.Fatalf("Unable to listen to unix socket: %s", err)
	}
	g := grpc.NewServer()
	v1alpha1.RegisterCSIDriverProviderServer(g, s)
	g.Serve(l)
}
