package scim

import (
	"net/http"

	"github.com/elimity-com/scim"
	scimerrors "github.com/elimity-com/scim/errors"
	"github.com/elimity-com/scim/schema"

	"github.com/sourcegraph/sourcegraph/internal/scim/filter"
)

// Get returns the resource corresponding with the given identifier.
func (h *ResourceHandler) Get(r *http.Request, idStr string) (scim.Resource, error) {
	resource, err := h.service.Get(r.Context(), idStr)
	if err != nil {
		return scim.Resource{}, err
	}
	return resource, nil
}

// GetAll returns a paginated list of resources.
// An empty list of resources will be represented as `null` in the JSON response if `nil` is assigned to the
// Page.Resources. Otherwise, if an empty slice is assigned, an empty list will be represented as `[]`.
func (h *ResourceHandler) GetAll(r *http.Request, params scim.ListRequestParams) (scim.Page, error) {
	var totalCount int
	// We don't use `nil` for resources here, because Microsoft Entra fails the
	// connection check if for an empty set of resources we return `null`.
	resources := []scim.Resource{}
	var err error

	if params.Filter == nil {
		totalCount, resources, err = h.service.GetAll(r.Context(), params.StartIndex, &params.Count)

	} else {
		extensionSchemas := make([]schema.Schema, 0, len(h.schemaExtensions))
		for _, ext := range h.schemaExtensions {
			extensionSchemas = append(extensionSchemas, ext.Schema)
		}
		validator := filter.NewFilterValidator(params.Filter, h.coreSchema, extensionSchemas...)

		// Fetch all resources from the DB and then filter them here.
		// This doesn't feel efficient, but it wasn't reasonable to implement this in SQL in the time available.
		var allResources []scim.Resource
		// ignore the total count because it is calculated without the filter
		_, allResources, err = h.service.GetAll(r.Context(), 0, nil)
		for _, resource := range allResources {
			if err := validator.PassesFilter(resource.Attributes); err != nil {
				continue
			}

			totalCount++
			if totalCount >= params.StartIndex && len(resources) < params.Count {
				resources = append(resources, resource)
			}
			// No `break` here: the loop needs to continue even when `len(resources) >= params.Count`
			// because we want to put the total number of filtered users into `totalCount`.
		}
	}
	if err != nil {
		return scim.Page{}, scimerrors.ScimError{Status: http.StatusInternalServerError, Detail: err.Error()}
	}

	return scim.Page{
		TotalResults: totalCount,
		Resources:    resources,
	}, nil
}
