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
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
)

var (
	daemonset = flag.Bool("daemonset", false, "Controls whether the plugin executes in the DaemonSet mode, copying itself to TARGET_DIR")

	attributes = flag.String("attributes", "", "Secrets volume attributes.")
	secrets    = flag.String("secrets", "", "Kubernetes secrets passed through the CSI driver node publish interface.")
	targetPath = flag.String("targetPath", "", "Path to where the secrets should be written")
	permission = flag.Uint("permission", 700, "File permissions of the written secrets")
)

// The "provider" name in the "SecretProviderClass" CRD that this plugin
// operates on.
const providerName = "gcp"

func main() {
	flag.Parse()
	// TODO: https://github.com/kubernetes-sigs/secrets-store-csi-driver does
	// not log plugin stderr output.
	log.SetOutput(os.Stdout)
	ctx := withShutdownSignal(context.Background())

	// This plugin and the github.com/kubernetes-sigs/secrets-store-csi-driver
	// driver are both installed as DaemonSets that share a common folder on
	// the host. When the "-daemonset" flag is set this binary copies itself
	// to the TARGET_DIR folder and sleeps indefinitely.
	if *daemonset {
		if err := copyself(ctx); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to create secretmanager client: %v", err)
	}

	// Fetch and write secrets.
	if err := handleMountEvent(ctx, client, &mountParams{
		attributes:  *attributes,
		kubeSecrets: *secrets,
		targetPath:  *targetPath,
		permissions: os.FileMode(*permission),
	}); err != nil {
		log.Fatal(err)
	}
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

// copyself copies the current binary to the correct path in TARGET_DIR and
// then sleeps until the context done signal is received.
func copyself(ctx context.Context) error {
	td := os.Getenv("TARGET_DIR")
	if td == "" {
		return errors.New("TARGET_DIR not set")
	}
	if _, err := os.Stat(td); err != nil {
		return err
	}

	pluginDir := filepath.Join(td, providerName)
	pluginPath := filepath.Join(pluginDir, "provider-"+providerName)
	defer func() {
		// Attempt to cleanup since we are creating a folder and writing a
		// binary to a hostPath
		log.Printf("cleanup %v: %v", pluginDir, os.RemoveAll(pluginDir))
	}()

	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return err
	}

	self, err := ioutil.ReadFile(os.Args[0])
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(pluginPath, self, 0754); err != nil {
		return err
	}

	<-ctx.Done()
	log.Printf("terminating")
	return nil
}
