package background

import (
	"fmt"
	"time"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/sentinel/shared"
)

// Open Source Vulnerability format
// https://ossf.github.io/osv-schema/
type OSV struct {
	SchemaVersion string    `json:"schema_version"`
	ID            string    `json:"id"`
	Modified      time.Time `json:"modified"`
	Published     time.Time `json:"published"`
	Withdrawn     time.Time `json:"withdrawn"`
	Aliases       []string  `json:"aliases"`
	Related       []string  `json:"related"`
	Summary       string    `json:"summary"`
	Details       string    `json:"details"`
	Severity      []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
	Affected   []OSVAffected `json:"affected"`
	References []struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"references"`
	Credits []struct {
		Name    string   `json:"name"`
		Contact []string `json:"contact"`
	} `json:"credits"`
	DatabaseSpecific interface{} `json:"database_specific"`
}

type OSVAffected struct {
	Package struct {
		Ecosystem string `json:"ecosystem"`
		Name      string `json:"name"`
		Purl      string `json:"purl"`
	} `json:"package"`

	Ranges []struct {
		Type   string `json:"type"`
		Repo   string `json:"repo"`
		Events []struct {
			Introduced   string `json:"introduced"`
			Fixed        string `json:"fixed"`
			LastAffected string `json:"last_affected"`
			Limit        string `json:"limit"`
		} `json:"events"`
		DatabaseSpecific interface{} `json:"database_specific"`
	} `json:"ranges"`

	Versions          []string    `json:"versions"`
	EcosystemSpecific interface{} `json:"ecosystem_specific"`
	// EcosystemSpecific GoVulnDBAffectedEcosystemSpecific `json:"ecosystem_specific"`
	DatabaseSpecific interface{} `json:"database_specific"`
	// DatabaseSpecific map[string]string `json:"database_specific"` // TODO: Currently hardcoding GoVulndb format
}

type DataSourceHandler interface {
	topLevelHandler(OSV, *shared.Vulnerability) error
	affectedHandler(OSVAffected, *shared.AffectedPackage) error
}

// The Open Source Vulnerability (OSV) format is a common interchange format for exchanging
// vulnerability data. Some fields are common, and some fields are database-specific.
func osvToVuln(o OSV, dataSourceHandler DataSourceHandler) (vuln shared.Vulnerability, err error) {
	// Core sections:
	//	- /General details
	//  - Severity - TODO, need to loop over
	//	- /Affected - TODO add custom handlers
	//  - /References
	//  - Credits
	//  - /Database_specific

	v := shared.Vulnerability{
		SourceID:  o.ID,
		Summary:   o.Summary,
		Details:   o.Details,
		Published: o.Published,
		Modified:  &o.Modified,
		Withdrawn: &o.Withdrawn,
		Related:   o.Related,
		Aliases:   o.Aliases,
	}

	for _, reference := range o.References {
		v.URLs = append(v.URLs, reference.URL)
	}

	// Process top-level data with a database-specific handler
	if err := dataSourceHandler.topLevelHandler(o, &v); err != nil {
		return v, err
	}

	var pas []shared.AffectedPackage
	for _, affected := range o.Affected {
		var ap shared.AffectedPackage

		ap.PackageName = affected.Package.Name
		ap.Language = affected.Package.Ecosystem

		if err := dataSourceHandler.affectedHandler(affected, &ap); err != nil {
			return v, err
		}

		if len(affected.Ranges) > 1 {
			fmt.Printf("More than one affected range in %s\n", v.SourceID)
		}

		// In all observed cases a single range is used, so keep it simple
		for _, affectedRange := range affected.Ranges {
			// Implement dataSourceHandler.affectedRangeHandler here if needed

			for _, event := range affectedRange.Events {
				if event.Introduced != "" {
					ap.VersionConstraint = append(ap.VersionConstraint, ">="+event.Introduced)
				}
				if event.Fixed != "" {
					ap.VersionConstraint = append(ap.VersionConstraint, "<"+event.Fixed)
					ap.Fixed = true
					ap.FixedIn = event.Fixed
				}
				if event.LastAffected != "" {
					ap.VersionConstraint = append(ap.VersionConstraint, "<="+event.LastAffected)
				}
				if event.Limit != "" {
					ap.VersionConstraint = append(ap.VersionConstraint, "<="+event.Limit)
				}
			}
		}

		pas = append(pas, ap)
	}

	v.AffectedPackages = pas

	return v, nil
}
