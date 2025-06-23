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

//go:build secretmanager_e2e || parametermanager_e2e || all_e2e
// +build secretmanager_e2e parametermanager_e2e all_e2e

package test

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// zone to set up test cluster in
const zone = "us-central1-c"

type testFixture struct {
	tempDir             string
	gcpProviderBranch   string
	testClusterName     string
	testSecretID        string
	testRotateSecretID  string
	testExtractSecretID string
	kubeconfigFile      string
	testProjectID       string
	secretStoreVersion  string
	gkeVersion          string
	location            string

	// below fields explicitly used for parameter manager
	pmReferenceGlobalSecret1       string
	pmReferenceGlobalSecret2       string
	pmReferenceRegionalSecret1     string
	pmReferenceRegionalSecret2     string
	parameterIdYaml                string
	parameterIdJson                string
	parameterVersionIdYAML         string
	parameterVersionIdJSON         string
	regionalParameterIdYAML        string
	regionalParameterIdJSON        string
	regionalParameterVersionIdYAML string
	regionalParameterVersionIdJSON string
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
	outputStr := string(stdoutStderr)
	fmt.Println(outputStr) // Always print output for visibility
	if err != nil {
		log.Printf("Command failed: %v\nOutput:\n%s", err, outputStr) // Log error and output on failure
	}
	return err
}

// Replaces variables in an input template file and writes the result to an
// output file.
func replaceTemplate(templateFile string, destFile string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	templateBytes, err := os.ReadFile(filepath.Join(pwd, templateFile))
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
	template = strings.ReplaceAll(template, "$LOCATION_ID", f.location)

	template = strings.ReplaceAll(template, "$TEST_PARAMETER_ID_YAML", f.parameterIdYaml)
	template = strings.ReplaceAll(template, "$TEST_PARAMETER_ID_JSON", f.parameterIdJson)
	template = strings.ReplaceAll(template, "$TEST_VERSION_ID_YAML", f.parameterVersionIdYAML)
	template = strings.ReplaceAll(template, "$TEST_VERSION_ID_JSON", f.parameterVersionIdJSON)
	template = strings.ReplaceAll(template, "$TEST_REGIONAL_PARAMETER_ID_YAML", f.regionalParameterIdYAML)
	template = strings.ReplaceAll(template, "$TEST_REGIONAL_PARAMETER_ID_JSON", f.regionalParameterIdJSON)
	template = strings.ReplaceAll(template, "$TEST_REGIONAL_VERSION_ID_YAML", f.regionalParameterVersionIdYAML)
	template = strings.ReplaceAll(template, "$TEST_REGIONAL_VERSION_ID_JSON", f.regionalParameterVersionIdJSON)
	return os.WriteFile(destFile, []byte(template), 0644)
}

// Executed before any tests are run. Setup is only run once for all tests in the suite.
func setupTestSuite(isTokenPassed bool, suiteType string) {

	rand.Seed(time.Now().UTC().UnixNano())

	f.gcpProviderBranch = os.Getenv("GCP_PROVIDER_SHA")
	if len(f.gcpProviderBranch) == 0 {
		log.Fatal("GCP_PROVIDER_SHA is empty")
	}
	f.testProjectID = os.Getenv("PROJECT_ID")
	if len(f.testProjectID) == 0 {
		log.Fatal("PROJECT_ID is empty")
	}
	f.location = os.Getenv("LOCATION_ID")
	if len(f.location) == 0 {
		log.Fatal("LOCATION_ID is empty")
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

	tempDir, err := os.MkdirTemp("", "csi-tests")
	check(err)
	f.tempDir = tempDir
	f.testClusterName = fmt.Sprintf("testcluster-%d", rand.Int31())

	// Build the plugin deploy yaml
	pluginFile := filepath.Join(tempDir, "provider-gcp-plugin.yaml")
	check(replaceTemplate("templates/provider-gcp-plugin.yaml.tmpl", pluginFile))

	// Create test cluster
	clusterFile := filepath.Join(tempDir, "test-cluster.yaml")
	check(replaceTemplate("templates/test-cluster.yaml.tmpl", clusterFile))
	check(execCmd(exec.Command("kubectl", "apply", "-f", clusterFile)))
	check(execCmd(exec.Command("kubectl", "wait", "containercluster/"+f.testClusterName,
		"--for=condition=Ready", "--timeout", "30m")))

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

	// Create f.testSecretID (for direct SM tests) if secretmanager or all suite is run
	if suiteType == "secretmanager" || suiteType == "all" {
		setupSmTestSuite()
	}

	if suiteType == "parametermanager" || suiteType == "all" {
		setupPmTestSuite()
	}

	if isTokenPassed {
		type metadataStruct struct {
			Name string `yaml:"name"`
		}

		type audienceStruct struct {
			Audience string `yaml:"audience"`
		}

		type specStruct struct {
			PodInfoOnMount       bool             `yaml:"podInfoOnMount"`
			AttachRequired       bool             `yaml:"attachRequired"`
			VolumeLifecycleModes []string         `yaml:"volumeLifecycleModes"`
			TokenRequests        []audienceStruct `yaml:"tokenRequests"`
			RequiredRepublish    bool             `yaml:"requiresRepublish"`
		}

		type driver struct {
			ApiVersion string         `yaml:"apiVersion"`
			Kind       string         `yaml:"kind"`
			Metadata   metadataStruct `yaml:"metadata"`
			Spec       specStruct     `yaml:"spec"`
		}

		aud := audienceStruct{
			Audience: "secretmanager-csi-build.svc.id.goog", //	audience value is set as idPool for GCP project secretmanager-csi-build
		}

		csiDriver := driver{
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "CSIDriver",
			Metadata: metadataStruct{
				Name: "secrets-store.csi.k8s.io",
			},
			Spec: specStruct{
				PodInfoOnMount:       true,
				AttachRequired:       false,
				VolumeLifecycleModes: []string{"Ephemeral"},
				TokenRequests:        []audienceStruct{aud},
				RequiredRepublish:    true,
			},
		}

		yamlData, err := yaml.Marshal(&csiDriver)

		if err != nil {
			fmt.Printf("Error while Marshaling YAML file: %v", err)
		}

		fileName := "csidriver.yaml"
		err = os.WriteFile(fileName, yamlData, 0644)
		if err != nil {
			panic("Unable to create YAML file")
		}

		// set tokenRequests in driver spec and reinstall driver to perform E2E testing when token is received by provider from driver
		check(execCmd(exec.Command("kubectl", "apply",
			"-f", fileName,
		)))
	}
}

// Executed after tests are run. Teardown is only run once for all tests in the suite.
func teardownTestSuite(suiteType string) {
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

	// Cleanup
	os.RemoveAll(f.tempDir)
	execCmd(exec.Command("kubectl", "delete", "containercluster", f.testClusterName))

	if suiteType == "secretmanager" || suiteType == "all" {
		teardownSmTestSuite()
	}

	if suiteType == "parametermanager" || suiteType == "all" {
		teardownPmTestSuite()
	}
}

// Entry point for go test.
func TestMain(m *testing.M) {
	envSuiteType := os.Getenv("E2E_TEST_SUITE")
	if envSuiteType == "" {
		log.Println("E2E_TEST_SUITE environment variable not set, defaulting to 'all'.")
		envSuiteType = "all"
	}
	log.Printf("E2E_TEST_SUITE is '%s'. This will determine which test sequences (setup/teardown pairs) are run.\n", envSuiteType)
	log.Println("The actual tests executed by m.Run() within each sequence are determined by build tags.")

	var exitCode int

	if envSuiteType == "secretmanager" || envSuiteType == "all" {
		log.Println("Executing Secret Manager test runs...")
		// Pass "secretmanager" to runTest, which setupTestSuite/teardownTestSuite will use.
		smWithoutTokenStatus := runTest(m, false, "secretmanager")
		smWithTokenStatus := runTest(m, true, "secretmanager")
		fmt.Printf("Secret Manager Tests -> No Token Exit Code: %v, With Token Exit Code: %v\n", smWithoutTokenStatus, smWithTokenStatus)
		exitCode |= smWithoutTokenStatus | smWithTokenStatus
	}

	if envSuiteType == "parametermanager" || envSuiteType == "all" {
		log.Println("Executing Parameter Manager test runs...")
		// Pass "parametermanager" to runTest.
		pmWithoutTokenStatus := runTest(m, false, "parametermanager")
		pmWithTokenStatus := runTest(m, true, "parametermanager")
		fmt.Printf("Parameter Manager Tests -> No Token Exit Code: %v, With Token Exit Code: %v\n", pmWithoutTokenStatus, pmWithTokenStatus)
		exitCode |= pmWithoutTokenStatus | pmWithTokenStatus
	}

	if envSuiteType != "secretmanager" && envSuiteType != "parametermanager" && envSuiteType != "all" {
		log.Printf("Error: Invalid E2E_TEST_SUITE value: '%s'. Must be 'secretmanager', 'parametermanager', or 'all'.", envSuiteType)
		if exitCode == 0 {
			exitCode = 1
		} // Ensure non-zero exit if invalid suite and no tests ran.
	}
	os.Exit(exitCode)
}

// Handles setup/teardown test suite and runs test. Returns exit code.
func runTest(m *testing.M, isTokenPassed bool, suiteType string) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Test execution panic:", r)
			code = 1
		}
		teardownTestSuite(suiteType)
	}()

	setupTestSuite(isTokenPassed, suiteType)
	return m.Run()
}
