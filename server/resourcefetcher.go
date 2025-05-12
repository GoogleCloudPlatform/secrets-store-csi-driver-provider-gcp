package server

import (
	"context"
	"fmt"
	"sync"

	parametermanager "cloud.google.com/go/parametermanager/apiv1"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/util"
	"github.com/googleapis/gax-go/v2"
)

type ResourceType int

const (
	ParameterVersion ResourceType = iota
	SecretRef
)

// resourceFetcher is the interface for fetching external resources.
type resourceFetcherInterface interface {
	FetchSecrets(context.Context, *gax.CallOption, *secretmanager.Client, chan<- *Resource)
	FetchParameterVersions(context.Context, *gax.CallOption, *parametermanager.Client, chan<- *Resource)
}

type resourceFetcher struct {
	TypeOfResource ResourceType
	ResourceURI    string
	FileName       string
	Path           string
	MetricName     string
	Mode           *int32
	ExtractJSONKey string
	ExtractYAMLKey string
}

// Resource represents the Resource that is fetched.
type Resource struct {
	ID       string
	FileName string
	Path     string
	Version  string
	Payload  []byte
	Err      error
}

func (r *resourceFetcher) Orchestrator(ctx context.Context, s *Server, authOption *gax.CallOption, resultChan chan<- *Resource, wg *sync.WaitGroup) {
	defer wg.Done()
	if util.IsSecretResource(r.ResourceURI) {
		r.TypeOfResource = SecretRef
		location, err := util.ExtractLocationFromSecretResource(r.ResourceURI)
		if err != nil {
			resultChan <- getErrorResource(r.ResourceURI, r.FileName, r.Path, err)
			return
		}
		var smClient *secretmanager.Client
		if len(location) == 0 {
			smClient = s.SecretClient
		} else {
			smClient = s.RegionalSecretClients[location]
		}
		r.MetricName = "secretmanager_access_secret_version_requests"
		r.FetchSecrets(ctx, authOption, smClient, resultChan)
	} else if util.IsParameterManagerResource(r.ResourceURI) {
		r.TypeOfResource = ParameterVersion
		location, err := util.ExtractLocationFromParameterManagerResource(r.ResourceURI)
		if err != nil {
			resultChan <- getErrorResource(r.ResourceURI, r.FileName, r.Path, err)
			return
		}
		var pmClient *parametermanager.Client
		if len(location) == 0 {
			pmClient = s.ParameterManagerClient
		} else {
			pmClient = s.RegionalParameterManagerClients[location]
		}
		r.MetricName = "parametermanager_render_parameter_version_requests"
		r.FetchParameterVersions(ctx, authOption, pmClient, resultChan)
	} else {
		resultChan <- getErrorResource(
			r.ResourceURI,
			r.FileName,
			r.Path,
			fmt.Errorf("unknown resource type"),
		)
	}
}

func getErrorResource(resourceURI, fileName, path string, err error) *Resource {
	return &Resource{
		ID:       resourceURI,
		FileName: fileName,
		Path:     path,
		Payload:  nil,
		Err:      err,
	}
}
