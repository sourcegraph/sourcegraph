package geolocation

import (
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkInferCountryCode(b *testing.B) {
	exampleIPs := []string{
		"61.144.235.160",
		"93.184.216.34",
		"2606:2800:220:1:248:1893:25c8:1946",
	}
	for range b.N {
		_, err := InferCountryCode(exampleIPs[b.N%len(exampleIPs)])
		if err != nil {
			b.Log(err.Error())
			b.FailNow()
		}
	}
}

func TestInferCountryCode(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ipAddress string

		wantError       autogold.Value
		wantCountryCode autogold.Value
	}{
		{
			name:      "empty input",
			ipAddress: "",
			wantError: autogold.Expect("no IP address provided"),
		},
		{
			name:      "not an IP address",
			ipAddress: "sourcegraph.com",
			wantError: autogold.Expect(`invalid IP address "sourcegraph.com" provided`),
		},
		{
			name:            "example 1 valid IPv4",
			ipAddress:       "61.144.235.160", // example from OSS datasets
			wantCountryCode: autogold.Expect("CN"),
		},
		{
			name:            "example 2 valid IPv4",
			ipAddress:       "93.184.216.34", // ping -c1 example.net
			wantCountryCode: autogold.Expect("GB"),
		},
		{
			name:            "example valid IPv6",
			ipAddress:       "2606:2800:220:1:248:1893:25c8:1946", // ping6 -c1 example.net
			wantCountryCode: autogold.Expect("US"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, err := InferCountryCode(tc.ipAddress)
			if tc.wantError != nil {
				require.Error(t, err)
				tc.wantError.Equal(t, err.Error())
			} else {
				assert.NoError(t, err)
				tc.wantCountryCode.Equal(t, code)
			}
		})
	}
}
