// Copyright 2020 Google LLC
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
package test

import (
	"bytes"
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
	fmt.Println(string(stdoutStderr))
	return err
}

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

func checkMountedParameterVersion(podName, filePath, expectedPayload string) error {
	var stdout, stderr bytes.Buffer
	command := exec.Command("kubectl", "exec", podName,
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default",
		"--",
		"cat", filePath)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		return fmt.Errorf("could not read parameter version from container: %w", err)
	}
	if !bytes.Equal(stdout.Bytes(), []byte(expectedPayload)) {
		return fmt.Errorf("parameter version payload value is %v, want: %v", stdout.String(), expectedPayload)
	}
	return nil
}

func checkMountedParameterVersionFileMode(podName, dataFilePath string, fileMode int) error {
	var stdout, stderr bytes.Buffer
	command := exec.Command("kubectl", "exec", podName,
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default",
		"--",
		"stat", "--printf", "%a", dataFilePath)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		return fmt.Errorf("could not stat parameter version file %s in pod %s: %w", dataFilePath, podName, err)
	}
	expectedFileMode := fmt.Sprintf("%o", fileMode)
	if !bytes.Equal(stdout.Bytes(), []byte(expectedFileMode)) {
		return fmt.Errorf("parameter version file mode is %v, want: %d", stdout.String(), fileMode)
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

// getParameterPrincipalID describes a parameter and returns its iamPolicyUidPrincipal.
func getParameterPrincipalID(parameterID, location, projectID string) (string, error) {
	var stdout, stderr bytes.Buffer
	args := []string{
		"parametermanager", "parameters", "describe", parameterID,
		"--location", location,
		"--project", projectID,
		"--format=value(policyMember.iamPolicyUidPrincipal)",
	}
	log.Println("+ gcloud", strings.Join(args, " ")) // Log the command
	cmd := exec.Command("gcloud", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Fatalf("Stdout: %v\n", stdout.String())
		log.Fatalf("Stderr: %v\n", stderr.String())
		return "", fmt.Errorf("failed to describe parameter %s in location %s: %w\nStderr: %s", parameterID, location, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Executed before any tests are run. Setup is only run once for all tests in the suite.
func setupTestSuite(isTokenPassed bool) {

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
	if len(f.testProjectID) == 0 {
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
	f.testSecretID = fmt.Sprintf("testsecret-%d", rand.Int31())

	f.testRotateSecretID = f.testSecretID + "-rotate"
	f.testExtractSecretID = f.testSecretID + "-extract"

	// Parameter manager specific e2e fields
	f.parameterIdYaml = fmt.Sprintf("testparameteryaml-%d", rand.Int31())
	f.parameterIdJson = fmt.Sprintf("testparameterjson-%d", rand.Int31())
	f.parameterVersionIdYAML = fmt.Sprintf("testparameterversionyaml-%d", rand.Int31())
	f.parameterVersionIdJSON = fmt.Sprintf("testparameterversionjson-%d", rand.Int31())
	f.regionalParameterIdYAML = fmt.Sprintf("testregionalparameteryaml-%d", rand.Int31())
	f.regionalParameterIdJSON = fmt.Sprintf("testregionalparameterjson-%d", rand.Int31())
	f.regionalParameterVersionIdYAML = fmt.Sprintf("testregionalparameterversion-%d", rand.Int31())
	f.regionalParameterVersionIdJSON = fmt.Sprintf("testregionalparameterversionjson-%d", rand.Int31())

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
	check(os.WriteFile(secretFile, []byte(f.testSecretID), 0644))
	check(execCmd(exec.Command("gcloud", "secrets", "create", f.testSecretID, "--replication-policy", "automatic",
		"--data-file", secretFile, "--project", f.testProjectID)))

	// Create test parameter and parameter versions -> global region (both YAML and JSON)
	parameterVersionFileYaml := filepath.Join(f.tempDir, "parameterValueYaml")
	parameterVersionFileJson := filepath.Join(f.tempDir, "parameterValueJson")

	// Write the byte payload of the parameters into files similar to how secret manager is doing it.
	check(os.WriteFile(parameterVersionFileYaml, []byte(fmt.Sprintf("user: admin\nuser2: support\ndb_pwd: __REF__(//secretmanager.googleapis.com/projects/%s/secrets/%s/versions/1)\n", f.testProjectID, f.testSecretID)), 0644))
	check(os.WriteFile(parameterVersionFileJson, []byte(fmt.Sprintf("{\"user\": \"admin\",\n \"user2\": \"support\", \"db_pwd\": \"__REF__(//secretmanager.googleapis.com/projects/%s/secrets/%s/versions/1)\"}", f.testProjectID, f.testSecretID)), 0644))

	// Create Parameters first
	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "create", f.parameterIdYaml,
		"--location", "global", "--parameter-format", "YAML", "--project", f.testProjectID)))

	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "create", f.parameterIdJson,
		"--location", "global", "--parameter-format", "JSON", "--project", f.testProjectID)))

	// Grant parameter principals access to the global secret
	globalYamlPrincipal, err := getParameterPrincipalID(f.parameterIdYaml, "global", f.testProjectID)
	check(err) // Use check(err) which panics on error
	check(execCmd(exec.Command("gcloud", "secrets", "add-iam-policy-binding", f.testSecretID,
		"--member", globalYamlPrincipal,
		"--role", "roles/secretmanager.secretAccessor",
		"--project", f.testProjectID)))

	globalJsonPrincipal, err := getParameterPrincipalID(f.parameterIdJson, "global", f.testProjectID)
	check(err)
	check(execCmd(exec.Command("gcloud", "secrets", "add-iam-policy-binding", f.testSecretID,
		"--member", globalJsonPrincipal,
		"--role", "roles/secretmanager.secretAccessor",
		"--project", f.testProjectID)))

	// Now create the versions using the files you just wrote
	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "versions", "create", f.parameterVersionIdYAML,
		"--parameter", f.parameterIdYaml, "--location", "global",
		"--payload-data-from-file", parameterVersionFileYaml, // Use the file path here
		"--project", f.testProjectID)))

	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "versions", "create", f.parameterVersionIdJSON,
		"--parameter", f.parameterIdJson, "--location", "global",
		"--payload-data-from-file", parameterVersionFileJson, // And here
		"--project", f.testProjectID)))

	// Create regional secret
	secretFile = filepath.Join(f.tempDir, "regionalSecretValue")
	check(os.WriteFile(secretFile, []byte(f.testSecretID+"-regional"), 0644))
	check(execCmd(exec.Command("gcloud", "config", "set", "api_endpoint_overrides/secretmanager",
		"https://secretmanager."+f.location+".rep.googleapis.com/")))
	check(execCmd(exec.Command("gcloud", "secrets", "create", f.testSecretID, "--location", f.location,
		"--data-file", secretFile, "--project", f.testProjectID)))

	// Create regional parameter and regional parameter version
	parameterVersionFileYamlRegional := filepath.Join(f.tempDir, "parameterValueYamlRegional")
	parameterVersionFileJsonRegional := filepath.Join(f.tempDir, "parameterValueJsonRegional")
	check(os.WriteFile(parameterVersionFileYamlRegional, []byte(fmt.Sprintf("user: admin\nuser2: support\ndb_pwd: __REF__(//secretmanager.googleapis.com/projects/%s/locations/%s/secrets/%s/versions/1)\n", f.testProjectID, f.location, f.testSecretID)), 0644))
	check(os.WriteFile(parameterVersionFileJsonRegional, []byte(fmt.Sprintf("{\"user\": \"admin\",\n \"user2\": \"support\", \"db_pwd\": \"__REF__(//secretmanager.googleapis.com/projects/%s/locations/%s/secrets/%s/versions/1)\"}", f.testProjectID, f.location, f.testSecretID)), 0644))

	// Set regional endpoint
	check(execCmd(exec.Command("gcloud", "config", "set", "api_endpoint_overrides/parametermanager",
		"https://parametermanager."+f.location+".rep.googleapis.com/")))

	// Create regional YAML and JSON parameters.
	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "create", f.regionalParameterIdYAML,
		"--location", f.location, "--parameter-format", "YAML", "--project", f.testProjectID)))
	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "create", f.regionalParameterIdJSON,
		"--location", f.location, "--parameter-format", "JSON", "--project", f.testProjectID)))

	// Grant parameter principals access to the regional secret
	regionalYamlPrincipal, err := getParameterPrincipalID(f.regionalParameterIdYAML, f.location, f.testProjectID)
	check(err)
	check(execCmd(exec.Command("gcloud", "secrets", "add-iam-policy-binding", f.testSecretID, // Assuming regional secret has same ID but different location context
		"--member", regionalYamlPrincipal,
		"--role", "roles/secretmanager.secretAccessor",
		"--project", f.testProjectID, "--location", f.location))) // Need location for regional secret binding

	regionalJsonPrincipal, err := getParameterPrincipalID(f.regionalParameterIdJSON, f.location, f.testProjectID)
	check(err)
	check(execCmd(exec.Command("gcloud", "secrets", "add-iam-policy-binding", f.testSecretID, // Assuming regional secret has same ID but different location context
		"--member", regionalJsonPrincipal,
		"--role", "roles/secretmanager.secretAccessor",
		"--project", f.testProjectID, "--location", f.location))) // Need location for regional secret binding

	// Now create corresponding parameter versions to YAML and JSON parameters just created
	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "versions", "create", f.regionalParameterVersionIdYAML,
		"--parameter", f.regionalParameterVersionIdYAML, "--location", f.location,
		"--payload-data-from-file", parameterVersionFileYamlRegional, // Use the file path here
		"--project", f.testProjectID)))

	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "versions", "create", f.regionalParameterVersionIdJSON,
		"--parameter", f.regionalParameterIdJSON, "--location", f.location,
		"--payload-data-from-file", parameterVersionFileJsonRegional, // And here
		"--project", f.testProjectID)))

	// Setting endpoints back to the global defaults
	check(execCmd(exec.Command("gcloud", "config", "unset", "api_endpoint_overrides/secretmanager")))
	check(execCmd(exec.Command("gcloud", "config", "unset", "api_endpoint_overrides/parametermanager")))

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
			"-f", fmt.Sprintf("./csidriver.yaml"),
		)))
	}
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

	// Cleanup
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
	execCmd(exec.Command(
		"gcloud", "secrets", "delete", f.testExtractSecretID,
		"--project", f.testProjectID,
		"--quiet",
	))

	// Execute gcloud delete parameter version and delete parameter -> Both YAML and JSON
	// gcloud parametermanager parameters versions delete PARAMETER_VERSION_ID --parameter=PARAMETER_ID --location=global
	check(execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "versions", "delete", f.parameterVersionIdYAML,
		"--parameter", f.parameterIdYaml,
		"--location", "global",
		"--project", f.testProjectID,
		"--quiet",
	)))
	check(execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "versions", "delete", f.parameterVersionIdJSON,
		"--parameter", f.parameterIdJson,
		"--location", "global",
		"--project", f.testProjectID,
		"--quiet",
	)))
	// gcloud parametermanager parameters delete PARAMETER_ID --location=global
	check(execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "delete", f.parameterIdYaml,
		"--location", "global",
		"--project", f.testProjectID,
		"--quiet",
	)))
	check(execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "delete", f.parameterIdJson,
		"--location", "global",
		"--project", f.testProjectID,
		"--quiet",
	)))
	// gcloud parametermanager parameters delete PARAMETER_ID --location=global

	// Cleanup regional secret
	check(execCmd(exec.Command("gcloud", "config", "set", "api_endpoint_overrides/secretmanager",
		"https://secretmanager."+f.location+".rep.googleapis.com/")))
	check(execCmd(exec.Command("gcloud", "secrets", "delete", f.testSecretID, "--location", f.location,
		"--project", f.testProjectID, "--quiet")))
	check(execCmd(exec.Command("gcloud", "secrets", "delete", f.testRotateSecretID, "--location", f.location,
		"--project", f.testProjectID, "--quiet")))
	check(execCmd(exec.Command("gcloud", "config", "unset", "api_endpoint_overrides/secretmanager")))

	// Clean regional parameters -> Both YAML and JSON
	check(execCmd(exec.Command("gcloud", "config", "set", "api_endpoint_overrides/parametermanager",
		"https://parametermanager."+f.location+".rep.googleapis.com/")))

	check(execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "versions", "delete", f.regionalParameterVersionIdYAML,
		"--parameter", f.regionalParameterIdYAML,
		"--location", f.location,
		"--project", f.testProjectID,
		"--quiet",
	)))
	check(execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "versions", "delete", f.regionalParameterVersionIdJSON,
		"--parameter", f.regionalParameterIdJSON,
		"--location", f.location,
		"--project", f.testProjectID,
		"--quiet",
	)))

	check(execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "delete", f.regionalParameterIdYAML,
		"--location", f.location,
		"--project", f.testProjectID,
		"--quiet",
	)))
	check(execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "delete", f.regionalParameterIdJSON,
		"--location", f.location,
		"--project", f.testProjectID,
		"--quiet",
	)))

	//gcloud parametermanager parameters versions delete PARAMETER_VERSION_ID --parameter=PARAMETER_IDl --location=LOCATION
	// gcloud parametermanager parameters delete PARAMETER_ID --location=LOCATION
	check(execCmd(exec.Command("gcloud", "config", "unset", "api_endpoint_overrides/parametermanager")))
}

// Entry point for go test.
func TestMain(m *testing.M) {
	withoutTokenStatus := runTest(m, false)
	withTokenStatus := runTest(m, true)
	fmt.Printf("Exit Code when token is not passed from driver to provder is: %v\n", withoutTokenStatus)
	fmt.Printf("Exit Code when token is passed from driver to provder is: %v\n", withTokenStatus)
	os.Exit(withoutTokenStatus | withTokenStatus)
}

// Handles setup/teardown test suite and runs test. Returns exit code.
func runTest(m *testing.M, isTokenPassed bool) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Test execution panic:", r)
			code = 1
		}
		teardownTestSuite()
	}()

	setupTestSuite(isTokenPassed)
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

	// TODO: Add checks for regional secret
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
		execCmd(exec.Command(
			"kubectl", "describe", "pods",
			"--namespace", "default",
			"--kubeconfig", f.kubeconfigFile,
		))
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

	// Wait for update. Keep in sync with driver's --rotation-poll-interval to
	// ensure enough time.
	time.Sleep(60 * time.Second)

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

	// TODO: Add checks for regional secret
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

// mounts global and regional parameter versions and checks whether they are equivalent or not (both json and yaml)
func TestMountParameterVersion(t *testing.T) {
	podFile := filepath.Join(f.tempDir, "test-parameter-version-pod.yaml")
	if err := replaceTemplate("templates/test-parameter-version-pod.yaml.tmpl", podFile); err != nil {
		t.Fatalf("Error replacing pod template: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"--namespace", "default", "-f", podFile)); err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	// As a workaround for https://github.com/kubernetes/kubernetes/issues/83242, we sleep to
	// ensure that the job resources exists before attempting to wait for it.
	time.Sleep(5 * time.Second)
	if err := execCmd(exec.Command("kubectl", "wait", "pod/test-parameter-version-mounter", "--for=condition=Ready",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default", "--timeout", "5m")); err != nil {
		t.Fatalf("Error waiting for job: %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-mounter", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version/%s/global/%s", f.parameterIdYaml, f.parameterVersionIdYAML), // mounted file path
		fmt.Sprintf("user: admin\nuser2: support\ndb_pwd: %s\n", f.testSecretID),                                 // expected payload
	); err != nil {
		t.Fatalf("Error while testing global yaml parameter version: %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-mounter", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version/%s/global/%s", f.parameterIdJson, f.parameterVersionIdJSON), // mounted filepath
		fmt.Sprintf("{\"user\": \"admin\",\n \"user2\": \"support\", \"db_pwd\": \"%s\"}", f.testSecretID),       // expected payload
	); err != nil {
		t.Fatalf("Error while testing global json parameter version: %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-mounter", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version/%s/%s/%s", f.regionalParameterIdYAML, f.location, f.regionalParameterVersionIdYAML), // mounted filepath
		fmt.Sprintf("user: admin\nuser2: support\ndb_pwd: %s-regional\n", f.testSecretID),                                                // expected payload
	); err != nil {
		t.Fatalf("Error while testing regional yaml parameter version: %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-mounter", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version/%s/%s/%s", f.regionalParameterIdJSON, f.location, f.regionalParameterVersionIdJSON), // filepath
		fmt.Sprintf("{\"user\": \"admin\",\n \"user2\": \"support\", \"db_pwd\": \"%s-regional\"}", f.testSecretID),                      // expected payload
	); err != nil {
		t.Fatalf("Error while testing regional json parameter version: %v", err)
	}
}

// mounts global and regional parameter versions and applies extractYAMLKey and extractJSONKey whenever applicable
func TestMountParameterVersionExtractKeys(t *testing.T) {
	podFile := filepath.Join(f.tempDir, "test-parameter-version-extract-keys.yaml")
	if err := replaceTemplate("templates/test-parameter-version-extract-keys.yaml.tmpl", podFile); err != nil {
		t.Fatalf("Error replacing pod template: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"--namespace", "default", "-f", podFile)); err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-mounter-filemode", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-keys/%s/global/%s", f.parameterIdYaml, f.parameterVersionIdYAML), // mounted file path
		f.testSecretID, // expected payload (extractYAMLKey used with key db_pwd used)
	); err != nil {
		t.Fatalf("Error while testing global yaml parameter version extracted key 'db_pwd': %v", err) // expected global secret
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-mounter-filemode", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-keys/%s/global/%s", f.parameterIdJson, f.parameterVersionIdJSON), // mounted filepath
		"admin", // expected payload (extractJSONKey with key user used)
	); err != nil {
		t.Fatalf("Error while testing global json parameter version extracted key 'user': %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-mounter-filemode", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-keys/%s/%s/%s", f.regionalParameterIdYAML, f.location, f.regionalParameterVersionIdYAML), // mounted filepath
		"support", // expected payload (extractYAMLKey used with key user2 used)
	); err != nil {
		t.Fatalf("Error while testing regional yaml parameter version extracted key 'user2': %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-mounter-filemode", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-keys/%s/%s/%s", f.regionalParameterIdJSON, f.location, f.regionalParameterVersionIdJSON), // filepath
		fmt.Sprintf("%s-regional", f.testSecretID), // expected payload (extractJSONKey used with key db_pwd used)
	); err != nil {
		t.Fatalf("Error while testing regional json parameter version extracted key 'db_pwd': %v", err) // expected regional secret
	}
}

// mounts global and regional yaml and json parameter versions at the exact ..data locations, not at their symlinks
func TestMountParameterVersionFileMode(t *testing.T) {
	podFile := filepath.Join(f.tempDir, "test-parameter-version-pod-mode.yaml")
	if err := replaceTemplate("templates/test-parameter-version-pod-mode.yaml.tmpl", podFile); err != nil {
		t.Fatalf("Error replacing pod template: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"--namespace", "default", "-f", podFile)); err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	if err := checkMountedParameterVersionFileMode(
		"test-parameter-version-mounter-filemode", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-mode/..data/%s/global/%s", f.parameterIdYaml, f.parameterVersionIdYAML), // mounted file path
		0420, // expected mode
	); err != nil {
		t.Fatalf("Error while testing global yaml parameter version: %v", err)
	}

	if err := checkMountedParameterVersionFileMode(
		"test-parameter-version-mounter-filemode", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-mode/..data/%s/global/%s", f.parameterIdJson, f.parameterVersionIdJSON), // mounted filepath
		0600, // expected mode
	); err != nil {
		t.Fatalf("Error while testing global json parameter version: %v", err)
	}

	if err := checkMountedParameterVersionFileMode(
		"test-parameter-version-mounter-filemode", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-mode/..data/%s/%s/%s", f.regionalParameterIdYAML, f.location, f.regionalParameterVersionIdYAML), // mounted filepath
		0400, // expected mode
	); err != nil {
		t.Fatalf("Error while testing regional yaml parameter version filemode: %v", err)
	}

	if err := checkMountedParameterVersionFileMode(
		"test-parameter-version-mounter-filemode", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-mode/..data/%s/%s/%s", f.regionalParameterIdJSON, f.location, f.regionalParameterVersionIdJSON), // filepath
		0440, // expected mode
	); err != nil {
		t.Fatalf("Error while testing regional json parameter version filemode: %v", err)
	}
}
