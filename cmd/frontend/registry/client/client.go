package client

const (
	// APIVersion is a string that uniquely identifies this API version.
	APIVersion = "20180621"

	// AcceptHeader is the value of the "Accept" HTTP request header sent by the client.
	AcceptHeader = "application/vnd.sourcegraph.api+json;version=" + APIVersion

	// MediaTypeHeaderName is the name of the HTTP response header whose value the client expects to
	// equal MediaType.
	MediaTypeHeaderName = "X-Sourcegraph-Media-Type"

	// MediaType is the client's expected value for the MediaTypeHeaderName HTTP response header.
	MediaType = "sourcegraph.v" + APIVersion + "; format=json"
)
