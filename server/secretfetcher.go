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

func (r *resourceFetcher) FetchSecrets(ctx context.Context, authOption *gax.CallOption, smClient *secretmanager.Client, resultChan chan<- *Resource) {
	smMetricRecorder := csrmetrics.OutboundRPCStartRecorder(r.MetricName)
	request := &secretmanagerpb.AccessSecretVersionRequest{
		Name: r.ResourceURI,
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
		resultChan <- getErrorResource(r.ResourceURI, r.FileName, r.Path, err)
		return
	}
	smMetricRecorder(csrmetrics.OutboundRPCStatusOK)
	// Both simultaneously can't be populated.
	if len(r.ExtractJSONKey) > 0 && len(r.ExtractYAMLKey) > 0 {
		resultChan <- getErrorResource(
			r.ResourceURI,
			r.FileName,
			r.Path,
			fmt.Errorf(r.ResourceURI, "both ExtractJSONKey and ExtractYAMLKey can't be simultaneously non empty strings"),
		)
		return
	}
	if len(r.ExtractJSONKey) > 0 { // ExtractJSONKey populated
		content, err := util.ExtractContentUsingJSONKey(response.Payload.Data, r.ExtractJSONKey)
		if err != nil {
			resultChan <- getErrorResource(r.ResourceURI, r.FileName, r.Path, err)
			return
		}
		resultChan <- &Resource{
			ID:       r.ResourceURI,
			FileName: r.FileName,
			Path:     r.Path,
			Version:  response.GetName(),
			Payload:  content,
			Err:      nil,
		}
		return
	}
	if len(r.ExtractYAMLKey) > 0 { // ExtractYAMLKey populated
		content, err := util.ExtractContentUsingYAMLKey(response.Payload.Data, r.ExtractYAMLKey)
		if err != nil {
			resultChan <- getErrorResource(r.ResourceURI, r.FileName, r.Path, err)
			return
		}
		resultChan <- &Resource{
			ID:       r.ResourceURI,
			FileName: r.FileName,
			Path:     r.Path,
			Version:  response.GetName(),
			Payload:  content,
			Err:      nil,
		}
		return
	}
	resultChan <- &Resource{
		ID:       r.ResourceURI,
		FileName: r.FileName,
		Path:     r.Path,
		Version:  response.GetName(),
		Payload:  response.Payload.Data,
		Err:      nil,
	}
}
