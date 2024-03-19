package endpoint

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestNew(t *testing.T) {
	m := New("http://test")
	expectEndpoints(t, m, "http://test")

	m = New("http://test-1 http://test-2")
	expectEndpoints(t, m, "http://test-1", "http://test-2")
}

func TestStatic(t *testing.T) {
	m := Static("http://test")
	expectEndpoints(t, m, "http://test")

	m = Static("http://test-1", "http://test-2")
	expectEndpoints(t, m, "http://test-1", "http://test-2")
}

func TestStatic_empty(t *testing.T) {
	m := Static()
	expectEndpoints(t, m)

	// Empty maps should fail on Get but not on Endpoints
	_, err := m.Get("foo")
	if _, ok := err.(*EmptyError); !ok {
		t.Fatal("Get should return EmptyError")
	}

	_, err = m.GetN("foo", 5)
	if _, ok := err.(*EmptyError); !ok {
		t.Fatal("GetN should return EmptyError")
	}

	_, err = m.GetMany("foo")
	if _, ok := err.(*EmptyError); !ok {
		t.Fatal("GetMany should return EmptyError")
	}

	eps, err := m.Endpoints()
	if err != nil {
		t.Fatal("Endpoints should not return an error")
	}
	if len(eps) != 0 {
		t.Fatal("Endpoints should be empty")
	}
}

func TestGetN(t *testing.T) {
	endpoints := []string{"http://test-1", "http://test-2", "http://test-3", "http://test-4"}
	m := Static(endpoints...)

	node, _ := m.Get("foo")
	have, _ := m.GetN("foo", 3)

	if len(have) != 3 {
		t.Fatalf("GetN(3) didn't return 3 nodes")
	}

	if have[0] != node {
		t.Fatalf("GetN(foo, 3)[0] != Get(foo): %s != %s", have[0], node)
	}

	want := []string{"http://test-3", "http://test-2", "http://test-4"}
	if !reflect.DeepEqual(have, want) {
		t.Fatalf("GetN(\"foo\", 3):\nhave: %v\nwant: %v", have, want)
	}
}

func expectEndpoints(t *testing.T, m *Map, endpoints ...string) {
	t.Helper()

	// We ask for the URL of a large number of keys, we expect to see every
	// endpoint and only those endpoints.
	count := map[string]int{}
	for _, e := range endpoints {
		count[e] = 0
	}
	for i := range len(endpoints) * 10 {
		v, err := m.Get(fmt.Sprintf("test-%d", i))
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if _, ok := count[v]; !ok {
			t.Fatalf("map returned unexpected endpoint %v. Valid: %v", v, endpoints)
		}
		count[v] = count[v] + 1
	}
	t.Logf("counts: %v", count)
	for e, c := range count {
		if c == 0 {
			t.Fatalf("map never returned %v", e)
		}
	}

	// Ensure GetMany matches Get
	var keys, vals []string
	for i := range len(endpoints) * 10 {
		keys = append(keys, fmt.Sprintf("test-%d", i))
		v, err := m.Get(keys[i])
		if err != nil {
			t.Fatalf("Get for GetMany failed: %v", err)
		}
		vals = append(vals, v)
	}
	if got, err := m.GetMany(keys...); err != nil {
		t.Fatalf("GetMany failed: %v", err)
	} else if diff := cmp.Diff(vals, got, cmpopts.EquateEmpty()); diff != "" {
		t.Fatalf("GetMany(%v) unexpected response (-want, +got):\n%s", keys, diff)
	}
}

func TestEndpoints(t *testing.T) {
	want := []string{"http://test-1", "http://test-2", "http://test-3", "http://test-4"}
	m := Static(want...)
	got, err := m.Endpoints()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("m.Endpoints() unexpected return:\ngot:  %v\nwant: %v", got, want)
	}
}

func TestSync(t *testing.T) {
	eps := make(chan endpoints, 1)
	defer close(eps)

	urlspec := "http://test"
	m := &Map{
		urlspec: urlspec,
		discofunk: func(disco chan endpoints) {
			for {
				v, ok := <-eps
				if !ok {
					return
				}
				disco <- v
			}
		},
	}

	// Test that we block m.Get() until eps sends its first value
	want := []string{"a", "b"}
	eps <- endpoints{
		Service:   urlspec,
		Endpoints: want,
	}
	expectEndpoints(t, m, want...)

	// We now rely on sync, so we retry until we see what we want. Set an
	// error.
	eps <- endpoints{
		Service: urlspec,
		Error:   errors.New("boom"),
	}
	if !waitUntil(5*time.Second, func() bool {
		_, err := m.Get("test")
		return err != nil
	}) {
		t.Fatal("expected map to return error")
	}

	eps <- endpoints{
		Service:   urlspec,
		Endpoints: want,
	}
	if !waitUntil(5*time.Second, func() bool {
		_, err := m.Get("test")
		return err == nil
	}) {
		t.Fatal("expected map to recover from error")
	}
}

// waitUntil will wait d. It will return early when pred returns true.
// Otherwise it will return pred() after d.
func waitUntil(d time.Duration, pred func() bool) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if pred() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return pred()
}
