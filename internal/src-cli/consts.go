package srccli

// MinimumVersion is the minimum src-cli release version that works
// with this instance. This must be updated manually between releases.
// The public HTTP API will return this (or an updated patch version)
// as the suggested download with this instance.
//
// At the time of a Sourcegraph release, this is always the latest src-cli version.
const MinimumVersion = "5.5.0"
