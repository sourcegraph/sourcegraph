package api

import "net/http"

// HandleRegistryExtensionBundle is called to handle HTTP requests for an extension's JavaScript
// bundle and other assets. If there is no local extension registry, it returns an HTTP error
// response.
var HandleRegistryExtensionBundle = func(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "no local extension registry exists", http.StatusNotFound)
}
