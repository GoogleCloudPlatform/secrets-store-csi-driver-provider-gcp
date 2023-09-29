package vars

import (
	"fmt"
	"os"

	"k8s.io/klog/v2"
)

type EnvVar struct {
	env_var_name  string
	default_value string
	is_required   bool
}

func (ev EnvVar) GetValue() (string, error) {
	os_env := ev.env_var_name
	os_env_value, is_present := os.LookupEnv(os_env)

	if is_present {
		return os_env_value, nil
	}

	if ev.is_required {
		return "", fmt.Errorf("%s: a required OS Environment is not present!", os_env)
	}

	// Return default endpoint
	fmt.Println(os_env, "OS Environment is not present! Using Default Prod endpoint")
	fmt.Println(os.Environ())
	fmt.Println("\n\n")
	klog.V(5).InfoS(os_env, "OS Environment is not present! Using Default Prod endpoint", ev.default_value)
	return ev.default_value, nil
}

var IdentityBindingTokenEndPoint = EnvVar{
	env_var_name:  "SM_PROVIDER_ID_BINDING_TOKEN_ENDPOINT",
	default_value: "https://securetoken.googleapis.com/v1/identitybindingtoken",
	is_required:   false,
}

var GkeWorkloadIdentityEndPoint = EnvVar{
	env_var_name:  "SM_PROVIDER_GKE_WORKLOAD_IDENTITY_ENDPOINT",
	default_value: "https://container.googleapis.com/v1",
	is_required:   false,
}
