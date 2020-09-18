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
package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// Reads secret specified by TEST_SECRET_ID from mounted directory and writes it to a k8s config map.
func main() {
	secretId := os.Getenv("TEST_SECRET_ID")
	if len(secretId) == 0 {
		log.Fatal("TEST_SECRET_ID is empty")
	}

	secret, err := ioutil.ReadFile(filepath.Join("/var/gcp-test-secrets", secretId))
	if err != nil {
		log.Fatalf("Could not read secret file %v: %v", secretId, err)
	}

	command := exec.Command("kubectl", "create", "configmap", "secretmap",
		"--from-literal=csiSecret="+base64.StdEncoding.EncodeToString(secret))
	fmt.Println("+", command)
	stdoutStderr, err := command.CombinedOutput()
	fmt.Println(string(stdoutStderr))
	if err != nil {
		log.Fatalf("Could not create config map: %v", err)
	}
}
