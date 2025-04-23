package server

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/csrmetrics"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/util"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/grpc/status"
)

func (s *resourceFetcher) FetchSecrets(ctx context.Context, authOption *gax.CallOption, smClient *secretmanager.Client, resultChan chan<- *Resource) {
	smMetricRecorder := csrmetrics.OutboundRPCStartRecorder(s.MetricName)
	request := &secretmanagerpb.AccessSecretVersionRequest{
		Name: s.ResourceURI,
	}
	response, err := smClient.AccessSecretVersion(ctx, request, *authOption)
	if err != nil {
		if e, ok := status.FromError(err); ok {
			smMetricRecorder(csrmetrics.OutboundRPCStatus(e.Code().String()))
		} else {
			// TODO: Keeping the same current implementation ->
			// But should we keep the status as okay when we have encountered an error?
			// In my opininon we should throw a default 500 error (rare case)
			smMetricRecorder(csrmetrics.OutboundRPCStatusOK)
		}
		resultChan <- getErrorResource(s.ResourceURI, s.FileName, err)
		return
	}
	// Both simultaneously can't be populated.
	if len(s.ExtractJSONKey) > 0 && len(s.ExtractYAMLKey) > 0 {
		resultChan <- getErrorResource(
			s.ResourceURI,
			s.FileName,
			fmt.Errorf(s.ResourceURI, "both ExtractJSONKey and ExtractYAMLKey can't be simultaneously non empty strings"),
		)
	} else if len(s.ExtractJSONKey) > 0 { // ExtractJSONKey populated
		content, err := util.ExtractContentUsingJSONKey(response.Payload.Data, s.ExtractJSONKey)
		if err != nil {
			resultChan <- getErrorResource(s.ResourceURI, s.FileName, err)
			return
		}
		resultChan <- &Resource{
			ID:       s.ResourceURI,
			FileName: s.FileName,
			Version:  response.GetName(),
			Payload:  content,
			Err:      nil,
		}
	} else if len(s.ExtractYAMLKey) > 0 { // ExtractJSONKey populated
		content, err := util.ExtractContentUsingYAMLKey(response.Payload.Data, s.ExtractYAMLKey)
		if err != nil {
			resultChan <- getErrorResource(s.ResourceURI, s.FileName, err)
			return
		}
		resultChan <- &Resource{
			ID:       s.ResourceURI,
			FileName: s.FileName,
			Version:  response.GetName(),
			Payload:  content,
			Err:      nil,
		}
	} else {
		resultChan <- &Resource{
			ID:       s.ResourceURI,
			FileName: s.FileName,
			Version:  response.GetName(),
			Payload:  response.Payload.Data,
			Err:      nil,
		}
	}
}
