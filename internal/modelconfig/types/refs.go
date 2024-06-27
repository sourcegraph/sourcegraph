package types

import "strings"

type ProviderID string

type APIVersionID string

type ModelID string

// `${ProviderID}::${APIVersionID}`
type ProviderApiVersionRef string

// `${ProviderID}::${APIVersionID}::${ModelID}`
type ModelRef string

// Assuming the ModelRef is valid, returns the ModelID.
// Will return junk data if the ModelRef is invalid.
func (mref ModelRef) ModelID() ModelID {
	lastSep := strings.LastIndex(string(mref), "::")
	if lastSep == -1 {
		return "error-invalid-modelref"
	}
	return ModelID(string(mref)[lastSep+2:])
}

// Assuming the ModelRef is valid, returns the ProviderID.
// Will return junk data if the ModelRef is invalid.
func (mref ModelRef) ProviderID() ProviderID {
	firstSep := strings.Index(string(mref), "::")
	if firstSep == -1 {
		return "error-invalid-modelref"
	}
	return ProviderID(string(mref)[:firstSep])
}
