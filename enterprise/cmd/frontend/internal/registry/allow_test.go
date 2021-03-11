package registry

import (
	"reflect"
	"sort"
	"testing"

	frontendregistry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/api"
	registry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/client"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestIsRemoteExtensionAllowed(t *testing.T) {
	defer licensing.TestingSkipFeatureChecks()()
	defer conf.Mock(nil)

	if !frontendregistry.IsRemoteExtensionAllowed("a") {
		t.Errorf("want %q to be allowed", "a")
	}

	conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{Extensions: &schema.Extensions{AllowRemoteExtensions: nil}}})
	if !frontendregistry.IsRemoteExtensionAllowed("a") {
		t.Errorf("want %q to be allowed", "a")
	}

	conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{Extensions: &schema.Extensions{AllowRemoteExtensions: []string{}}}})
	if frontendregistry.IsRemoteExtensionAllowed("a") {
		t.Errorf("want %q to be disallowed", "a")
	}

	conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{Extensions: &schema.Extensions{AllowRemoteExtensions: []string{"a"}}}})
	if !frontendregistry.IsRemoteExtensionAllowed("a") {
		t.Errorf("want %q to be allowed", "a")
	}
}

func sameElements(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aCopy := make([]string, len(a))
	bCopy := make([]string, len(b))

	copy(aCopy, a)
	copy(bCopy, b)

	sort.Strings(aCopy)
	sort.Strings(bCopy)

	return reflect.DeepEqual(aCopy, bCopy)
}

func TestFilterRemoteExtensions(t *testing.T) {
	defer licensing.TestingSkipFeatureChecks()()

	run := func(allowRemoteExtensions *[]string, extensions []string, want []string) {
		t.Helper()
		if allowRemoteExtensions != nil {
			conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{Extensions: &schema.Extensions{AllowRemoteExtensions: *allowRemoteExtensions}}})
			defer conf.Mock(nil)
		}
		var xs []*registry.Extension
		for _, id := range extensions {
			xs = append(xs, &registry.Extension{ExtensionID: id})
		}
		got := []string{}
		for _, x := range frontendregistry.FilterRemoteExtensions(xs) {
			got = append(got, x.ExtensionID)
		}
		if !sameElements(got, want) {
			t.Errorf("want %+v got %+v", want, got)
		}
	}

	run(nil, []string{}, []string{})
	run(nil, []string{"a"}, []string{"a"})
	run(&[]string{}, []string{}, []string{})
	run(&[]string{"a"}, []string{}, []string{})
	run(&[]string{}, []string{"a"}, []string{})
	run(&[]string{"a"}, []string{"b"}, []string{})
	run(&[]string{"a"}, []string{"a"}, []string{"a"})
	run(&[]string{"b", "c"}, []string{"a", "b", "c"}, []string{"b", "c"})
}
