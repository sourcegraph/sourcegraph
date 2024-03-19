package precise

import (
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol"
)

type ID string

// MetaData contains data describing the overall structure of a bundle.
type MetaData struct {
	NumResultChunks int
}

// DocumentData represents a single document within an index. The data here can answer
// definitions, references, and hover queries if the results are all contained in the
// same document.
type DocumentData struct {
	Ranges             map[ID]RangeData
	HoverResults       map[ID]string // hover text normalized to markdown string
	Monikers           map[ID]MonikerData
	PackageInformation map[ID]PackageInformationData
	Diagnostics        []DiagnosticData
}

// RangeData represents a range vertex within an index. It contains the same relevant
// edge data, which can be subsequently queried in the containing document. The data
// that was reachable via a result set has been collapsed into this object during
// conversion.
type RangeData struct {
	StartLine              int  // 0-indexed, inclusive
	StartCharacter         int  // 0-indexed, inclusive
	EndLine                int  // 0-indexed, inclusive
	EndCharacter           int  // 0-indexed, inclusive
	DefinitionResultID     ID   // possibly empty
	ReferenceResultID      ID   // possibly empty
	ImplementationResultID ID   // possibly empty
	HoverResultID          ID   // possibly empty
	MonikerIDs             []ID // possibly empty
}

const (
	Local          = "local"
	Import         = "import"
	Export         = "export"
	Implementation = "implementation"
)

// MonikerData represent a unique name (eventually) attached to a range.
type MonikerData struct {
	Kind                 string // local, import, export, implementation
	Scheme               string // name of the package manager type
	Identifier           string // unique identifier
	PackageInformationID ID     // possibly empty
}

// PackageInformationData indicates a globally unique namespace for a moniker.
type PackageInformationData struct {
	// Name of the package manager.
	Manager string

	// Name of the package that contains the moniker.
	Name string

	// Version of the package.
	Version string
}

// QualifiedMonikerData pairs a moniker with its package information.
type QualifiedMonikerData struct {
	MonikerData
	PackageInformationData
}

// DiagnosticData carries diagnostic information attached to a range within its
// containing document.
type DiagnosticData struct {
	Severity       int
	Code           string
	Message        string
	Source         string
	StartLine      int // 0-indexed, inclusive
	StartCharacter int // 0-indexed, inclusive
	EndLine        int // 0-indexed, inclusive
	EndCharacter   int // 0-indexed, inclusive
}

// ResultChunkData represents a row of the resultChunk table. Each row is a subset
// of definition and reference result data in the index. Results are inserted into
// chunks based on the hash of their identifier, thus every chunk has a roughly
// proportional amount of data.
type ResultChunkData struct {
	// DocumentPaths is a mapping from document identifiers to their paths. This
	// must be used to convert a document identifier in DocumentIDRangeIDs into
	// a key that can be used to fetch document data.
	DocumentPaths map[ID]string

	// DocumentIDRangeIDs is a mapping from a definition or result reference
	// identifier to the set of ranges that compose that result set. Each range
	// is paired with the identifier of the document in which it can found.
	DocumentIDRangeIDs map[ID][]DocumentIDRangeID
}

// DocumentIDRangeID is a pair of document and range identifiers.
type DocumentIDRangeID struct {
	// The identifier of the document to which the range belongs. This id is only
	// relevant within the containing result chunk.
	DocumentID ID

	// The identifier of the range.
	RangeID ID
}

// DocumentPathRangeID denotes a range qualified by its containing document.
type DocumentPathRangeID struct {
	Path    string
	RangeID ID
}

// Loocation represents a range within a particular document relative to its
// containing bundle.
type LocationData struct {
	URI            string
	StartLine      int
	StartCharacter int
	EndLine        int
	EndCharacter   int
}

// MonikerLocations pairs a moniker scheme and identifier with the set of locations
// with that within a particular bundle.
type MonikerLocations struct {
	Kind       string
	Scheme     string
	Identifier string
	Locations  []LocationData
}

// KeyedDocumentData pairs a document with its path.
type KeyedDocumentData struct {
	Path     string
	Document DocumentData
}

// IndexedResultChunkData pairs a result chunk with its index.
type IndexedResultChunkData struct {
	Index       int
	ResultChunk ResultChunkData
}

// DocumentationNodeChild represents a child of a node.
type DocumentationNodeChild struct {
	// Node is non-nil if this child is another (non-new-page) node.
	Node *DocumentationNode `json:"node,omitempty"`

	// PathID is a non-empty string if this child is itself a new page.
	PathID string `json:"pathID,omitempty"`
}

// DocumentationNode describes one node in a tree of hierarchial documentation.
type DocumentationNode struct {
	// PathID is the path ID of this node itself.
	PathID        string                   `json:"pathID"`
	Documentation protocol.Documentation   `json:"documentation"`
	Label         protocol.MarkupContent   `json:"label"`
	Detail        protocol.MarkupContent   `json:"detail"`
	Children      []DocumentationNodeChild `json:"children"`
}

// DocumentationPageData describes a single page of documentation.
type DocumentationPageData struct {
	Tree *DocumentationNode
}

// DocumentationPathInfoData describes a single documentation path, what is located there and what
// pages are below it.
type DocumentationPathInfoData struct {
	// The pathID for this entry.
	PathID string `json:"pathID"`

	// IsIndex tells if the page at this path is an empty index page whose only purpose is to describe
	// all the pages below it.
	IsIndex bool `json:"isIndex"`

	// Children is a list of the children page paths immediately below this one.
	Children []string `json:"children,omitempty"`
}

// DocumentationMapping maps a documentationResult vertex ID to its path IDs, which are unique in
// the context of a bundle.
type DocumentationMapping struct {
	// ResultID is the documentationResult vertex ID.
	ResultID uint64 `json:"resultID"`

	// PathID is the path ID corresponding to the documentationResult vertex ID.
	PathID string `json:"pathID"`

	// The file path corresponding to the documentationResult vertex ID, or nil if there is no
	// associated file.
	FilePath *string `json:"filePath"`
}

// DocumentationSearchResult describes a single documentation search result, from the
// lsif_data_docs_search_public or lsif_data_docs_search_private table.
type DocumentationSearchResult struct {
	ID        int64
	RepoID    int32
	UploadID  int32
	DumpRoot  string
	PathID    string
	Detail    string
	Lang      string
	RepoName  string
	Tags      []string
	SearchKey string
	Label     string
}

// Package pairs a package name and the dump that provides it.
type Package struct {
	Scheme  string
	Manager string
	Name    string
	Version string
}

func (pi *Package) LessThan(pj *Package) bool {
	if pi.Scheme == pj.Scheme {
		if pi.Manager == pj.Manager {
			if pi.Name == pj.Name {
				return pi.Version < pj.Version
			}

			return pi.Name < pj.Name
		}

		return pi.Manager < pj.Manager
	}
	return pi.Scheme < pj.Scheme
}

// PackageReferences pairs a package name/version with a dump that depends on it.
type PackageReference struct {
	Package
}

// GroupedBundleData{Chans,Maps} is a view of a correlation State that sorts data by it's containing document
// and shared data into sharded result chunks. The fields of this type are what is written to
// persistent storage and what is read in the query path. The Chans version allows pipelining
// and parallelizing the work, while the Maps version can be modified for e.g. local development
// via the REPL or patching for incremental indexing.
type GroupedBundleDataChans struct {
	ProjectRoot       string
	Meta              MetaData
	Documents         chan KeyedDocumentData
	ResultChunks      chan IndexedResultChunkData
	Definitions       chan MonikerLocations
	References        chan MonikerLocations
	Implementations   chan MonikerLocations
	Packages          []Package
	PackageReferences []PackageReference
}

type GroupedBundleDataMaps struct {
	Meta              MetaData
	Documents         map[string]DocumentData
	ResultChunks      map[int]ResultChunkData
	Definitions       map[string]map[string]map[string][]LocationData
	References        map[string]map[string]map[string][]LocationData
	Packages          []Package
	PackageReferences []PackageReference
}
