package perforce

type PerforceDepotType string

const (
	Local   PerforceDepotType = "local"
	Remote  PerforceDepotType = "remote"
	Stream  PerforceDepotType = "stream"
	Spec    PerforceDepotType = "spec"
	Unload  PerforceDepotType = "unload"
	Archive PerforceDepotType = "archive"
	Tangent PerforceDepotType = "tangent"
	Graph   PerforceDepotType = "graph"
)

// Depot contains information of a Perforce depot.
type Depot struct {
	// Date is the date at which the depot has been created at.
	Date string `json:"Date"`
	// Depot is the path of the Perforce depot.
	//
	// This field is Depot in perforce output, but was lowercase in our code previously,
	// so not using an explicit json tag here will make it support both.
	Depot       string
	Description string            `json:"Description"`
	Map         string            `json:"Map"`
	Owner       string            `json:"Owner"`
	Type        PerforceDepotType `json:"Type"`
}

// PerforceDepot is a definiton of a depot that matches the format
// returned from `p4 -Mj -ztag depots`
// TODO: Reconcile with newly added type.
type PerforceDepot struct {
	Desc string `json:"desc,omitempty"`
	Map  string `json:"map,omitempty"`
	Name string `json:"name,omitempty"`
	// Time is seconds since the Epoch, but p4 quotes it in the output, so it's a string
	Time string `json:"time,omitempty"`
	// Type is local, remote, stream, spec, unload, archive, tangent, graph
	Type PerforceDepotType `json:"type,omitempty"`
}
