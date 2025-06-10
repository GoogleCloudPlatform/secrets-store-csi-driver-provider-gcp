package server

import (
	"context"
	"fmt"

	parametermanager "cloud.google.com/go/parametermanager/apiv1"
	"cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/csrmetrics"
	"github.com/GoogleCloudPlatform/secrets-store-csi-driver-provider-gcp/util"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/grpc/status"
)

// This method calls the RenderAPI of parameter manager and stores the result in
// Resource chan where we store the resourceID and payload (also error if any)
func (r *resourceFetcher) FetchParameterVersions(ctx context.Context, authOption *gax.CallOption, pmClient *parametermanager.Client, resultChan chan<- *Resource) {
	pmMetricRecorder := csrmetrics.OutboundRPCStartRecorder(r.MetricName)
	request := &parametermanagerpb.RenderParameterVersionRequest{
		Name: r.ResourceURI,
	}
	response, err := pmClient.RenderParameterVersion(ctx, request, *authOption)
	if err != nil {
		if e, ok := status.FromError(err); ok {
			pmMetricRecorder(csrmetrics.OutboundRPCStatus(e.Code().String()))
		} else {
			// TODO: Keeping the same current implementation ->
			// But should we keep the status as okay when we have encountered an error?
			// In my opininon we should throw a default 500 error (rare case)
			pmMetricRecorder(csrmetrics.OutboundRPCStatusOK)
		}
		resultChan <- getErrorResource(r.ResourceURI, r.FileName, r.Path, err)
		return
	}
	pmMetricRecorder(csrmetrics.OutboundRPCStatusOK)
	// Both simultaneously can't be populated.
	if len(r.ExtractJSONKey) > 0 && len(r.ExtractYAMLKey) > 0 {
		resultChan <- getErrorResource(
			r.ResourceURI,
			r.FileName,
			r.Path,
			fmt.Errorf("both ExtractJSONKey and ExtractYAMLKey can't be simultaneously non empty strings"),
		)
		return
	}
	if len(r.ExtractJSONKey) > 0 { // ExtractJSONKey populated
		content, err := util.ExtractContentUsingJSONKey(response.RenderedPayload, r.ExtractJSONKey)
		if err != nil {
			resultChan <- getErrorResource(r.ResourceURI, r.FileName, r.Path, err)
			return
		}
		resultChan <- &Resource{
			ID:       r.ResourceURI,
			FileName: r.FileName,
			Path:     r.Path,
			Version:  response.GetParameterVersion(),
			Payload:  content,
			Err:      nil,
		}
		return
	}
	if len(r.ExtractYAMLKey) > 0 { // ExtractYAMLKey populated
		content, err := util.ExtractContentUsingYAMLKey(response.RenderedPayload, r.ExtractYAMLKey)
		if err != nil {
			resultChan <- getErrorResource(r.ResourceURI, r.FileName, r.Path, err)
			return
		}
		resultChan <- &Resource{
			ID:       r.ResourceURI,
			FileName: r.FileName,
			Path:     r.Path,
			Version:  response.GetParameterVersion(),
			Payload:  content,
			Err:      nil,
		}
		return
	}
	resultChan <- &Resource{
		ID:       r.ResourceURI,
		FileName: r.FileName,
		Path:     r.Path,
		Version:  response.GetParameterVersion(),
		Payload:  response.RenderedPayload,
		Err:      nil,
	}
}
