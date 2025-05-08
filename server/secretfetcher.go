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
	fmt.Printf("\n\n+++++Request is %v++++++", request)
	fmt.Printf("\n\n+++++Response is %v++++++", response)
	fmt.Printf("\n\n+++++Error is %v++++++", err)
	fmt.Printf("\n\n****Client endpoint is %v*****", smClient)
	fmt.Printf("\n\n&&&&& Auth Option is %v&&&&&&", authOption)

	if err != nil {
		if e, ok := status.FromError(err); ok {
			smMetricRecorder(csrmetrics.OutboundRPCStatus(e.Code().String()))
		} else {
			// TODO: Keeping the same current implementation ->
			// But should we keep the status as okay when we have encountered an error?
			// In my opininon we should throw a default 500 error (rare case)
			smMetricRecorder(csrmetrics.OutboundRPCStatusOK)
		}
		resultChan <- getErrorResource(r.ResourceURI, r.FileName, err)
		return
	}
	if len(r.ExtractJSONKey) > 0 { // ExtractJSONKey populated
		content, err := util.ExtractContentUsingJSONKey(response.Payload.Data, r.ExtractJSONKey)
		if err != nil {
			resultChan <- getErrorResource(r.ResourceURI, r.FileName, err)
			return
		}
		resultChan <- &Resource{
			ID:       r.ResourceURI,
			FileName: r.FileName,
			Version:  response.GetName(),
			Payload:  content,
			Err:      nil,
		}
		return
	}
	resultChan <- &Resource{
		ID:       r.ResourceURI,
		FileName: r.FileName,
		Version:  response.GetName(),
		Payload:  response.Payload.Data,
		Err:      nil,
	}
}
