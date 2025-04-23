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
func (pm *resourceFetcher) FetchParameterVersions(ctx context.Context, authOption *gax.CallOption, pmClient *parametermanager.Client, resultChan chan<- *Resource) {
	pmMetricRecorder := csrmetrics.OutboundRPCStartRecorder(pm.MetricName)
	request := &parametermanagerpb.RenderParameterVersionRequest{
		Name: pm.ResourceURI,
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
		resultChan <- getErrorResource(pm.ResourceURI, pm.FileName, err)
		return
	}
	// Both simultaneously can't be populated.
	if len(pm.ExtractJSONKey) > 0 && len(pm.ExtractYAMLKey) > 0 {
		resultChan <- getErrorResource(
			pm.ResourceURI,
			pm.FileName,
			fmt.Errorf("both ExtractJSONKey and ExtractYAMLKey can't be simultaneously non empty strings"),
		)
	} else if len(pm.ExtractJSONKey) > 0 { // ExtractJSONKey populated
		content, err := util.ExtractContentUsingJSONKey(response.RenderedPayload, pm.ExtractJSONKey)
		if err != nil {
			resultChan <- getErrorResource(pm.ResourceURI, pm.FileName, err)
			return
		}
		resultChan <- &Resource{
			ID:       pm.ResourceURI,
			FileName: pm.FileName,
			Version:  response.GetParameterVersion(),
			Payload:  content,
			Err:      nil,
		}
	} else if len(pm.ExtractYAMLKey) > 0 { // ExtractJSONKey populated
		content, err := util.ExtractContentUsingYAMLKey(response.RenderedPayload, pm.ExtractYAMLKey)
		if err != nil {
			resultChan <- getErrorResource(pm.ResourceURI, pm.FileName, err)
			return
		}
		resultChan <- &Resource{
			ID:       pm.ResourceURI,
			FileName: pm.FileName,
			Version:  response.GetParameterVersion(),
			Payload:  content,
			Err:      nil,
		}
	} else {
		resultChan <- &Resource{
			ID:       pm.ResourceURI,
			FileName: pm.FileName,
			Version:  response.GetParameterVersion(),
			Payload:  response.RenderedPayload,
			Err:      nil,
		}
	}
}
