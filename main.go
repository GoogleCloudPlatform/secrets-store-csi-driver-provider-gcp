package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var (
	daemonset = flag.Bool("daemonset", false, "Controls whether the plugin executes in the DaemonSet mode, copying itself to TARGET_DIR")

	attributes = flag.String("attributes", "", "Secrets volume attributes.")
	secrets    = flag.String("secrets", "", "Kubernetes secrets passed through the CSI driver node publish interface.")
	targetPath = flag.String("targetPath", "", "Path to where the secrets should be written")
	permission = flag.String("permission", "", "File permissions of the written secrets")
)

func main() {
	flag.Parse()
	// TODO: https://github.com/kubernetes-sigs/secrets-store-csi-driver does
	// not log plugin stderr output.
	log.SetOutput(os.Stdout)
	ctx := withShutdownSignal(context.Background())

	// This plugin and the github.com/kubernetes-sigs/secrets-store-csi-driver
	// driver are both installed as DaemonSets that share a common folder on
	// the host. When the "-daemonset" flag is set this binary copies itself
	// to the TARGET_DIR folder and sleeps indefinately.
	if *daemonset {
		if err := copyself(ctx); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	// Fetch and write secrets.
	if err := plugin(ctx); err != nil {
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

	pluginDir := filepath.Join(td, "gcp")
	pluginPath := filepath.Join(pluginDir, "provider-gcp")
	defer func() {
		// Attempt to cleanup since we are creating a folder and writing a
		// binary to a hostPath
		log.Printf("cleanup %v: %v", pluginPath, os.Remove(pluginPath))
		log.Printf("cleanup %v: %v", pluginDir, os.Remove(pluginDir))
	}()

	if err := os.MkdirAll(filepath.Join(td, "gcp"), 0755); err != nil {
		return err
	}

	self, err := ioutil.ReadFile(os.Args[0])
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(pluginPath, self, 0755); err != nil {
		return err
	}

	<-ctx.Done()
	log.Printf("terminating")
	return nil
}

func plugin(ctx context.Context) error {
	var attrib, secret map[string]string
	var filePermission os.FileMode

	// Everything in the "parameters" section of the SecretProviderClass.
	if err := json.Unmarshal([]byte(*attributes), &attrib); err != nil {
		return fmt.Errorf("failed to unmarshal attributes, err: %v", err)
	}

	// The secrets here are the relevant CSI driver (k8s) secrets. See
	// https://kubernetes-csi.github.io/docs/secrets-and-credentials-storage-class.html
	// Currently unused.
	if err := json.Unmarshal([]byte(*secrets), &secret); err != nil {
		return fmt.Errorf("failed to unmarshal secrets, err: %v", err)
	}

	// Permissions to apply to all files.
	if err := json.Unmarshal([]byte(*permission), &filePermission); err != nil {
		return fmt.Errorf("failed to unmarshal file permission, err: %v", err)
	}

	log.Printf("attributes: %v", attrib)
	log.Printf("secrets: %v", secret)
	log.Printf("filePermission: %v", filePermission)
	log.Printf("targetPath: %v", *targetPath)

	// TODO: actually fetch and write secrets.
	return nil
}
