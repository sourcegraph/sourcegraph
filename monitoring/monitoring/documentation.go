package monitoring

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/prometheus/prometheus/model/labels"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	canonicalAlertDocsURL      = "https://docs.sourcegraph.com/admin/observability/alerts"
	canonicalDashboardsDocsURL = "https://docs.sourcegraph.com/admin/observability/dashboards"

	alertsDocsFile     = "alerts.md"
	dashboardsDocsFile = "dashboards.md"
)

const alertsReferenceHeader = `# Alerts reference

<!-- DO NOT EDIT: generated via: bazel run //doc/admin/observability:write_monitoring_docs -->

This document contains a complete reference of all alerts in Sourcegraph's monitoring, and next steps for when you find alerts that are firing.
If your alert isn't mentioned here, or if the next steps don't help, [contact us](mailto:support@sourcegraph.com) for assistance.

To learn more about Sourcegraph's alerting and how to set up alerts, see [our alerting guide](https://docs.sourcegraph.com/admin/observability/alerting).

`

const dashboardsHeader = `# Dashboards reference

<!-- DO NOT EDIT: generated via: bazel run //doc/admin/observability:write_monitoring_docs -->

This document contains a complete reference on Sourcegraph's available dashboards, as well as details on how to interpret the panels and metrics.

To learn more about Sourcegraph's metrics and how to view these dashboards, see [our metrics guide](https://sourcegraph.com/docs/admin/observability/metrics).

`

// fprintSubtitle prints subtitle-class text
func fprintSubtitle(w io.Writer, text string) {
	fmt.Fprintf(w, "<p class=\"subtitle\">%s</p>\n\n", text)
}

// Write a standardized Observable header that one can reliably generate an anchor link for.
//
// See `observableAnchor`.
func fprintObservableHeader(w io.Writer, c *Dashboard, o *Observable, headerLevel int) {
	fmt.Fprint(w, strings.Repeat("#", headerLevel))
	if o.Name == "" {
		// TODO: It seems that we have an issue here, it generates the following:
		// see https://gist.github.com/jhchabran/9ceaed75abe1a78136c200b6bc98c584
		// to help you understand where it's possibly broken.
		fmt.Fprintf(w, "%s:\n\n", strings.TrimSpace(c.Name))
	} else {
		fmt.Fprintf(w, " %s: %s\n\n", c.Name, o.Name)
	}
}

// fprintOwnedBy prints information about who owns a particular monitoring definition.
func fprintOwnedBy(w io.Writer, owner ObservableOwner) {
	fmt.Fprintf(w, "<sub>*Managed by the %s.*</sub>\n", owner.toMarkdown())
}

// Create an anchor link that matches `fprintObservableHeader`
//
// Must match Prometheus template in `docker-images/prometheus/cmd/prom-wrapper/receivers.go`
func observableDocAnchor(c *Dashboard, o Observable) string {
	observableAnchor := strings.ReplaceAll(o.Name, "_", "-")
	return fmt.Sprintf("%s-%s", c.Name, observableAnchor)
}

type documentation struct {
	alertDocs  bytes.Buffer
	dashboards bytes.Buffer

	injectLabelMatchers []*labels.Matcher
}

func renderDocumentation(containers []*Dashboard) (*documentation, error) {
	var docs documentation

	fmt.Fprint(&docs.alertDocs, alertsReferenceHeader)
	fmt.Fprint(&docs.dashboards, dashboardsHeader)

	for _, c := range containers {
		fmt.Fprintf(&docs.dashboards, "## %s\n\n", c.Title)
		fprintSubtitle(&docs.dashboards, c.Description)
		fmt.Fprintf(&docs.dashboards, "To see this dashboard, visit `/-/debug/grafana/d/%[1]s/%[1]s` on your Sourcegraph instance.\n\n", c.Name)

		for gIndex, g := range c.Groups {
			// the "General" group is top-level
			if g.Title != "General" {
				fmt.Fprintf(&docs.dashboards, "### %s: %s\n\n", c.Title, g.Title)
			}

			for rIndex, r := range g.Rows {
				for oIndex, o := range r {
					if err := docs.renderAlertSolutionEntry(c, o); err != nil {
						return nil, errors.Errorf("error rendering alert solution entry %q %q: %w",
							c.Name, o.Name, err)
					}
					docs.renderDashboardPanelEntry(c, o, observablePanelID(gIndex, rIndex, oIndex))
				}
			}
		}
	}

	return &docs, nil
}

func (d *documentation) renderAlertSolutionEntry(c *Dashboard, o Observable) error {
	if o.Warning == nil && o.Critical == nil {
		return nil
	}

	fprintObservableHeader(&d.alertDocs, c, &o, 2)
	fprintSubtitle(&d.alertDocs, o.Description)

	var alertQueryDetails []string
	var prometheusAlertNames []string // collect names for silencing configuration
	// Render descriptions of various levels of this alert
	fmt.Fprintf(&d.alertDocs, "**Descriptions**\n\n")
	for _, alert := range []struct {
		level     string
		threshold *ObservableAlertDefinition
	}{
		{level: "warning", threshold: o.Warning},
		{level: "critical", threshold: o.Critical},
	} {
		if alert.threshold.isEmpty() {
			continue
		}
		desc, err := c.alertDescription(o, alert.threshold)
		if err != nil {
			return err
		}
		fmt.Fprintf(&d.alertDocs, "- <span class=\"badge badge-%s\">%s</span> %s\n", alert.level, alert.level, desc)

		alertQuery, err := alert.threshold.generateAlertQuery(o, d.injectLabelMatchers, newVariableApplier(c.Variables))
		if err != nil {
			return err
		}
		if alert.threshold.customQuery != "" {
			alertQueryDetails = append(alertQueryDetails, fmt.Sprintf("Custom query for %s alert: `%s`", alert.level, alertQuery))
		} else {
			alertQueryDetails = append(alertQueryDetails, fmt.Sprintf("Generated query for %s alert: `%s`", alert.level, alertQuery))
		}

		prometheusAlertNames = append(prometheusAlertNames,
			fmt.Sprintf("  \"%s\"", prometheusAlertName(alert.level, c.Name, o.Name)))
	}
	fmt.Fprint(&d.alertDocs, "\n")

	// Render next steps for dealing with this alert
	fmt.Fprintf(&d.alertDocs, "**Next steps**\n\n")
	if o.NextSteps != "none" {
		nextSteps, _ := toMarkdown(o.NextSteps, true)
		fmt.Fprintf(&d.alertDocs, "%s\n", nextSteps)
	}
	if o.Interpretation != "" && o.Interpretation != "none" {
		// indicate help is available in dashboards reference
		fmt.Fprintf(&d.alertDocs, "- More help interpreting this metric is available in the [dashboards reference](./%s#%s).\n",
			dashboardsDocsFile, observableDocAnchor(c, o))
	} else {
		// just show the panel reference
		fmt.Fprintf(&d.alertDocs, "- Learn more about the related dashboard panel in the [dashboards reference](./%s#%s).\n",
			dashboardsDocsFile, observableDocAnchor(c, o))
	}
	// add silencing configuration as another solution
	fmt.Fprintf(&d.alertDocs, "- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:\n\n")
	fmt.Fprintf(&d.alertDocs, "```json\n%s\n```\n\n", fmt.Sprintf(`"observability.silenceAlerts": [
%s
]`, strings.Join(prometheusAlertNames, ",\n")))
	if o.Owner.opsgenieTeam != "" {
		// add owner
		fprintOwnedBy(&d.alertDocs, o.Owner)
	}

	if len(alertQueryDetails) > 0 {
		fmt.Fprintf(&d.alertDocs, `
<details>
<summary>Technical details</summary>

%s

</details>
`, strings.Join(alertQueryDetails, "\n\n"))
	}

	// render break for readability
	fmt.Fprint(&d.alertDocs, "\n<br />\n\n")
	return nil
}

func (d *documentation) renderDashboardPanelEntry(c *Dashboard, o Observable, panelID uint) {
	fprintObservableHeader(&d.dashboards, c, &o, 4)
	fprintSubtitle(&d.dashboards, upperFirst(o.Description))

	// render interpretation reference if available
	if o.Interpretation != "" && o.Interpretation != "none" {
		interpretation, _ := toMarkdown(o.Interpretation, false)
		fmt.Fprintf(&d.dashboards, "%s\n\n", strings.TrimSpace(interpretation))
	}

	// add link to alerts reference IF there is an alert attached
	if !o.NoAlert {
		fmt.Fprintf(&d.dashboards, "Refer to the [alerts reference](./%s#%s) for %s related to this panel.\n\n",
			alertsDocsFile, observableDocAnchor(c, o), pluralize("alert", o.alertsCount()))
	} else {
		fmt.Fprintf(&d.dashboards, "This panel has no related alerts.\n\n")
	}

	// how to get to this panel
	fmt.Fprintf(&d.dashboards, "To see this panel, visit `/-/debug/grafana/d/%[1]s/%[1]s?viewPanel=%[2]d` on your Sourcegraph instance.\n\n",
		c.Name, panelID)

	if o.Owner.opsgenieTeam != "" {
		// add owner
		fprintOwnedBy(&d.dashboards, o.Owner)
	}

	fmt.Fprintf(&d.dashboards, `
<details>
<summary>Technical details</summary>

Query:

`+"```\n%s\n```"+`
</details>
`, o.Query)

	// render break for readability
	fmt.Fprint(&d.dashboards, "\n<br />\n\n")
}
