package test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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
	tempDir           string
	gcpProviderBranch string
	testClusterName   string
	testSecretId      string
	kubeconfigFile    string
	testProjectId     string
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
	template = strings.ReplaceAll(template, "$PROJECT_ID", f.testProjectId)
	template = strings.ReplaceAll(template, "$CLUSTER_NAME", f.testClusterName)
	template = strings.ReplaceAll(template, "$TEST_SECRET_ID", f.testSecretId)
	template = strings.ReplaceAll(template, "$GCP_PROVIDER_SHA", f.gcpProviderBranch)
	template = strings.ReplaceAll(template, "$ZONE", zone)
	return ioutil.WriteFile(destFile, []byte(template), 0644)
}

// Executed before any tests are run. Setup is only run once for all tests in the suite.
func setupTestSuite() {
	rand.Seed(time.Now().UTC().UnixNano())

	f.gcpProviderBranch = os.Getenv("GCP_PROVIDER_SHA")
	if len(f.gcpProviderBranch) == 0 {
		log.Fatal("GCP_PROVIDER_SHA is empty")
	}
	f.testProjectId = os.Getenv("PROJECT_ID")
	if len(f.testProjectId) == 0 {
		log.Fatal("PROJECT_ID is empty")
	}

	tempDir, err := ioutil.TempDir("", "csi-tests")
	check(err)
	f.tempDir = tempDir
	f.testClusterName = fmt.Sprintf("testcluster-%d", rand.Int31())

	// Create test cluster
	clusterFile := filepath.Join(tempDir, "test-cluster.yaml")
	check(replaceTemplate("templates/test-cluster.yaml.tmpl", clusterFile))
	check(execCmd(exec.Command("kubectl", "apply", "-f", clusterFile)))
	check(execCmd(exec.Command("kubectl", "wait", "containercluster/"+f.testClusterName,
		"--for=condition=Ready", "--timeout", "5m")))

	// Get kubeconfig to use to authenticate to test cluster
	f.kubeconfigFile = filepath.Join(f.tempDir, "test-cluster-kubeconfig")
	gcloudCmd := exec.Command("gcloud", "container", "clusters", "get-credentials", f.testClusterName,
		"--zone", zone, "--project", f.testProjectId)
	gcloudCmd.Env = append(os.Environ(), "KUBECONFIG="+f.kubeconfigFile)
	check(execCmd(gcloudCmd))

	// Install Secret Store
	check(execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile, "--namespace", "default",
		"-f", "deploy/rbac-secretproviderclass.yaml",
		"-f", "deploy/csidriver.yaml",
		"-f", "deploy/secrets-store.csi.x-k8s.io_secretproviderclasses.yaml",
		"-f", "deploy/secrets-store.csi.x-k8s.io_secretproviderclasspodstatuses.yaml",
		"-f", "deploy/secrets-store-csi-driver.yaml",
	)))

	// Install GCP Plugin and Workload Identity bindings
	check(execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile, "--namespace", "default",
		"-f", "deploy/workload-id-binding.yaml",
		"-f", "deploy/provider-gcp-plugin.yaml")))

	// Create test secret
	f.testSecretId = fmt.Sprintf("testsecret-%d", rand.Int31())
	secretFile := filepath.Join(f.tempDir, "secretValue")
	check(ioutil.WriteFile(secretFile, []byte(f.testSecretId), 0644))
	check(execCmd(exec.Command("gcloud", "secrets", "create", f.testSecretId, "--replication-policy", "automatic",
		"--data-file", secretFile, "--project", f.testProjectId)))
}

// Executed after tests are run. Teardown is only run once for all tests in the suite.
func teardownTestSuite() {
	os.RemoveAll(f.tempDir)
	execCmd(exec.Command("kubectl", "delete", "containercluster", f.testClusterName))
	execCmd(exec.Command("gcloud", "secrets", "delete", f.testSecretId, "--project", f.testProjectId, "--quiet"))
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

// Execute a test job that writes the test secret to a configmap and verify that the
// secret value is correct.
func TestMountSecret(t *testing.T) {
	jobFile := filepath.Join(f.tempDir, "test-job.yaml")
	if err := replaceTemplate("templates/test-job.yaml.tmpl", jobFile); err != nil {
		t.Fatalf("Error replacing job template: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"--namespace", "default", "-f", jobFile)); err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "wait", "jobs/test-secret-mounter-job", "--for=condition=Complete",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default", "--timeout", "5m")); err != nil {
		t.Fatalf("Error waiting for job: %v", err)
	}

	var stdout, stderr bytes.Buffer
	command := exec.Command("kubectl", "get", "configmap", "secretmap", "--kubeconfig", f.kubeconfigFile,
		"-o", "json", "--namespace", "default")
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", string(stdout.Bytes()))
		fmt.Println("Stderr:", string(stderr.Bytes()))
		t.Fatalf("Could not get config map: %v", err)
	}
	var secretConfigMap map[string]interface{}
	json.Unmarshal([]byte(string(stdout.Bytes())), &secretConfigMap)

	secret, present := secretConfigMap["data"].(map[string]interface{})["csiSecret"].(string)
	if !present {
		t.Fatalf("CSI secret not found in config map: %v", secretConfigMap)
	}

	// Secret value is set to the secret ID
	decoded, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		t.Fatalf("Error decoding secret (%v): %v", secret, err)
	}
	if string(decoded) != f.testSecretId {
		t.Fatalf("Secret value is %v, want: %v", secret, f.testSecretId)
	}
}
