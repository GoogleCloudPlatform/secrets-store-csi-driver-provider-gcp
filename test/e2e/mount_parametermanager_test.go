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

//go:build parametermanager_e2e || all_e2e
// +build parametermanager_e2e all_e2e

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
)

// Create f.pmReferenceSecretID (for PM parameter references) if parametermanager or all suite is run
func setupPmTestSuite() {

	// Parameter manager specific e2e fields
	f.parameterIdYaml = fmt.Sprintf("testparameteryaml-%d", rand.Int31())
	f.parameterIdJson = fmt.Sprintf("testparameterjson-%d", rand.Int31())
	f.parameterVersionIdYAML = fmt.Sprintf("testparameterversionyaml-%d", rand.Int31())
	f.parameterVersionIdJSON = fmt.Sprintf("testparameterversionjson-%d", rand.Int31())
	f.regionalParameterIdYAML = fmt.Sprintf("testregionalparameteryaml-%d", rand.Int31())
	f.regionalParameterIdJSON = fmt.Sprintf("testregionalparameterjson-%d", rand.Int31())
	f.regionalParameterVersionIdYAML = fmt.Sprintf("testregionalparameterversion-%d", rand.Int31())
	f.regionalParameterVersionIdJSON = fmt.Sprintf("testregionalparameterversionjson-%d", rand.Int31())
	f.pmReferenceGlobalSecret1 = fmt.Sprintf("pmReferenceGlobalSecret1-%d", rand.Int31())
	f.pmReferenceGlobalSecret2 = fmt.Sprintf("pmReferenceGlobalSecret2-%d", rand.Int31())
	f.pmReferenceRegionalSecret1 = fmt.Sprintf("pmReferenceRegionalSecret1-%d", rand.Int31())
	f.pmReferenceRegionalSecret2 = fmt.Sprintf("pmReferenceRegionalSecret2-%d", rand.Int31())

	// Create global test secrets to be referred for parametermanager
	// Path where data-files for secrets are stored
	globalSecretRef1 := filepath.Join(f.tempDir, "globalSecretRef1")
	check(os.WriteFile(globalSecretRef1, []byte(
		fmt.Sprintf("%s-%s", f.pmReferenceGlobalSecret1, "global-s3cr3t1"),
	), 0644))
	check(execCmd(exec.Command("gcloud", "secrets", "create", f.pmReferenceGlobalSecret1, "--replication-policy", "automatic",
		"--data-file", globalSecretRef1, "--project", f.testProjectID)))

	globalSecretRef2 := filepath.Join(f.tempDir, "globalSecretRef2")
	check(os.WriteFile(globalSecretRef2, []byte(
		fmt.Sprintf("%s-%s", f.pmReferenceGlobalSecret2, "global-s3cr3tReplica2"),
	), 0644))
	check(execCmd(exec.Command("gcloud", "secrets", "create", f.pmReferenceGlobalSecret2, "--replication-policy", "automatic",
		"--data-file", globalSecretRef2, "--project", f.testProjectID)))

	// Create test parameter and parameter versions -> global region (both YAML and JSON)
	parameterVersionFileYaml := filepath.Join(f.tempDir, "parameterValueYaml.yaml")
	parameterVersionFileJson := filepath.Join(f.tempDir, "parameterValueJson.json")

	// Write the byte payload of the parameters into files similar to how secret manager is doing it.

	check(os.WriteFile(parameterVersionFileYaml, []byte(
		fmt.Sprintf(
			`user: admin
user2: support
db_pwd: __REF__(//secretmanager.googleapis.com/projects/%s/secrets/%s/versions/1)
backup_pwd: __REF__(//secretmanager.googleapis.com/projects/%s/secrets/%s/versions/1)`,
			f.testProjectID, f.pmReferenceGlobalSecret1, f.testProjectID, f.pmReferenceGlobalSecret2)), 0644))

	check(os.WriteFile(parameterVersionFileJson, []byte(
		fmt.Sprintf(
			`{
	"user": "admin",
	"user2": "support",
	"db_pwd": "__REF__(//secretmanager.googleapis.com/projects/%s/secrets/%s/versions/1)",
	"backup_pwd": "__REF__(//secretmanager.googleapis.com/projects/%s/secrets/%s/versions/1)"
}`,
			f.testProjectID, f.pmReferenceGlobalSecret1, f.testProjectID, f.pmReferenceGlobalSecret2)), 0644))

	// Create Parameters first
	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "create", f.parameterIdYaml,
		"--location", "global", "--parameter-format", "YAML", "--project", f.testProjectID)))

	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "create", f.parameterIdJson,
		"--location", "global", "--parameter-format", "JSON", "--project", f.testProjectID)))

	// Grant parameter principals access to the global secret
	globalYamlPrincipal, err := getParameterPrincipalID(f.parameterIdYaml, "global", f.testProjectID)
	check(err) // Use check(err) which panics on error
	check(execCmd(exec.Command("gcloud", "secrets", "add-iam-policy-binding", f.pmReferenceGlobalSecret1,
		"--member", globalYamlPrincipal,
		"--role", "roles/secretmanager.secretAccessor",
		"--project", f.testProjectID)))
	check(execCmd(exec.Command("gcloud", "secrets", "add-iam-policy-binding", f.pmReferenceGlobalSecret2,
		"--member", globalYamlPrincipal,
		"--role", "roles/secretmanager.secretAccessor",
		"--project", f.testProjectID)))

	globalJsonPrincipal, err := getParameterPrincipalID(f.parameterIdJson, "global", f.testProjectID)
	check(err)
	check(execCmd(exec.Command("gcloud", "secrets", "add-iam-policy-binding", f.pmReferenceGlobalSecret1,
		"--member", globalJsonPrincipal,
		"--role", "roles/secretmanager.secretAccessor",
		"--project", f.testProjectID)))
	check(execCmd(exec.Command("gcloud", "secrets", "add-iam-policy-binding", f.pmReferenceGlobalSecret2,
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

	// Create regional parameter and regional parameter version
	parameterVersionFileYamlRegional := filepath.Join(f.tempDir, "parameterValueYamlRegional.yaml")
	parameterVersionFileJsonRegional := filepath.Join(f.tempDir, "parameterValueJsonRegional.json")

	check(os.WriteFile(parameterVersionFileYamlRegional, []byte(
		fmt.Sprintf(
			`user: admin
user2: support
db_regional_pwd: __REF__(//secretmanager.googleapis.com/projects/%s/locations/%s/secrets/%s/versions/1)
backup_regional_pwd: __REF__(//secretmanager.googleapis.com/projects/%s/locations/%s/secrets/%s/versions/1)`,
			f.testProjectID, f.location, f.pmReferenceRegionalSecret1, f.testProjectID, f.location, f.pmReferenceRegionalSecret2)), 0644))

	check(os.WriteFile(parameterVersionFileJsonRegional, []byte(
		fmt.Sprintf(
			`{
	"user": "admin",
	"user2": "support",
	"db_regional_pwd": "__REF__(//secretmanager.googleapis.com/projects/%s/locations/%s/secrets/%s/versions/1)",
	"backup_regional_pwd": "__REF__(//secretmanager.googleapis.com/projects/%s/locations/%s/secrets/%s/versions/1)"
}`,
			f.testProjectID, f.location, f.pmReferenceRegionalSecret1, f.testProjectID, f.location, f.pmReferenceRegionalSecret2)), 0644))

	// Set regional endpoint
	check(execCmd(exec.Command("gcloud", "config", "set", "api_endpoint_overrides/secretmanager",
		"https://secretmanager."+f.location+".rep.googleapis.com/")))
	check(execCmd(exec.Command("gcloud", "config", "set", "api_endpoint_overrides/parametermanager",
		"https://parametermanager."+f.location+".rep.googleapis.com/")))

	// Create regional secrets
	// Path where data-files for regional-secrets are stored
	regionalSecretRef1 := filepath.Join(f.tempDir, "regionalSecretRef1")
	check(os.WriteFile(regionalSecretRef1, []byte(
		fmt.Sprintf("%s-%s", f.pmReferenceRegionalSecret1, "regional-s3cr3t1"),
	), 0644))

	check(execCmd(
		exec.Command("gcloud", "secrets", "create", f.pmReferenceRegionalSecret1,
			"--location", f.location,
			"--data-file", regionalSecretRef1, "--project", f.testProjectID)))

	regionalSecretRef2 := filepath.Join(f.tempDir, "regionalSecretRef2")
	check(os.WriteFile(regionalSecretRef2, []byte(
		fmt.Sprintf("%s-%s", f.pmReferenceRegionalSecret2, "regional-s3cr3tReplica2"),
	), 0644))
	check(execCmd(
		exec.Command("gcloud", "secrets", "create", f.pmReferenceRegionalSecret2,
			"--location", f.location,
			"--data-file", regionalSecretRef2, "--project", f.testProjectID)))

	// Create regional YAML and JSON parameters.
	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "create", f.regionalParameterIdYAML,
		"--location", f.location, "--parameter-format", "YAML", "--project", f.testProjectID)))
	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "create", f.regionalParameterIdJSON,
		"--location", f.location, "--parameter-format", "JSON", "--project", f.testProjectID)))

	// Grant parameter principals access to the regional secret
	regionalYamlPrincipal, err := getParameterPrincipalID(f.regionalParameterIdYAML, f.location, f.testProjectID)
	check(err)
	check(execCmd(exec.Command("gcloud", "secrets", "add-iam-policy-binding", f.pmReferenceRegionalSecret1,
		"--member", regionalYamlPrincipal,
		"--role", "roles/secretmanager.secretAccessor",
		"--project", f.testProjectID, "--location", f.location)))

	check(execCmd(exec.Command("gcloud", "secrets", "add-iam-policy-binding", f.pmReferenceRegionalSecret2,
		"--member", regionalYamlPrincipal,
		"--role", "roles/secretmanager.secretAccessor",
		"--project", f.testProjectID, "--location", f.location)))

	regionalJsonPrincipal, err := getParameterPrincipalID(f.regionalParameterIdJSON, f.location, f.testProjectID)
	check(err)

	check(execCmd(exec.Command("gcloud", "secrets", "add-iam-policy-binding", f.pmReferenceRegionalSecret1,
		"--member", regionalJsonPrincipal,
		"--role", "roles/secretmanager.secretAccessor",
		"--project", f.testProjectID, "--location", f.location)))

	check(execCmd(exec.Command("gcloud", "secrets", "add-iam-policy-binding", f.pmReferenceRegionalSecret2,
		"--member", regionalJsonPrincipal,
		"--role", "roles/secretmanager.secretAccessor",
		"--project", f.testProjectID, "--location", f.location)))

	// Now create corresponding parameter versions to YAML and JSON parameters just created
	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "versions", "create", f.regionalParameterVersionIdYAML,
		"--parameter", f.regionalParameterIdYAML, "--location", f.location,
		"--payload-data-from-file", parameterVersionFileYamlRegional, // Use the file path here
		"--project", f.testProjectID)))

	check(execCmd(exec.Command("gcloud", "parametermanager", "parameters", "versions", "create", f.regionalParameterVersionIdJSON,
		"--parameter", f.regionalParameterIdJSON, "--location", f.location,
		"--payload-data-from-file", parameterVersionFileJsonRegional, // And here
		"--project", f.testProjectID)))

	// Add a delay to allow IAM changes for Parameter Manager service identities to propagate.
	// This is to mitigate potential 'context deadline exceeded' errors during parameter version rendering
	// if the Parameter's service identity doesn't yet have permissions to access referenced secrets.
	log.Println("Waiting 90s for IAM policy propagation for Parameter Manager service identities...")
	time.Sleep(90 * time.Second)

	// Setting endpoints back to the global defaults
	check(execCmd(exec.Command("gcloud", "config", "unset", "api_endpoint_overrides/secretmanager")))
	check(execCmd(exec.Command("gcloud", "config", "unset", "api_endpoint_overrides/parametermanager")))
}

func teardownPmTestSuite() {
	// Execute gcloud delete parameter version and delete parameter -> Both YAML and JSON
	execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "versions", "delete", f.parameterVersionIdYAML,
		"--parameter", f.parameterIdYaml,
		"--location", "global",
		"--project", f.testProjectID,
		"--quiet",
	))
	execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "versions", "delete", f.parameterVersionIdJSON,
		"--parameter", f.parameterIdJson,
		"--location", "global",
		"--project", f.testProjectID,
		"--quiet",
	))
	execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "delete", f.parameterIdYaml,
		"--location", "global",
		"--project", f.testProjectID,
		"--quiet",
	))
	execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "delete", f.parameterIdJson,
		"--location", "global",
		"--project", f.testProjectID,
		"--quiet",
	))

	// Delete pm referred global secrets
	execCmd(exec.Command(
		"gcloud", "secrets", "delete", f.pmReferenceGlobalSecret1,
		"--project", f.testProjectID,
		"--quiet",
	))
	execCmd(exec.Command(
		"gcloud", "secrets", "delete", f.pmReferenceGlobalSecret2,
		"--project", f.testProjectID,
		"--quiet",
	))

	// Clean regional parameters -> Both YAML and JSON
	check(execCmd(exec.Command("gcloud", "config", "set", "api_endpoint_overrides/parametermanager",
		"https://parametermanager."+f.location+".rep.googleapis.com/")))

	check(execCmd(exec.Command("gcloud", "config", "set", "api_endpoint_overrides/secretmanager",
		"https://secretmanager."+f.location+".rep.googleapis.com/")))

	execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "versions", "delete", f.regionalParameterVersionIdYAML,
		"--parameter", f.regionalParameterIdYAML,
		"--location", f.location,
		"--project", f.testProjectID,
		"--quiet",
	))
	execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "versions", "delete", f.regionalParameterVersionIdJSON,
		"--parameter", f.regionalParameterIdJSON,
		"--location", f.location,
		"--project", f.testProjectID,
		"--quiet",
	))

	execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "delete", f.regionalParameterIdYAML,
		"--location", f.location,
		"--project", f.testProjectID,
		"--quiet",
	))
	execCmd(exec.Command(
		"gcloud", "parametermanager", "parameters", "delete", f.regionalParameterIdJSON,
		"--location", f.location,
		"--project", f.testProjectID,
		"--quiet",
	))

	execCmd(exec.Command(
		"gcloud", "secrets", "delete", f.pmReferenceRegionalSecret1,
		"--location", f.location,
		"--project", f.testProjectID,
		"--quiet",
	))
	execCmd(exec.Command(
		"gcloud", "secrets", "delete", f.pmReferenceRegionalSecret2,
		"--location", f.location,
		"--project", f.testProjectID,
		"--quiet",
	))
	check(execCmd(exec.Command("gcloud", "config", "unset", "api_endpoint_overrides/parametermanager")))
	check(execCmd(exec.Command("gcloud", "config", "unset", "api_endpoint_overrides/secretmanager")))
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

func checkMountedParameterVersionFileMode(dataFilePath, fileMode string) error {
	var stdout, stderr bytes.Buffer
	command := exec.Command("kubectl", "exec", "test-parameter-version-mounter-filemode",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default",
		"--",
		"stat", "--printf", "%a", dataFilePath)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		fmt.Println("Stdout:", stdout.String())
		fmt.Println("Stderr:", stderr.String())
		return fmt.Errorf("could not read parameter version file %s from container, error: %w", dataFilePath, err)
	}
	if !bytes.Equal(stdout.Bytes(), []byte(fileMode)) {
		return fmt.Errorf("parameter version file mode is %v, want: %s", stdout.String(), fileMode)
	}
	return nil
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
		fmt.Sprintf(
			`user: admin
user2: support
db_pwd: %s
backup_pwd: %s`,
			fmt.Sprintf("%s-%s", f.pmReferenceGlobalSecret1, "global-s3cr3t1"),
			fmt.Sprintf("%s-%s", f.pmReferenceGlobalSecret2, "global-s3cr3tReplica2"),
		), // expected payload
	); err != nil {
		t.Fatalf("Error while testing global yaml parameter version: %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-mounter", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version/%s/global/%s", f.parameterIdJson, f.parameterVersionIdJSON), // mounted filepath
		fmt.Sprintf(
			`{
	"user": "admin",
	"user2": "support",
	"db_pwd": "%s",
	"backup_pwd": "%s"
}`,
			fmt.Sprintf("%s-%s", f.pmReferenceGlobalSecret1, "global-s3cr3t1"),
			fmt.Sprintf("%s-%s", f.pmReferenceGlobalSecret2, "global-s3cr3tReplica2"), // expected payload
		),
	); err != nil {
		t.Fatalf("Error while testing global json parameter version: %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-mounter", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version/%s/%s/%s", f.regionalParameterIdYAML, f.location, f.regionalParameterVersionIdYAML), // mounted filepath
		fmt.Sprintf(
			`user: admin
user2: support
db_regional_pwd: %s
backup_regional_pwd: %s`,
			fmt.Sprintf("%s-%s", f.pmReferenceRegionalSecret1, "regional-s3cr3t1"),
			fmt.Sprintf("%s-%s", f.pmReferenceRegionalSecret2, "regional-s3cr3tReplica2"),
		), // expected payload
	); err != nil {
		t.Fatalf("Error while testing regional yaml parameter version: %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-mounter", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version/%s/%s/%s", f.regionalParameterIdJSON, f.location, f.regionalParameterVersionIdJSON), // filepath
		fmt.Sprintf(
			`{
	"user": "admin",
	"user2": "support",
	"db_regional_pwd": "%s",
	"backup_regional_pwd": "%s"
}`,
			fmt.Sprintf("%s-%s", f.pmReferenceRegionalSecret1, "regional-s3cr3t1"),
			fmt.Sprintf("%s-%s", f.pmReferenceRegionalSecret2, "regional-s3cr3tReplica2"),
		), // expected payload
	); err != nil {
		t.Fatalf("Error while testing regional json parameter version: %v", err)
	}
}

// mounts global and regional parameter versions and applies extractJSONKey whenever applicable
func TestMountParameterVersionExtractKeys(t *testing.T) {
	podFile := filepath.Join(f.tempDir, "test-parameter-version-extract-keys.yaml")
	if err := replaceTemplate("templates/test-parameter-version-extract-keys.yaml.tmpl", podFile); err != nil {
		t.Fatalf("Error replacing pod template: %v", err)
	}

	if err := execCmd(exec.Command("kubectl", "apply", "--kubeconfig", f.kubeconfigFile,
		"--namespace", "default", "-f", podFile)); err != nil {
		t.Fatalf("Error creating job: %v", err)
	}

	// As a workaround for https://github.com/kubernetes/kubernetes/issues/83242, we sleep to
	// ensure that the job resources exists before attempting to wait for it.
	time.Sleep(5 * time.Second)
	if err := execCmd(exec.Command("kubectl", "wait", "pod/test-parameter-version-key-extraction", "--for=condition=Ready",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default", "--timeout", "5m")); err != nil {
		t.Fatalf("Error waiting for pod test-parameter-version-key-extraction: %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-key-extraction", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-keys/%s/global/%s", f.parameterIdYaml, f.parameterVersionIdYAML), // mounted file path
		fmt.Sprintf("%s-%s", f.pmReferenceGlobalSecret1, "global-s3cr3t1"),                                            // expected payload (extractYAMLKey with key db_pwd used)
	); err != nil {
		t.Fatalf("Error while testing global yaml parameter version extracted key 'db_pwd': %v", err) // expected global secret
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-key-extraction", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-keys/%s/global/%s", f.parameterIdJson, f.parameterVersionIdJSON), // mounted filepath
		"admin", // expected payload (extractJSONKey with key user used)
	); err != nil {
		t.Fatalf("Error while testing global json parameter version extracted key 'user': %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-key-extraction", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-keys/%s/%s/%s", f.regionalParameterIdYAML, f.location, f.regionalParameterVersionIdYAML), // mounted filepath
		"support", // expected payload (extractYAMLKey with key user2 used)
	); err != nil {
		t.Fatalf("Error while testing regional yaml parameter version extracted key 'user2': %v", err)
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-key-extraction", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-keys/%s/%s/db_regional_pwd/%s", f.regionalParameterIdJSON, f.location, f.regionalParameterVersionIdJSON), // filepath
		fmt.Sprintf("%s-%s", f.pmReferenceRegionalSecret1, "regional-s3cr3t1"),                                                                                // expected payload (extractJSONKey used with key db_regional_pwd used)
	); err != nil {
		t.Fatalf("Error while testing regional json parameter version extracted key 'db_regional_pwd': %v", err) // expected regional secret
	}

	if err := checkMountedParameterVersion(
		"test-parameter-version-key-extraction", // podName
		fmt.Sprintf("/var/gcp-test-parameter-version-keys/%s/%s/backup_regional_pwd/%s", f.regionalParameterIdJSON, f.location, f.regionalParameterVersionIdJSON), // filepath
		fmt.Sprintf("%s-%s", f.pmReferenceRegionalSecret2, "regional-s3cr3tReplica2"),                                                                             // expected payload (extractJSONKey used with key backup_regional_pwd used)
	); err != nil {
		t.Fatalf("Error while testing regional json parameter version extracted key 'backup_regional_pwd': %v", err) // expected regional secret
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

	// As a workaround for https://github.com/kubernetes/kubernetes/issues/83242, we sleep to
	// ensure that the job resources exists before attempting to wait for it.
	time.Sleep(5 * time.Second)
	if err := execCmd(exec.Command("kubectl", "wait", "pod/test-parameter-version-mounter-filemode", "--for=condition=Ready",
		"--kubeconfig", f.kubeconfigFile, "--namespace", "default", "--timeout", "5m")); err != nil {
		t.Fatalf("Error waiting for pod test-parameter-version-mounter-filemode: %v", err)
	}

	if err := checkMountedParameterVersionFileMode(
		fmt.Sprintf("/var/gcp-test-parameter-version-mode/..data/%s/global/%s", f.parameterIdYaml, f.parameterVersionIdYAML), // mounted file path
		"420", // expected mode
	); err != nil {
		t.Fatalf("Error while testing global yaml parameter version: %v", err)
	}

	if err := checkMountedParameterVersionFileMode(
		fmt.Sprintf("/var/gcp-test-parameter-version-mode/..data/%s/global/%s", f.parameterIdJson, f.parameterVersionIdJSON), // mounted filepath
		"600", // expected mode
	); err != nil {
		t.Fatalf("Error while testing global json parameter version: %v", err)
	}

	if err := checkMountedParameterVersionFileMode(
		fmt.Sprintf("/var/gcp-test-parameter-version-mode/..data/%s/%s/%s", f.regionalParameterIdYAML, f.location, f.regionalParameterVersionIdYAML), // mounted filepath
		"400", // expected mode
	); err != nil {
		t.Fatalf("Error while testing regional yaml parameter version filemode: %v", err)
	}

	if err := checkMountedParameterVersionFileMode(
		fmt.Sprintf("/var/gcp-test-parameter-version-mode/..data/%s/%s/%s", f.regionalParameterIdJSON, f.location, f.regionalParameterVersionIdJSON), // filepath
		"440", // expected mode
	); err != nil {
		t.Fatalf("Error while testing regional json parameter version filemode: %v", err)
	}
}
