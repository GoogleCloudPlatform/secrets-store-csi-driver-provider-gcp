// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build secretmanager_e2e || all_e2e
// +build secretmanager_e2e all_e2e

package test

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Checks mounted secret content
func checkMountedSecret(secretId string) error {
	var stdout, stderr bytes.Buffer
	command := exec.Command("kubectl", "exec", "test-secret-mounter",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default",
		"--",
		"cat", fmt.Sprintf("/var/gcp-test-secrets/%s", secretId))
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		return fmt.Errorf("Could not read secret from container: %v", err)
	}
	if !bytes.Equal(stdout.Bytes(), []byte(secretId)) {
		return fmt.Errorf("Secret value is %v, want: %v", stdout.String(), secretId)
	}
	return nil
}

// Checks file mode of secrets
func checkFileMode(secretId string) error {
	var stdout, stderr bytes.Buffer
	command := exec.Command("kubectl", "exec", "test-secret-mode",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default",
		"--",
		"stat", "--printf", "%a", fmt.Sprintf("/var/gcp-test-secrets/..data/%s", secretId))
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		return fmt.Errorf("Could not read secret from container: %v", err)
	}
	if !bytes.Equal(stdout.Bytes(), []byte("400")) {
		return fmt.Errorf("Secret file mode is %v, want: %v", stdout.String(), "400")
	}
	return nil
}

// countGcloudVersions lists versions for a secret and returns the count or an error.
func countGcloudVersions(secretID, projectID, locationID string) (int, error) {
	args := []string{"secrets", "versions", "list", secretID, "--project", projectID, "--format=value(name)"}
	if locationID != "" {
		args = append(args, "--location", locationID)
	}

	cmd := exec.Command("gcloud", args...)
	// Log the command being executed
	fmt.Println("+", cmd.String())

	output, err := cmd.CombinedOutput()
	// Log the full output of the command for debugging
	logMessage := fmt.Sprintf("gcloud output for counting versions of secret '%s' (location: '%s'):\n%s", secretID, locationID, string(output))
	fmt.Println(logMessage)

	if err != nil {
		return 0, fmt.Errorf("error listing versions for %s (location: %s): %w. Output: %s", secretID, locationID, err, string(output))
	}

	trimmedOutput := strings.TrimSpace(string(output))
	if trimmedOutput == "" {
		return 0, nil // No versions found, no error from gcloud
	}
	return len(strings.Split(trimmedOutput, "\n")), nil
}

// waitForMinVersions polls until the specified secret has at least minVersions or a timeout is reached.
func waitForMinVersions(t *testing.T, secretID, projectID, locationID string, minVersions int, timeout time.Duration) {
	t.Helper()
	startTime := time.Now()
	var lastErr error
	for {
		if time.Since(startTime) > timeout {
			t.Fatalf("Timeout waiting for secret %s (location: %s) to have at least %d versions. Last error: %v", secretID, locationID, minVersions, lastErr)
		}

		count, err := countGcloudVersions(secretID, projectID, locationID)
		lastErr = err // Store the last error for the timeout message

		if err == nil && count >= minVersions {
			t.Logf("Secret %s (location: %s) now has %d version(s).", secretID, locationID, count)
			return
		}
		t.Logf("Secret %s (location: %s) has %d/%d versions. Error (if any): %v. Retrying in 5s...", secretID, locationID, count, minVersions, err)
		time.Sleep(5 * time.Second) // Poll interval
	}
}

func setupSmTestSuite() {

	f.testSecretID = fmt.Sprintf("testsecret-%d", rand.Int31())

	f.testRotateSecretID = f.testSecretID + "-rotate"
	f.testExtractSecretID = f.testSecretID + "-extract"

	// Create test secret
	secretFile := filepath.Join(f.tempDir, "secretValue")
	check(os.WriteFile(secretFile, []byte(f.testSecretID), 0644))
	check(execCmd(exec.Command("gcloud", "secrets", "create", f.testSecretID, "--replication-policy", "automatic",
		"--data-file", secretFile, "--project", f.testProjectID)))

	// Create regional secret
	secretFile = filepath.Join(f.tempDir, "regionalSecretValue")
	check(os.WriteFile(secretFile, []byte(f.testSecretID+"-regional"), 0644))

	// Setting endpoint to regional one (us-central1)
	check(execCmd(exec.Command("gcloud", "config", "set", "api_endpoint_overrides/secretmanager",
		"https://secretmanager."+f.location+".rep.googleapis.com/")))
	check(execCmd(exec.Command("gcloud", "secrets", "create", f.testSecretID, "--location", f.location,
		"--data-file", secretFile, "--project", f.testProjectID)))

	// Setting endpoints back to the global defaults
	check(execCmd(exec.Command("gcloud", "config", "unset", "api_endpoint_overrides/secretmanager")))
}

func teardownSmTestSuite() {
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
	execCmd(exec.Command(
		"gcloud", "secrets", "delete", f.testExtractSecretID,
		"--project", f.testProjectID,
		"--quiet",
	))

	// Cleanup regional secret
	check(execCmd(exec.Command("gcloud", "config", "set", "api_endpoint_overrides/secretmanager",
		"https://secretmanager."+f.location+".rep.googleapis.com/")))
	execCmd(exec.Command("gcloud", "secrets", "delete", f.testSecretID, "--location", f.location,
		"--project", f.testProjectID, "--quiet"))
	execCmd(exec.Command("gcloud", "secrets", "delete", f.testRotateSecretID, "--location", f.location,
		"--project", f.testProjectID, "--quiet"))
	check(execCmd(exec.Command("gcloud", "config", "unset", "api_endpoint_overrides/secretmanager")))
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

	if err := checkMountedSecret(f.testSecretID); err != nil {
		t.Fatalf("Error while testing global secret: %v", err)
	}
	if err := checkMountedSecret(f.testSecretID + "-regional"); err != nil {
		t.Fatalf("Error while testing regional secret: %v", err)
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
	if err := checkFileMode(f.testSecretID); err != nil {
		t.Fatalf("Error while testing global secret: %v", err)
	}
	if err := checkFileMode(f.testSecretID + "-regional"); err != nil {
		t.Fatalf("Error while testing regional secret: %v", err)
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

	// TODO: Add checks for regional secret -> Present from before (not added by me)
}

func TestMountRotateSecret(t *testing.T) {
	secretA := []byte("secreta")
	secretB := []byte("secretb")

	// Enable rotation.
	check(execCmd(exec.Command("enable-rotation.sh", f.kubeconfigFile)))

	// Wait for deployment to finish.
	time.Sleep(3 * time.Minute)

	var stdout, stderr bytes.Buffer
	err := execCmd(exec.Command("kubectl", "get", "daemonset", "csi-secrets-store",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "kube-system"))
	if err != nil {
		t.Logf("Could not get daemonset status: %v", err)
	} else {
		t.Logf("Daemonset status after editing: %v", stdout.String())
	}

	// Create test secret.
	secretFileA := filepath.Join(f.tempDir, "secretValue-A")
	check(os.WriteFile(secretFileA, secretA, 0644))
	check(execCmd(exec.Command(
		"gcloud", "secrets", "create", f.testRotateSecretID,
		"--replication-policy", "automatic",
		"--data-file", secretFileA,
		"--project", f.testProjectID,
	)))

	// create a regional test secret
	check(execCmd(exec.Command("gcloud", "config", "set", "api_endpoint_overrides/secretmanager",
		"https://secretmanager."+f.location+".rep.googleapis.com/")))

	check(execCmd(exec.Command(
		"gcloud", "secrets", "create", f.testRotateSecretID,
		"--location", f.location,
		"--data-file", secretFileA,
		"--project", f.testProjectID,
	)))
	check(execCmd(exec.Command("gcloud", "config", "unset", "api_endpoint_overrides/secretmanager")))

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

	stdout.Reset()
	stderr.Reset()
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
	check(os.WriteFile(secretFileB, secretB, 0644))
	check(execCmd(exec.Command(
		"gcloud", "secrets", "versions", "add", f.testRotateSecretID,
		"--data-file", secretFileB,
		"--project", f.testProjectID,
	)))

	// Rotate regional secret
	check(execCmd(exec.Command("gcloud", "config", "set", "api_endpoint_overrides/secretmanager",
		"https://secretmanager."+f.location+".rep.googleapis.com/")))
	check(execCmd(exec.Command(
		"gcloud", "secrets", "versions", "add", f.testRotateSecretID,
		"--data-file", secretFileB,
		"--project", f.testProjectID,
		"--location", f.location,
	)))

	check(execCmd(exec.Command("gcloud", "config", "unset", "api_endpoint_overrides/secretmanager")))

	// Wait for the global secret to have 2 versions.
	waitForMinVersions(t, f.testRotateSecretID, f.testProjectID, "" /* global */, 2, 180*time.Second)

	time.Sleep(150 * time.Second)

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

// Execute a test job that mounts a extract secret and checks that the value is correct.
func TestMountExtractSecret(t *testing.T) {
	secretData := []byte(`{"user":"admin", "password":"password@1234"}`)

	// Create test secret
	secretFile := filepath.Join(f.tempDir, "secretExtractValue")
	check(os.WriteFile(secretFile, secretData, 0644))
	check(execCmd(exec.Command(
		"gcloud", "secrets", "create", f.testExtractSecretID,
		"--replication-policy", "automatic",
		"--data-file", secretFile,
		"--project", f.testProjectID,
	)))

	podFile := filepath.Join(f.tempDir, "test-extract-key.yaml")
	if err := replaceTemplate("templates/test-extract-key.yaml.tmpl", podFile); err != nil {
		t.Fatalf("Error replacing pod template: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"--namespace", "default", "-f", podFile)); err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "wait", "pod/test-secret-mounter-extract", "--for=condition=Ready",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default", "--timeout", "5m")); err != nil {
		t.Fatalf("Error waiting for job: %v", err)
	}
	testExtractSecret := []byte("admin")

	// Check Mounted Secrets
	var stdout, stderr bytes.Buffer
	command := exec.Command(
		"kubectl", "exec", "test-secret-mounter-extract",
		"--kubeconfig", f.kubeconfigFile,
		"--namespace", "default",
		"--",
		"cat", "/var/gcp-test-secrets/extract")
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		t.Fatalf("Could not read secret from container: %v", err)
	}
	if got := stdout.Bytes(); !bytes.Equal(got, testExtractSecret) {
		t.Fatalf("Secret value is %v, want: %v", got, testExtractSecret)
	}
}
