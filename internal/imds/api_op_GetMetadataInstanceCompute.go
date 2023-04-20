package imds

import (
	"context"
	"encoding/json"
	"net/http"
)

const getInstanceMetadataPath = "/computeMetadata/v1/instance"

// GetInstanceIdentity retrieves an identity document describing an instance.
// Error is returned if the request fails or is unable to parse the response.
func (c *Client) GetInstanceMetadata(ctx context.Context, params *GetInstanceMetadataInput) (*GetMetadataInstanceOutput, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.options.Endpoint+getInstanceMetadataPath, nil)
	req.Header.Set("Metadata-Flavor", "Google")

	q := req.URL.Query()
	q.Add("recursive", "true")
	req.URL.RawQuery = q.Encode()

	resp, err := c.options.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	result, err := buildGetInstanceMetadataOutput(resp)
	if err != nil {
		return nil, err
	}

	out := result.(*GetMetadataInstanceOutput)
	return out, nil
}

// GetInstanceMetadataInput provides the input parameters for GetMetadataInstance operation.
type GetInstanceMetadataInput struct{}

// GetMetadataInstanceOutput provides the output parameters for GetMetadataInstance operation.
type GetMetadataInstanceOutput struct {
	InstanceIdentityDocument
}

func buildGetInstanceMetadataOutput(resp *http.Response) (v interface{}, err error) {
	output := &GetMetadataInstanceOutput{}
	if err = json.NewDecoder(resp.Body).Decode(&output.InstanceIdentityDocument); err != nil {
		return nil, err
	}

	return output, nil
}

// InstanceIdentityDocument provides the shape for unmarshaling an metadata instance document
type InstanceIdentityDocument struct {
	Hostname    string `json:"hostname,omitempty"`
	ID          string `json:"id,omitempty"`
	Image       string `json:"image,omitempty"`
	MachineType string `json:"machineType,omitempty"`
	Zone        string `json:"zone,omitempty"`
}
