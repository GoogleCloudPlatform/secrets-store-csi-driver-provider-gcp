// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// zone to set up test cluster in
const zone = "us-central1-c"

type testFixture struct {
	tempDir            string
	gcpProviderBranch  string
	testClusterName    string
	testSecretID       string
	testRotateSecretID string
	kubeconfigFile     string
	testProjectID      string
	secretStoreVersion string
	gkeVersion         string
}

var f testFixture

// Panics with the provided error if it is not nil.
func check(err error) {
	if err != nil {
		panic(err)
	}
}

// Prints and executes a command.
func execCmd(command *exec.Cmd) error {
	fmt.Println("+", command)
	stdoutStderr, err := command.CombinedOutput()
	fmt.Println(string(stdoutStderr))
	return err
}

// Replaces variables in an input template file and writes the result to an
// output file.
func replaceTemplate(templateFile string, destFile string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	templateBytes, err := ioutil.ReadFile(filepath.Join(pwd, templateFile))
	if err != nil {
		return err
	}
	template := string(templateBytes)
	template = strings.ReplaceAll(template, "$PROJECT_ID", f.testProjectID)
	template = strings.ReplaceAll(template, "$CLUSTER_NAME", f.testClusterName)
	template = strings.ReplaceAll(template, "$TEST_SECRET_ID", f.testSecretID)
	template = strings.ReplaceAll(template, "$GCP_PROVIDER_SHA", f.gcpProviderBranch)
	template = strings.ReplaceAll(template, "$ZONE", zone)
	template = strings.ReplaceAll(template, "$GKE_VERSION", f.gkeVersion)
	return ioutil.WriteFile(destFile, []byte(template), 0644)
}

// Executed before any tests are run. Setup is only run once for all tests in the suite.
func setupTestSuite() {
	rand.Seed(time.Now().UTC().UnixNano())

	f.gcpProviderBranch = os.Getenv("GCP_PROVIDER_SHA")
	if len(f.gcpProviderBranch) == 0 {
		log.Fatal("GCP_PROVIDER_SHA is empty")
	}
	f.testProjectID = os.Getenv("PROJECT_ID")
	if len(f.testProjectID) == 0 {
		log.Fatal("PROJECT_ID is empty")
	}
	f.secretStoreVersion = os.Getenv("SECRET_STORE_VERSION")
	if len(f.secretStoreVersion) == 0 {
		log.Println("SECRET_STORE_VERSION is empty, defaulting to 'main'")
		f.secretStoreVersion = "main"
	}
	// Version of the GKE cluster to run the tests on
	// spec.releaseChannel.channel from:
	// https://cloud.google.com/config-connector/docs/reference/resource-docs/container/containercluster
	f.gkeVersion = os.Getenv("GKE_VERSION")
	switch f.gkeVersion {
	case "STABLE":
	case "REGULAR":
	case "RAPID":
		break
	default:
		log.Printf("GKE_VERSION is invalid (%q), defaulting to 'STABLE'", f.gkeVersion)
		f.gkeVersion = "STABLE"
	}

	tempDir, err := ioutil.TempDir("", "csi-tests")
	check(err)
	f.tempDir = tempDir
	f.testClusterName = fmt.Sprintf("testcluster-%d", rand.Int31())
	f.testSecretID = fmt.Sprintf("testsecret-%d", rand.Int31())
	f.testRotateSecretID = f.testSecretID + "-rotate"

	// Build the plugin deploy yaml
	pluginFile := filepath.Join(tempDir, "provider-gcp-plugin.yaml")
	check(replaceTemplate("templates/provider-gcp-plugin.yaml.tmpl", pluginFile))

	// Create test cluster
	clusterFile := filepath.Join(tempDir, "test-cluster.yaml")
	check(replaceTemplate("templates/test-cluster.yaml.tmpl", clusterFile))
	check(execCmd(exec.Command("kubectl", "apply", "-f", clusterFile)))
	check(execCmd(exec.Command("kubectl", "wait", "containercluster/"+f.testClusterName,
		"--for=condition=Ready", "--timeout", "15m")))

	// Get kubeconfig to use to authenticate to test cluster
	f.kubeconfigFile = filepath.Join(f.tempDir, "test-cluster-kubeconfig")
	gcloudCmd := exec.Command("gcloud", "container", "clusters", "get-credentials", f.testClusterName,
		"--zone", zone, "--project", f.testProjectID)
	gcloudCmd.Env = append(os.Environ(), "KUBECONFIG="+f.kubeconfigFile)
	check(execCmd(gcloudCmd))

	// Install Secret Store
	check(execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"-f", fmt.Sprintf("https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/%s/deploy/rbac-secretproviderclass.yaml", f.secretStoreVersion),
		"-f", fmt.Sprintf("https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/%s/deploy/rbac-secretprovidersyncing.yaml", f.secretStoreVersion),
		"-f", fmt.Sprintf("https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/%s/deploy/csidriver.yaml", f.secretStoreVersion),
		"-f", fmt.Sprintf("https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/%s/deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml", f.secretStoreVersion),
		"-f", fmt.Sprintf("https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/%s/deploy/secrets-store.csi.x-k8s.io_secretproviderclasspodstatuses.yaml", f.secretStoreVersion),
		"-f", fmt.Sprintf("https://raw.githubusercontent.com/kubernetes-sigs/secrets-store-csi-driver/%s/deploy/secrets-store-csi-driver.yaml", f.secretStoreVersion),
	)))

	// Install GCP Plugin and Workload Identity bindings
	check(execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"-f", pluginFile)))

	// Create test secret
	secretFile := filepath.Join(f.tempDir, "secretValue")
	check(ioutil.WriteFile(secretFile, []byte(f.testSecretID), 0644))
	check(execCmd(exec.Command("gcloud", "secrets", "create", f.testSecretID, "--replication-policy", "automatic",
		"--data-file", secretFile, "--project", f.testProjectID)))
}

// Executed after tests are run. Teardown is only run once for all tests in the suite.
func teardownTestSuite() {
	// print cluster information, useful when debugging
	execCmd(exec.Command(
		"kubectl", "describe", "pods",
		"--all-namespaces",
		"--kubeconfig", f.kubeconfigFile,
	))
	execCmd(exec.Command(
		"kubectl", "logs", "-l", "app=csi-secrets-store",
		"--tail", "-1",
		"-n", "kube-system",
		"--kubeconfig", f.kubeconfigFile,
	))
	execCmd(exec.Command(
		"kubectl", "logs", "-l", "app=csi-secrets-store-provider-gcp",
		"--tail", "-1",
		"-n", "kube-system",
		"--kubeconfig", f.kubeconfigFile,
	))

	// cleanup
	os.RemoveAll(f.tempDir)
	execCmd(exec.Command("kubectl", "delete", "containercluster", f.testClusterName))
	execCmd(exec.Command(
		"gcloud", "secrets", "delete", f.testSecretID,
		"--project", f.testProjectID,
		"--quiet",
	))
	execCmd(exec.Command(
		"gcloud", "secrets", "delete", f.testRotateSecretID,
		"--project", f.testProjectID,
		"--quiet",
	))
}

// Entry point for go test.
func TestMain(m *testing.M) {
	os.Exit(runTest(m))
}

// Handles setup/teardown test suite and runs test. Returns exit code.
func runTest(m *testing.M) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Test execution panic:", r)
			code = 1
		}
		teardownTestSuite()
	}()

	setupTestSuite()
	return m.Run()
}

// Execute a test job that mounts a secret and checks that the value is correct.
func TestMountSecret(t *testing.T) {
	podFile := filepath.Join(f.tempDir, "test-pod.yaml")
	if err := replaceTemplate("templates/test-pod.yaml.tmpl", podFile); err != nil {
		t.Fatalf("Error replacing pod template: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"--namespace", "default", "-f", podFile)); err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	// As a workaround for https://github.com/kubernetes/kubernetes/issues/83242, we sleep to
	// ensure that the job resources exists before attempting to wait for it.
	time.Sleep(5 * time.Second)
	if err := execCmd(exec.Command("kubectl", "wait", "pod/test-secret-mounter", "--for=condition=Ready",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default", "--timeout", "5m")); err != nil {
		t.Fatalf("Error waiting for job: %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := exec.Command("kubectl", "exec", "test-secret-mounter",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default",
		"--",
		"cat", fmt.Sprintf("/var/gcp-test-secrets/%s", f.testSecretID))
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		t.Fatalf("Could not read secret from container: %v", err)
	}
	if !bytes.Equal(stdout.Bytes(), []byte(f.testSecretID)) {
		t.Fatalf("Secret value is %v, want: %v", stdout.String(), f.testSecretID)
	}
}

func TestMountSecretFileMode(t *testing.T) {
	podFile := filepath.Join(f.tempDir, "test-mode.yaml")
	if err := replaceTemplate("templates/test-mode.yaml.tmpl", podFile); err != nil {
		t.Fatalf("Error replacing mode template: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"--namespace", "default", "-f", podFile)); err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	// As a workaround for https://github.com/kubernetes/kubernetes/issues/83242, we sleep to
	// ensure that the job resources exists before attempting to wait for it.
	time.Sleep(5 * time.Second)
	if err := execCmd(exec.Command("kubectl", "wait", "pod/test-secret-mode", "--for=condition=Ready",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default", "--timeout", "5m")); err != nil {
		t.Fatalf("Error waiting for job: %v", err)
	}

	// stat the file in the symlinked '..data' directory, symlink will always return 777 otherwise
	var stdout, stderr bytes.Buffer
	command := exec.Command("kubectl", "exec", "test-secret-mode",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default",
		"--",
		"stat", "--printf", "%a", fmt.Sprintf("/var/gcp-test-secrets/..data/%s", f.testSecretID))
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		t.Fatalf("Could not read secret from container: %v", err)
	}
	if !bytes.Equal(stdout.Bytes(), []byte("400")) {
		t.Fatalf("Secret file mode is %v, want: %v", stdout.String(), "400")
	}
}

func TestMountNestedPath(t *testing.T) {
	podFile := filepath.Join(f.tempDir, "test-nested.yaml")
	if err := replaceTemplate("templates/test-nested.yaml.tmpl", podFile); err != nil {
		t.Fatalf("Error replacing pod template: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"--namespace", "default", "-f", podFile)); err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	// As a workaround for https://github.com/kubernetes/kubernetes/issues/83242, we sleep to
	// ensure that the job resources exists before attempting to wait for it.
	time.Sleep(5 * time.Second)
	if err := execCmd(exec.Command("kubectl", "wait", "pod/test-secret-nested", "--for=condition=Ready",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default", "--timeout", "5m")); err != nil {
		t.Fatalf("Error waiting for job: %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := exec.Command("kubectl", "exec", "test-secret-nested",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default",
		"--",
		"cat", fmt.Sprintf("/var/gcp-test-secrets/my/nested/path/%s", f.testSecretID))
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		t.Fatalf("Could not read secret from container: %v", err)
	}
	if !bytes.Equal(stdout.Bytes(), []byte(f.testSecretID)) {
		t.Fatalf("Secret value is %v, want: %v", stdout.String(), f.testSecretID)
	}
}

func TestMountInvalidPath(t *testing.T) {
	podFile := filepath.Join(f.tempDir, "test-invalid.yaml")
	if err := replaceTemplate("templates/test-invalid.yaml.tmpl", podFile); err != nil {
		t.Fatalf("Error replacing pod template: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"--namespace", "default", "-f", podFile)); err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	// We cannot use a 'wait for condition' since we are expecting a failure (that gets retried indefinitely).
	// Instead wait for enough time to give the kubelet a chance to attempt the mount and have it fail.
	time.Sleep(15 * time.Second)

	var stdout, stderr bytes.Buffer
	command := exec.Command("kubectl", "get", "events", "--field-selector", "involvedObject.name=test-secret-invalid",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default")
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		t.Fatalf("Could not read secret from container: %v", err)
	}
	if !strings.Contains(stdout.String(), "invalid path") {
		t.Fatalf("Unable to find 'invalid path' error: %v", stdout.String())
	}
}

func TestMountSyncSecret(t *testing.T) {
	podFile := filepath.Join(f.tempDir, "test-sync.yaml")
	if err := replaceTemplate("templates/test-sync.yaml.tmpl", podFile); err != nil {
		t.Fatalf("Error replacing pod template: %v", err)
	}

	if err := execCmd(exec.Command(
		"kubectl", "apply", "-f", podFile,
		"--kubeconfig", f.kubeconfigFile,
		"--namespace", "default",
	)); err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	// As a workaround for https://github.com/kubernetes/kubernetes/issues/83242, we sleep to
	// ensure that the job resources exists before attempting to wait for it.
	time.Sleep(5 * time.Second)
	if err := execCmd(exec.Command(
		"kubectl", "wait", "pod/test-secret-mounter-sync",
		"--for=condition=Ready",
		"--kubeconfig", f.kubeconfigFile,
		"--namespace", "default",
		"--timeout", "5m",
	)); err != nil {
		t.Fatalf("Error waiting for job: %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := exec.Command(
		"kubectl", "exec", "test-secret-mounter-sync",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default",
		"--",
		"printenv",
	)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		t.Fatalf("Could not read secret from container: %v", err)
	}
	if got := stdout.Bytes(); !bytes.Contains(got, []byte(f.testSecretID)) {
		t.Fatalf("pod env value is %s, does not contain: %s", string(got), f.testSecretID)
	}
}

func TestMountRotateSecret(t *testing.T) {
	secretA := []byte("secreta")
	secretB := []byte("secretb")

	// Enable rotation.
	check(execCmd(exec.Command("enable-rotation.sh", f.kubeconfigFile)))

	// Wait for deployment to finish.
	time.Sleep(45 * time.Second)

	// Create test secret.
	secretFileA := filepath.Join(f.tempDir, "secretValue-A")
	check(ioutil.WriteFile(secretFileA, secretA, 0644))
	check(execCmd(exec.Command(
		"gcloud", "secrets", "create", f.testRotateSecretID,
		"--replication-policy", "automatic",
		"--data-file", secretFileA,
		"--project", f.testProjectID,
	)))

	// Deploy the test pod.
	podFile := filepath.Join(f.tempDir, "test-rotate.yaml")
	if err := replaceTemplate("templates/test-rotate.yaml.tmpl", podFile); err != nil {
		t.Fatalf("Error replacing pod template: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"--namespace", "default", "-f", podFile)); err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	// As a workaround for https://github.com/kubernetes/kubernetes/issues/83242, we sleep to
	// ensure that the job resources exists before attempting to wait for it.
	time.Sleep(5 * time.Second)
	if err := execCmd(exec.Command(
		"kubectl", "wait", "pod/test-secret-mounter-rotate",
		"--for=condition=Ready",
		"--kubeconfig", f.kubeconfigFile,
		"--namespace", "default",
		"--timeout", "5m",
	)); err != nil {
		t.Fatalf("Error waiting for job: %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := exec.Command(
		"kubectl", "exec", "test-secret-mounter-rotate",
		"--kubeconfig", f.kubeconfigFile,
		"--namespace", "default",
		"--",
		"cat", "/var/gcp-test-secrets/rotate")
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		t.Fatalf("Could not read secret from container: %v", err)
	}
	if got := stdout.Bytes(); !bytes.Equal(got, secretA) {
		t.Fatalf("Secret value is %v, want: %v", got, secretA)
	}

	// Rotate the secret.
	secretFileB := filepath.Join(f.tempDir, "secretValue-B")
	check(ioutil.WriteFile(secretFileB, secretB, 0644))
	check(execCmd(exec.Command(
		"gcloud", "secrets", "versions", "add", f.testRotateSecretID,
		"--data-file", secretFileB,
		"--project", f.testProjectID,
	)))

	// Wait for update. Keep in sync with driver's --rotation-poll-interval to
	// ensure enough time.
	time.Sleep(30 * time.Second)

	// Verify update.
	stdout.Reset()
	stderr.Reset()
	command = exec.Command(
		"kubectl", "exec", "test-secret-mounter-rotate",
		"--kubeconfig", f.kubeconfigFile,
		"--namespace", "default",
		"--",
		"cat", "/var/gcp-test-secrets/rotate",
	)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		t.Fatalf("Could not read secret from container: %v", err)
	}
	if got := stdout.Bytes(); !bytes.Equal(got, secretB) {
		t.Fatalf("Secret value is %v, want: %v", got, secretB)
	}
}
