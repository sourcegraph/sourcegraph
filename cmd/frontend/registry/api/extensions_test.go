package api

import (
	"context"
	"reflect"
	"testing"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	registry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/client"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

func TestSplitExtensionID(t *testing.T) {
	tests := map[string]struct {
		wantPrefix    string
		wantPublisher string
		wantName      string
		wantErr       bool
	}{
		"":        {wantErr: true},
		"/":       {wantErr: true},
		"a/":      {wantErr: true},
		"/a":      {wantErr: true},
		"//":      {wantErr: true},
		"b/c":     {wantPublisher: "b", wantName: "c"},
		"a/b/c":   {wantPrefix: "a", wantPublisher: "b", wantName: "c"},
		"a/b/c/d": {wantPrefix: "a/b", wantPublisher: "c", wantName: "d"},
	}
	for extensionID, test := range tests {
		t.Run(extensionID, func(t *testing.T) {
			prefix, publisher, name, err := SplitExtensionID(extensionID)
			if (err != nil) != test.wantErr {
				t.Errorf("got error %v, want error? %v", err, test.wantErr)
			}
			if err != nil {
				return
			}
			if prefix != test.wantPrefix {
				t.Errorf("got prefix %q, want %q", prefix, test.wantPrefix)
			}
			if publisher != test.wantPublisher {
				t.Errorf("got publisher %q, want %q", publisher, test.wantPublisher)
			}
			if name != test.wantName {
				t.Errorf("got name %q, want %q", name, test.wantName)
			}
		})
	}
}

func TestParseExtensionID(t *testing.T) {
	tests := map[string]struct {
		mockConfiguredPrefix         string
		wantPrefix                   string
		wantExtensionIDWithoutPrefix string
		wantIsLocal                  bool
		wantErr                      bool
	}{
		"":      {wantErr: true},
		"b/c":   {wantExtensionIDWithoutPrefix: "b/c", wantIsLocal: true},
		"a/b/c": {wantErr: true},
		"x/y/z": {mockConfiguredPrefix: "x", wantPrefix: "x", wantExtensionIDWithoutPrefix: "y/z", wantIsLocal: true},
		"y/z":   {mockConfiguredPrefix: "x", wantExtensionIDWithoutPrefix: "y/z", wantIsLocal: false},
		"w/y/z": {mockConfiguredPrefix: "x", wantErr: true},
	}
	for extensionID, test := range tests {
		t.Run(extensionID, func(t *testing.T) {
			var tmp *string
			if test.mockConfiguredPrefix != "" {
				tmp = &test.mockConfiguredPrefix
			}
			mockLocalRegistryExtensionIDPrefix = &tmp
			defer func() { mockLocalRegistryExtensionIDPrefix = nil }()

			prefix, extensionIDWithoutPrefix, isLocal, err := ParseExtensionID(extensionID)
			if (err != nil) != test.wantErr {
				t.Errorf("got error %v, want error? %v", err, test.wantErr)
			}
			if err != nil {
				return
			}
			if prefix != test.wantPrefix {
				t.Errorf("got prefix %q, want %q", prefix, test.wantPrefix)
			}
			if extensionIDWithoutPrefix != test.wantExtensionIDWithoutPrefix {
				t.Errorf("got extensionIDWithoutPrefix %q, want %q", extensionIDWithoutPrefix, test.wantExtensionIDWithoutPrefix)
			}
			if isLocal != test.wantIsLocal {
				t.Errorf("got isLocal %v, want %v", isLocal, test.wantIsLocal)
			}
		})
	}
}

type mockRegistryExtension struct {
	id   int32
	name string
	graphqlbackend.RegistryExtension
}

func TestGetExtensionByExtensionID(t *testing.T) {
	ctx := context.Background()
	db := new(dbtesting.MockDB)

	t.Run("root", func(t *testing.T) {
		mockLocalRegistryExtensionIDPrefix = &strnilptr
		defer func() { mockLocalRegistryExtensionIDPrefix = nil }()

		t.Run("2-part", func(t *testing.T) {
			GetLocalExtensionByExtensionID = func(ctx context.Context, db dbutil.DB, extensionID string) (graphqlbackend.RegistryExtension, error) {
				if want := "a/b"; extensionID != want {
					t.Errorf("got %q, want %q", extensionID, want)
				}
				return &mockRegistryExtension{id: 1, name: "b"}, nil
			}
			defer func() { GetLocalExtensionByExtensionID = nil }()
			local, remote, err := GetExtensionByExtensionID(ctx, db, "a/b")
			if err != nil {
				t.Fatal(err)
			}
			if want := (&mockRegistryExtension{id: 1, name: "b"}); !reflect.DeepEqual(local, want) {
				t.Errorf("got %+v, want %+v", local, want)
			}
			if remote != nil {
				t.Error()
			}
		})

		t.Run("3-part", func(t *testing.T) {
			if _, _, err := GetExtensionByExtensionID(ctx, db, "a/b/c"); err == nil {
				t.Fatal()
			}
		})
	})

	t.Run("non-root", func(t *testing.T) {
		mockLocalRegistryExtensionIDPrefix = strptrptr("x")
		defer func() { mockLocalRegistryExtensionIDPrefix = nil }()

		t.Run("2-part", func(t *testing.T) {
			mockGetRemoteRegistryExtension = func(field, value string) (*registry.Extension, error) {
				if want := "extensionID"; field != want {
					t.Errorf("got field %q, want %q", field, want)
				}
				if want := "a/b"; value != want {
					t.Errorf("got value %q, want %q", value, want)
				}
				return &registry.Extension{UUID: "u", ExtensionID: "a/b"}, nil
			}
			defer func() { mockGetRemoteRegistryExtension = nil }()
			local, remote, err := GetExtensionByExtensionID(ctx, db, "a/b")
			if err != nil {
				t.Fatal(err)
			}
			if local != nil {
				t.Error()
			}
			if want := (&registry.Extension{UUID: "u", ExtensionID: "a/b"}); !reflect.DeepEqual(remote, want) {
				t.Errorf("got %+v, want %+v", remote, want)
			}
		})

		t.Run("3-part", func(t *testing.T) {
			GetLocalExtensionByExtensionID = func(ctx context.Context, db dbutil.DB, extensionID string) (graphqlbackend.RegistryExtension, error) {
				if want := "b/c"; extensionID != want {
					t.Errorf("got %q, want %q", extensionID, want)
				}
				return &mockRegistryExtension{id: 1, name: "b"}, nil
			}
			defer func() { GetLocalExtensionByExtensionID = nil }()
			local, remote, err := GetExtensionByExtensionID(ctx, db, "x/b/c")
			if err != nil {
				t.Fatal(err)
			}
			if want := (&mockRegistryExtension{id: 1, name: "b"}); !reflect.DeepEqual(local, want) {
				t.Errorf("got %+v, want %+v", local, want)
			}
			if remote != nil {
				t.Error()
			}
		})
	})

	t.Run("invalid extension ID", func(t *testing.T) {
		if _, _, err := GetExtensionByExtensionID(ctx, db, "a/b/c/d"); err == nil {
			t.Fatal()
		}
	})
}

func TestIsWorkInProgressExtension(t *testing.T) {
	tests := map[*string]bool{
		nil:                                        true,
		strptr(`{`):                                false,
		strptr(`{}`):                               false,
		strptr(`{"title":null}`):                   false,
		strptr(`{"title":""}`):                     false,
		strptr(`{"title":"a b"}`):                  false,
		strptr(`{"title":"WIP: a"}`):               true,
		strptr(`{"title":"[WIP] a"}`):              true,
		strptr(`{"wip": true, "title":"a"}`):       true,
		strptr(`{"wip": false, "title":"a"}`):      false,
		strptr(`{"wip": false, "title":"WIP: a"}`): true,
	}
	for manifest, want := range tests {
		got := IsWorkInProgressExtension(manifest)
		if got != want {
			var label string
			if manifest == nil {
				label = "nil"
			} else {
				label = *manifest
			}
			t.Errorf("got %v, want %v (manifest: %s)", got, want, label)
		}
	}
}

var strnilptr *string

func strptrptr(s string) **string {
	tmp := &s
	return &tmp
}
