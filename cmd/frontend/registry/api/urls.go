package api

import "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/ui/router"

// ExtensionURL returns the URL path to an extension.
var ExtensionURL = router.Extension

// PublisherExtensionsURL returns the URL path to a publisher's extensions.
var PublisherExtensionsURL = router.RegistryPublisherExtensions
