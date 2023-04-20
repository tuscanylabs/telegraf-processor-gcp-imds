package imds

// https://cloud.google.com/compute/docs/metadata/default-metadata-values

import (
	"net/http"
)

// Client provides the API client for interacting with the Instance Metadata Service API.
type Client struct {
	options Options
}

const (
	defaultIPv4Endpoint = "http://metadata.google.internal"
)

func New(options Options) *Client {
	var transport = &http.Transport{Proxy: nil}
	options.HTTPClient = http.Client{Transport: transport}

	client := &Client{
		options: options,
	}

	return client
}

// NewClient returns an initialized Client based on the functional options. Provide
// additional functional options to further configure the behavior of the client,
// such as changing the client's endpoint or adding custom middleware behavior.
func NewClient() *Client {
	opts := Options{
		Endpoint: defaultIPv4Endpoint,
		Format:   "json",
	}

	return New(opts)
}

// Options provides the fields for configuring the API client's behavior.
type Options struct {
	// The endpoint the client will use to retrieve instance metadata.
	Endpoint string

	// The HTTP client to invoke API calls with. Defaults to client's default
	// HTTP implementation if nil.
	HTTPClient http.Client

	Format string
}
