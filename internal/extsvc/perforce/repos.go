package perforce

type DepotType string

const (
	Local   DepotType = "local"
	Remote  DepotType = "remote"
	Stream  DepotType = "stream"
	Spec    DepotType = "spec"
	Unload  DepotType = "unload"
	Archive DepotType = "archive"
	Tangent DepotType = "tangent"
	Graph   DepotType = "graph"
)

type StreamType string

const (
	Development StreamType = "development"
	Mainline    StreamType = "mainline"
	Release     StreamType = "release"
	Task        StreamType = "task"
	Virtual     StreamType = "virtual"
)

type ParentView string

const (
	Inherit   ParentView = "inherit"
	NoInherit ParentView = "noinherit"
)

// DepotSpec is a definiton of a depot that matches the format
// returned from `p4 -Mj -ztag depots`
type DepotSpec struct {
	Desc string `json:"desc,omitempty"`
	Map  string `json:"map,omitempty"`
	Name string `json:"name,omitempty"`
	// Time is seconds since the Epoch, but p4 quotes it in the output, so it's a string
	Time string    `json:"time,omitempty"`
	Type DepotType `json:"type,omitempty"`
}

// perforce.StreamSpec is a definiton of a depot stream that matches the format
// returned from `p4 -Mj -ztag streams`
type StreamSpec struct {
	// Access is seconds since the Epoch, but p4 quotes it in the output, so it's a string
	Access     string     `json:"Access,omitempty"`     // 1651172591
	Name       string     `json:"Name,omitempty"`       // stream2
	Options    string     `json:"Options,omitempty"`    // allsubmit unlocked toparent fromparent mergedown
	Owner      string     `json:"Owner,omitempty"`      // admin
	Parent     string     `json:"Parent,omitempty"`     // //stream-test/main
	ParentView ParentView `json:"ParentView,omitempty"` // inherit
	Stream     string     `json:"Stream,omitempty"`     // //stream-test/stream2
	Type       StreamType `json:"Type,omitempty"`       // development
	// Update is seconds since the Epoch, but p4 quotes it in the output, so it's a string
	Update                string `json:"Update,omitempty"`                // 1651172591
	BaseParent            string `json:"baseParent,omitempty"`            // //stream-test/main
	ChangeFlowsFromParent string `json:"changeFlowsFromParent,omitempty"` // true
	ChangeFlowsToParent   string `json:"changeFlowsToParent,omitempty"`   // true
	Desc                  string `json:"desc,omitempty"`                  // Created by admin.\n
	FirmerThanParent      string `json:"firmerThanParent,omitempty"`      // false
}

// Depot contains information of a Perforce depot.
type Depot struct {
	Depot string    `json:"depot"` // Depot is the path of the Perforce depot.
	Type  DepotType `json:"type"`  // Type is the type of the Perforce depot (we support "local" and "stream").
}
