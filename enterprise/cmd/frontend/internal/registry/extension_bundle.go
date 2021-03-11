package registry

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	frontendregistry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
)

func init() {
	frontendregistry.HandleRegistryExtensionBundle = handleRegistryExtensionBundle
}

// sourceMappingURLLineRegex is a regular expression that matches all lines with a `//# sourceMappingURL` comment
var sourceMappingURLLineRegex = lazyregexp.New(`(?m)\r?\n?^//# sourceMappingURL=.+$`)

// handleRegistryExtensionBundle serves the bundled JavaScript source file or the source map for an
// extension in the registry as a raw JavaScript or JSON file.
func handleRegistryExtensionBundle(w http.ResponseWriter, r *http.Request) {
	if conf.Extensions() == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	filename := mux.Vars(r)["RegistryExtensionReleaseFilename"]
	wantSourceMap := filepath.Ext(filename) == ".map"

	releaseID, err := parseExtensionBundleFilename(filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	bundle, sourceMap, err := dbReleases{}.GetArtifacts(r.Context(), releaseID)
	if errcode.IsNotFound(err) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 🚨 SECURITY: Prevent this URL from being rendered as an HTML page by browsers (to prevent an
	// XSS attack). That would let attackers upload an HTML file with inline JavaScript and then
	// cause victims to visit it, thereby executing the attacker's JavaScript in the context of
	// Sourcegraph's domain.
	//
	// Note that it IS safe for the file to be served as application/javascript. If an attacker
	// references it in a <script> tag on the attacker's site, the JavaScript will execute in the
	// context of the attacker's site, not Sourcegraph. The script file being hosted by Sourcegraph
	// does not give it any privileges with respect to Sourcegraph's domain.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("X-Frame-Options", "deny")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-XSS-Protection", "1; mode=block")

	// 🚨 SECURITY sourcegraph.com only: downstream Sourcegraph sites' clients to access this file directly.
	// On private registries, requests to fetch extension bundles are authenticated, and Access-Control headers
	// should be preserved.
	if envvar.SourcegraphDotComMode() {
		w.Header().Del("Access-Control-Allow-Credentials") // credentials are not needed
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	// We want to cache forever because an extension release is immutable, except that if the
	// database is reset and and the registry_extension_releases.id sequence starts over, we don't
	// want stale data from pre-reset. So, assume that the presence of a query string means that it
	// includes some identifier that changes when the database is reset.
	if r.URL.RawQuery != "" {
		w.Header().Set("Cache-Control", "max-age=604800, private, immutable")
	}
	var data []byte
	if wantSourceMap {
		if sourceMap == nil {
			http.Error(w, "extension has no source map", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		data = sourceMap
	} else {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		data = sourceMappingURLLineRegex.ReplaceAll(bundle, []byte{})
	}
	_, _ = w.Write(data)

	if !wantSourceMap && sourceMap != nil {
		// Append `//# sourceMappingURL=` directive to JS bundle if we have a source map. It is
		// necessary to provide the absolute URL because the JS bundle is not loaded directly (e.g.,
		// via importScripts); it is saved to a blob URL and then executed, which means any relative
		// source map URL would be interpreted relative to the blob URL (so a relative URL wouldn't
		// work). Also, we can't rely on the original sourceMappingURL directive (if provided at
		// publish time) because it has no way of knowing the absolute URL to the source map.
		//
		// This implementation is not ideal because it means the JS bundle's contents depend on the
		// external URL, which makes it technically not immutable. But given the blob URL constraint
		// mentioned above, it's the best known solution.
		if externalURL, _ := url.Parse(conf.Get().ExternalURL); externalURL != nil {
			sourceMapURL := externalURL.ResolveReference(&url.URL{Path: path.Join(path.Dir(r.URL.Path), fmt.Sprintf("%d.map", releaseID))}).String()
			fmt.Fprintf(w, "\n//# sourceMappingURL=%s", sourceMapURL)
		}
	}
}

// parseExtensionBundleFilename parses the release ID from the extension bundle's filename, which is
// of the form "1234-publisher-extension-id.js" or ".map". The part of the filename after the "-"
// and before the extension is ignored; it exists to help distinguish log messages from different
// extensions in debugging.
func parseExtensionBundleFilename(filename string) (int64, error) {
	ext := filepath.Ext(filename)
	releaseIDStr := strings.TrimSuffix(filename, ext)
	if i := strings.Index(releaseIDStr, "-"); i != -1 {
		releaseIDStr = releaseIDStr[:i]
	}
	return strconv.ParseInt(releaseIDStr, 10, 64)
}
