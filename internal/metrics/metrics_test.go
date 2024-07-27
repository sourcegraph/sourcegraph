package metrics

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNoGo(t *testing.T) {
	t.Skip()
	resp, err := http.Get("https://example.com/") // Need to call Close()
	if err != nil {
		panic(err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = body
	_ = err
}

func testingHTTPClient(handler http.Handler) (*http.Client, func()) {
	s := httptest.NewServer(handler)

	cli := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, network, _ string) (net.Conn, error) {
				return net.Dial(network, s.Listener.Addr().String())
			},
		},
	}

	return cli, s.Close
}

func doRequest(hc *http.Client, u string) error {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}

	_, err = hc.Do(req)
	return err
}

func TestRequestMeterTransport(t *testing.T) {
	rm := NewRequestMeter("foosystem", "Total number of requests sent to foosystem.")

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond * 10)
		_, err := w.Write([]byte("the quick brown fox jumps over the lazy dog"))
		if err != nil {
			t.Error(err)
		}
	})
	hc, teardown := testingHTTPClient(h)
	defer teardown()

	hc.Transport = rm.Transport(hc.Transport, func(u *url.URL) string {
		return u.Path
	})

	err := doRequest(hc, "http://foosystem.com/apiCallA")
	if err != nil {
		t.Error(err)
	}

	err = doRequest(hc, "http://foosystem.com/apiCallB")
	if err != nil {
		t.Error(err)
	}

	c, err := rm.counter.GetMetricWith(map[string]string{
		labelCategory:  "/apiCallA",
		labelCode:      "200",
		labelHost:      "foosystem.com",
		labelTask:      "unknown",
		labelFromCache: "false",
	})
	if err != nil {
		t.Error(err)
	}
	val := testutil.ToFloat64(c)

	if val != 1.0 {
		t.Errorf("expected counter == 1, got %f", val)
	}
}

func TestMustRegisterDiskMonitor(t *testing.T) {
	registry := prometheus.NewPedanticRegistry()
	registerer = registry
	defer func() { registerer = prometheus.DefaultRegisterer }()

	want := []string{}
	for i := 0; i <= 2; i++ {
		path := t.TempDir()
		// Register twice to ensure we don't panic and we don't collect twice.
		MustRegisterDiskMonitor(path)
		MustRegisterDiskMonitor(path)
		want = append(want,
			fmt.Sprintf("src_disk_space_available_bytes{path=%s}", path),
			fmt.Sprintf("src_disk_space_total_bytes{path=%s}", path))
	}

	mfs, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, mf := range mfs {
		for _, m := range mf.Metric {
			var labels []string
			for _, l := range m.Label {
				labels = append(labels, fmt.Sprintf("%s=%s", *l.Name, *l.Value))
			}
			got = append(got, fmt.Sprintf("%s{%s}", *mf.Name, strings.Join(labels, " ")))
		}
	}

	sort.Strings(want)
	sort.Strings(got)
	if !cmp.Equal(want, got) {
		t.Errorf("mismatch (-want +got):\n%s", cmp.Diff(want, got))
	}
}
