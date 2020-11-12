//go:generate echo "Regenerating monitoring..."
//go:generate go build -o /tmp/monitoring-generator
//go:generate /tmp/monitoring-generator

//nolint:golint,gocritic
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/grafana-tools/sdk"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v2"
)

// Container describes a Docker container to be observed.
type Container struct {
	// Name of the Docker container, e.g. "syntect-server".
	Name string

	// Title of the Docker container, e.g. "Syntect Server".
	Title string

	// Description of the Docker container. It should describe what the container
	// is responsible for, so that the impact of issues in it is clear.
	Description string

	// Groups of observable information about the container.
	Groups []Group
}

func (c *Container) validate() error {
	if !isValidUID(c.Name) {
		return fmt.Errorf("Container.Name must be lowercase alphanumeric + dashes; found \"%s\"", c.Name)
	}
	if c.Title != strings.Title(c.Title) {
		return fmt.Errorf("Container.Title must be in Title Case; found \"%s\" want \"%s\"", c.Title, strings.Title(c.Title))
	}
	if c.Description != withPeriod(c.Description) || c.Description != upperFirst(c.Description) {
		return fmt.Errorf("Container.Description must be sentence starting with an uppercas eletter and ending with period; found \"%s\"", c.Description)
	}
	for _, g := range c.Groups {
		if err := g.validate(); err != nil {
			return fmt.Errorf("group %q: %v", g.Title, err)
		}
	}
	return nil
}

// Group describes a group of observable information about a container.
type Group struct {
	// Title of the group, briefly summarizing what this group is about, or
	// "General" if the group is just about the container in general.
	Title string

	// Hidden indicates whether or not the group should be hidden by default.
	//
	// This should only be used when the dashboard is already full of information
	// and the information presented in this group is unlikely to be the cause of
	// issues and should generally only be inspected in the event that an alert
	// for that information is firing.
	Hidden bool

	// Rows of observable metrics.
	Rows []Row
}

func (g Group) validate() error {
	if g.Title != upperFirst(g.Title) || g.Title == withPeriod(g.Title) {
		return fmt.Errorf("Group.Title must start with an uppercase letter and not end with a period; found \"%s\"", g.Title)
	}
	for i, r := range g.Rows {
		if err := r.validate(); err != nil {
			return fmt.Errorf("row %d: %v", i, err)
		}
	}
	return nil
}

// Row of observable metrics.
type Row []Observable

func (r Row) validate() error {
	if len(r) < 1 || len(r) > 4 {
		return fmt.Errorf("row must have 1 to 4 observables only, found %v", len(r))
	}
	for _, o := range r {
		if err := o.validate(); err != nil {
			return fmt.Errorf("observable %q: %v", o.Name, err)
		}
	}
	return nil
}

// ObservableOwner denotes a team that owns an Observable. The current teams are described in
// the handbook: https://about.sourcegraph.com/company/team/org_chart#engineering
type ObservableOwner string

const (
	ObservableOwnerSearch       ObservableOwner = "search"
	ObservableOwnerCampaigns    ObservableOwner = "campaigns"
	ObservableOwnerCodeIntel    ObservableOwner = "code-intel"
	ObservableOwnerDistribution ObservableOwner = "distribution"
	ObservableOwnerSecurity     ObservableOwner = "security"
	ObservableOwnerWeb          ObservableOwner = "web"
	ObservableOwnerCloud        ObservableOwner = "cloud"
)

// Observable describes a metric about a container that can be observed. For example, memory usage.
type Observable struct {
	// Name is a short and human-readable lower_snake_case name describing what is being observed.
	//
	// It must be unique relative to the service name.
	//
	// Good examples:
	//
	//  github_rate_limit_remaining
	// 	search_error_rate
	//
	// Bad examples:
	//
	//  repo_updater_github_rate_limit
	// 	search_error_rate_over_5m
	//
	Name string

	// Description is a human-readable description of exactly what is being observed.
	//
	// Good examples:
	//
	// 	"remaining GitHub API rate limit quota"
	// 	"number of search errors every 5m"
	//  "90th percentile search request duration over 5m"
	//
	// Bad examples:
	//
	// 	"GitHub rate limit"
	// 	"search errors[5m]"
	// 	"P90 search latency"
	//
	Description string

	// Owner indicates the team that owns any alerts associated with this Observable.
	Owner ObservableOwner

	// Query is the actual Prometheus query that should be observed.
	Query string

	// DataMayNotExist indicates if the query may not return data until some event occurs in the
	// future.
	//
	// For example, repo_updater_memory_usage should always have data present and an alert should
	// fire if for some reason that query is not returning any data, so this would be set to false.
	// In contrast, search_error_rate would depend on users actually performing searches and we
	// would not want an alert to fire if no data was present, so this would be set to true.
	DataMayNotExist bool

	// DataMayNotBeNaN indicates whether or not the query may return NaN regularly.
	// In other words, when true, alerts will fire if the query returns NaN.
	//
	// NaN often indicates a mistaken divide by zero - for many types of alert queries,
	// this is a common problem on low-traffic deployments where the values of many
	// metrics frequently end up being 0, so the default is to allow it.
	//
	// However, for some queries NaN values may be unexpected, in which case you should
	// set this to true.
	DataMayNotBeNaN bool

	// A list of alert definitions associated with this Observable.
	Alerts []Alert

	// PossibleSolutions is Markdown describing possible solutions in the event that the alert is
	// firing. If there is no clear potential resolution, "none" must be explicitly stated.
	//
	// Contacting support should not be mentioned as part of a possible solution, as it is
	// communicated elsewhere.
	//
	// To make writing the Markdown more friendly in Go, string literals like this:
	//
	// 	Observable{
	// 		PossibleSolutions: `
	// 			- Foobar 'some code'
	// 		`
	// 	}
	//
	// Becomes:
	//
	// 	- Foobar `some code`
	//
	// In other words:
	//
	// 1. The preceding newline is removed.
	// 2. The indentation in the string literal is removed (based on the last line).
	// 3. Single quotes become backticks.
	// 4. The last line (which is all indention) is removed.
	PossibleSolutions string

	// PanelOptions describes some options for how to render the metric in the Grafana panel.
	PanelOptions panelOptions
}

func (o Observable) validate() error {
	if strings.Contains(o.Name, " ") || strings.ToLower(o.Name) != o.Name {
		return fmt.Errorf("Observable.Name must be in lower_snake_case; found \"%s\"", o.Name)
	}

	if v := string([]rune(o.Description)[0]); v != strings.ToLower(v) {
		return fmt.Errorf("Observable.Description must be lowercase; found \"%s\"", o.Description)
	}

	if o.Alerts == nil {
		return fmt.Errorf("%s: Alerts must be set. If you don't need alerts, set this field to NoAlerts", o.Name)
	}

	for i, a := range o.Alerts {
		if a.isEmpty() {
			return fmt.Errorf("%s: Alert #%d is empty", o.Name, i)
		}
	}

	if len(o.Alerts) > 0 {
		if o.PossibleSolutions == "" {
			return fmt.Errorf(`%s: PossibleSolutions: must list solutions or "none"`, o.Name)
		} else if o.PossibleSolutions != "none" {
			if _, err := goMarkdown(o.PossibleSolutions); err != nil {
				return fmt.Errorf(`%s: PossibleSolutions: %v`, o.Name, err)
			}
		}
	}

	if o.Owner == "" {
		return fmt.Errorf("%s: has now Owner set", o.Name)
	}

	return nil
}

// NoAlerts is used by Observables that don't need any alerts.
// We want to be explicit about this to ensure engineers think about it.
var NoAlerts = []Alert{}

// AlertLevel defines different levels of Alerts
type AlertLevel string

const (
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// Alert defines when an alert would be considered firing.
type Alert struct {
	// Level of the alert.
	Level AlertLevel

	// GreaterOrEqual, when non-zero, indicates the alert should fire when
	// greater or equal to this value.
	GreaterOrEqual *float32

	// LessOrEqual, when non-zero, indicates the alert should fire when less
	// than or equal to this value.
	LessOrEqual *float32

	// For indicates how long the given thresholds must be exceeded for this
	// alert to be considered firing. Defaults to 0s.
	For time.Duration
}

func Threshold(f float32) *float32 { return &f }

func (a Alert) isEmpty() bool { return a == Alert{} }

// UnitType for controlling the unit type display on graphs.
type UnitType string

// short returns the short string description of the unit, for qualifying a
// number of this unit type as human-readable.
func (u UnitType) short() string {
	switch u {
	case Number, "":
		return ""
	case Milliseconds:
		return "ms"
	case Seconds:
		return "s"
	case Percentage:
		return "%"
	case Bytes:
		return "B"
	case BitsPerSecond:
		return "bps"
	default:
		panic("never here")
	}
}

// From https://sourcegraph.com/github.com/grafana/grafana@b63b82976b3708b082326c0b7d42f38d4bc261fa/-/blob/packages/grafana-data/src/valueFormats/categories.ts#L23
const (
	// Number is the default unit type.
	Number UnitType = "short"

	// Milliseconds for representing time.
	Milliseconds UnitType = "dtdurationms"

	// Seconds for representing time.
	Seconds UnitType = "dtdurations"

	// Percentage in the range of 0-100.
	Percentage UnitType = "percent"

	// Bytes in IEC (1024) format, e.g. for representing storage sizes.
	Bytes UnitType = "bytes"

	// BitsPerSecond, e.g. for representing network and disk IO.
	BitsPerSecond UnitType = "bps"
)

type panelOptions struct {
	min, max     *float64
	minAuto      bool
	legendFormat string
	unitType     UnitType
	interval     string
}

// Min sets the minimum value of the Y axis on the panel. The default is zero.
func (p panelOptions) Min(min float64) panelOptions {
	p.min = &min
	return p
}

// Min sets the minimum value of the Y axis on the panel to auto, instead of
// the default zero.
//
// This is generally only useful if trying to show negative numbers.
func (p panelOptions) MinAuto() panelOptions {
	p.minAuto = true
	return p
}

// Max sets the maximum value of the Y axis on the panel. The default is auto.
func (p panelOptions) Max(max float64) panelOptions {
	p.max = &max
	return p
}

// LegendFormat sets the panel's legend format, which may use Go template strings to select
// labels from the Prometheus query.
func (p panelOptions) LegendFormat(format string) panelOptions {
	p.legendFormat = format
	return p
}

// Unit sets the panel's Y axis unit type.
func (p panelOptions) Unit(t UnitType) panelOptions {
	p.unitType = t
	return p
}

func (p panelOptions) Interval(ms int) panelOptions {
	p.interval = fmt.Sprintf("%dms", ms)
	return p
}

func (p panelOptions) withDefaults() panelOptions {
	if p.min == nil && !p.minAuto {
		defaultMin := 0.0
		p.min = &defaultMin
	}
	if p.legendFormat == "" {
		// Important: We use "value" as the default legend format and not, say, "{{instance}}" or
		// an empty string (Grafana defaults to all labels in that case) because:
		//
		// 1. Using "{{instance}}" is often wrong, see: https://about.sourcegraph.com/handbook/engineering/observability/monitoring_pillars#faq-why-can-t-i-create-a-graph-panel-with-more-than-5-cardinality-labels
		// 2. More often than not, you actually do want to aggregate your whole query with `sum()`, `max()` or similar.
		// 3. If "{{instance}}" or similar was the default, it would be easy for people to say "I guess that's intentional"
		//    instead of seeing multiple "value" labels on their dashboard (which immediately makes them think
		//    "how can I fix that?".)
		//
		p.legendFormat = "value"
	}
	if p.unitType == "" {
		p.unitType = Number
	}
	return p
}

func PanelOptions() panelOptions { return panelOptions{} }

var alertThresholdFillColors = map[AlertLevel]string{
	AlertLevelWarning:  "rgba(255, 73, 53, 0.8)",
	AlertLevelCritical: "rgba(255, 17, 36, 0.8)",
}

// dashboard generates the Grafana dashboard for this container.
func (c *Container) dashboard() *sdk.Board {
	board := sdk.NewBoard(c.Title)
	board.Version = uint(rand.Uint32())
	board.UID = c.Name
	board.ID = 0
	board.Timezone = "utc"
	board.Timepicker.RefreshIntervals = []string{"5s", "10s", "30s", "1m", "5m", "15m", "30m", "1h", "2h", "1d"}
	board.Time.From = "now-6h"
	board.Time.To = "now"
	board.SharedCrosshair = true
	board.AddTags("builtin")
	board.Templating.List = []sdk.TemplateVar{
		{
			Label:      "Filter alert level",
			Name:       "alert_level",
			AllValue:   ".*",
			Current:    sdk.Current{Text: "all", Value: "$__all"},
			IncludeAll: true,
			Options: []sdk.Option{
				{Text: "all", Value: "$__all", Selected: true},
				{Text: "critical", Value: "critical"},
				{Text: "warning", Value: "warning"},
			},
			Query: "critical,warning",
			Type:  "custom",
		},
	}

	description := sdk.NewText("")
	description.Title = "" // Removes vertical space the title would otherwise take up
	setPanelSize(description, 24, 3)
	description.TextPanel.Mode = "html"
	description.TextPanel.Content = fmt.Sprintf(`
	<div style="text-align: left;">
	  <img src="https://sourcegraphstatic.com/sourcegraph-logo-light.png" style="height:30px; margin:0.5rem"></img>
	  <div style="margin-left: 1rem; margin-top: 0.5rem; font-size: 20px;"><span style="color: #8e8e8e">%s:</span> %s <a style="font-size: 15px" target="_blank" href="https://docs.sourcegraph.com/dev/background-information/architecture">(⧉ architecture diagram)</a></span>
	</div>
	`, c.Name, c.Description)
	board.Panels = append(board.Panels, description)

	alertsDefined := sdk.NewTable("Alerts defined")
	setPanelSize(alertsDefined, 9, 5)
	setPanelPos(alertsDefined, 0, 3)
	alertsDefined.TablePanel.Sort = &sdk.Sort{Desc: true, Col: 4}
	alertsDefined.TablePanel.Styles = []sdk.ColumnStyle{
		{
			Pattern: "Time",
			Type:    "hidden",
		},
		{
			Pattern: "level",
			Type:    "hidden",
		},
		{
			Pattern: "_01_level",
			Alias:   stringPtr("level"),
		},
		{
			Pattern:     "Value",
			Alias:       stringPtr("firing?"),
			ColorMode:   stringPtr("row"),
			Colors:      &[]string{"rgba(50, 172, 45, 0.97)", "rgba(237, 129, 40, 0.89)", "rgba(245, 54, 54, 0.9)"},
			Thresholds:  &[]string{"0.99999", "1"},
			Type:        "string",
			MappingType: 1,
			ValueMaps: []sdk.ValueMap{
				{TextType: "false", Value: "0"},
				{TextType: "true", Value: "1"},
			},
		},
	}
	alertsDefined.AddTarget(&sdk.Target{
		Expr:    fmt.Sprintf(`label_replace(sum(max by (level,service_name,name,description)(alert_count{service_name="%s",name!="",level=~"$alert_level"})) by (level,description), "_01_level", "$1", "level", "(.*)")`, c.Name),
		Format:  "table",
		Instant: true,
	})
	board.Panels = append(board.Panels, alertsDefined)

	alertsFiring := sdk.NewGraph("Alerts firing")
	setPanelSize(alertsFiring, 15, 5)
	setPanelPos(alertsFiring, 9, 3)
	alertsFiring.GraphPanel.Legend.Show = true
	alertsFiring.GraphPanel.Fill = 1
	alertsFiring.GraphPanel.Bars = true
	alertsFiring.GraphPanel.NullPointMode = "null"
	alertsFiring.GraphPanel.Pointradius = 2
	alertsFiring.GraphPanel.AliasColors = map[string]string{}
	alertsFiring.GraphPanel.Xaxis = sdk.Axis{
		Show: true,
	}
	alertsFiring.GraphPanel.Yaxes = []sdk.Axis{
		{
			Decimals: 0,
			Format:   "short",
			LogBase:  1,
			Max:      sdk.NewFloatString(1),
			Min:      sdk.NewFloatString(0),
			Show:     true,
		},
		{
			Format:  "short",
			LogBase: 1,
			Show:    true,
		},
	}
	alertsFiring.AddTarget(&sdk.Target{
		Expr:         fmt.Sprintf(`sum by (service_name,level,name)(max by (level,service_name,name,description)(alert_count{service_name="%s",name!="",level=~"$alert_level"}) >= 1)`, c.Name),
		LegendFormat: "{{level}}: {{name}}",
	})
	board.Panels = append(board.Panels, alertsFiring)

	baseY := 8
	offsetY := baseY
	for _, group := range c.Groups {
		// Non-general groups are shown as collapsible panels.
		var rowPanel *sdk.Panel
		if group.Title != "General" {
			rowPanel = &sdk.Panel{RowPanel: &sdk.RowPanel{}}
			rowPanel.OfType = sdk.RowType
			rowPanel.Type = "row"
			rowPanel.Title = group.Title
			offsetY++
			setPanelPos(rowPanel, 0, offsetY)
			rowPanel.Collapsed = group.Hidden
			rowPanel.Panels = []sdk.Panel{} // cannot be null
			board.Panels = append(board.Panels, rowPanel)
		}

		// Generate a panel for displaying each observable in each row.
		for _, row := range group.Rows {
			panelWidth := 24 / len(row)
			offsetY++
			for i, o := range row {
				panelTitle := strings.ToTitle(string([]rune(o.Description)[0])) + string([]rune(o.Description)[1:])
				panel := sdk.NewGraph(panelTitle)
				setPanelSize(panel, panelWidth, 5)
				setPanelPos(panel, i*panelWidth, offsetY)
				panel.GraphPanel.Legend.Show = true
				panel.GraphPanel.Fill = 1
				panel.GraphPanel.Lines = true
				panel.GraphPanel.Linewidth = 1
				panel.GraphPanel.NullPointMode = "connected"
				panel.GraphPanel.Pointradius = 2
				panel.GraphPanel.AliasColors = map[string]string{}
				panel.GraphPanel.Xaxis = sdk.Axis{
					Show: true,
				}

				opt := o.PanelOptions.withDefaults()
				leftAxis := sdk.Axis{
					Decimals: 0,
					Format:   string(opt.unitType),
					LogBase:  1,
					Show:     true,
				}

				for _, a := range o.Alerts {
					if a.GreaterOrEqual != nil {
						panel.GraphPanel.Thresholds = append(panel.GraphPanel.Thresholds, sdk.Threshold{
							Value:     *a.GreaterOrEqual,
							Op:        "gt",
							ColorMode: "custom",
							Fill:      true,
							Line:      false,
							FillColor: alertThresholdFillColors[a.Level],
						})
					}

					if a.LessOrEqual != nil {
						panel.GraphPanel.Thresholds = append(panel.GraphPanel.Thresholds, sdk.Threshold{
							Value:     *a.LessOrEqual,
							Op:        "lt",
							ColorMode: "custom",
							Fill:      true,
							Line:      false,
							FillColor: alertThresholdFillColors[a.Level],
						})
					}
				}

				if opt.min != nil {
					leftAxis.Min = sdk.NewFloatString(*opt.min)
				}
				if opt.max != nil {
					leftAxis.Max = sdk.NewFloatString(*opt.max)
				}
				panel.GraphPanel.Yaxes = []sdk.Axis{
					leftAxis,
					{
						Format:  "short",
						LogBase: 1,
						Show:    true,
					},
				}
				panel.AddTarget(&sdk.Target{
					Expr:         o.Query,
					LegendFormat: opt.legendFormat,
					Interval:     opt.interval,
				})
				if rowPanel != nil && group.Hidden {
					rowPanel.RowPanel.Panels = append(rowPanel.RowPanel.Panels, *panel)
				} else {
					board.Panels = append(board.Panels, panel)
				}
			}
		}
	}
	return board
}

// alertDescription generates an alert description for the specified coontainer's alert.
func (c *Container) alertDescription(o Observable, alert Alert) string {
	if alert.isEmpty() {
		panic("never here")
	}
	var description string

	// description based on thresholds
	units := o.PanelOptions.unitType.short()
	if alert.GreaterOrEqual != nil && alert.LessOrEqual != nil {
		description = fmt.Sprintf("%s: %v%s+ or less than %v%s %s", c.Name, *alert.GreaterOrEqual, units, *alert.LessOrEqual, units, o.Description)
	} else if alert.GreaterOrEqual != nil {
		// e.g. "zoekt-indexserver: 20+ indexed search request errors every 5m by code"
		description = fmt.Sprintf("%s: %v%s+ %s", c.Name, *alert.GreaterOrEqual, units, o.Description)
	} else if alert.LessOrEqual != nil {
		// e.g. "zoekt-indexserver: less than 20 indexed search requests every 5m by code"
		description = fmt.Sprintf("%s: less than %v%s %s", c.Name, *alert.LessOrEqual, units, o.Description)
	} else {
		panic(fmt.Sprintf("unable to generate description for observable %+v", o))
	}

	// add information about "for"
	if alert.For > 0 {
		return fmt.Sprintf("%s for %s", description, alert.For)
	}
	return description
}

// promAlertsFile generates the Prometheus rules file which defines our
// high-level alerting metrics for the container. For more information about
// how these work, see:
//
// https://docs.sourcegraph.com/admin/observability/metrics_guide#high-level-alerting-metrics
//
func (c *Container) promAlertsFile() *promRulesFile {
	f := &promRulesFile{}
	group := promGroup{Name: c.Name}
	for _, g := range c.Groups {
		for _, r := range g.Rows {
			for _, o := range r {
				for _, a := range o.Alerts {
					if a.isEmpty() {
						continue
					}

					hasUpperAndLowerBounds := (a.GreaterOrEqual != nil) && (a.LessOrEqual != nil)
					makeLabels := func(bound string) map[string]string {
						var name, description string
						if hasUpperAndLowerBounds {
							// if both bounds are present, since we generate an alert for each bound
							// make sure the prometheus alert description only describes one bound
							name = fmt.Sprintf("%s_%s", o.Name, bound)
							if bound == "high" {
								description = c.alertDescription(o, Alert{
									GreaterOrEqual: a.GreaterOrEqual,
								})
							} else if bound == "low" {
								description = c.alertDescription(o, Alert{
									LessOrEqual: a.LessOrEqual,
								})
							} else {
								panic(fmt.Sprintf("never here, bad alert bound: %s", bound))
							}
						} else {
							name = o.Name
							description = c.alertDescription(o, a)
						}
						return map[string]string{
							"name":         name,
							"level":        string(a.Level),
							"service_name": c.Name,
							"description":  description,
							"owner":        string(o.Owner),
						}
					}

					// The alertQuery must contribute a query that returns a value < 1 when it is not
					// firing, or a value of >= 1 when it is firing.
					var alertQuery string

					// Replace NaN values with zero (not firing) or one (firing) if they are present.
					fireOnNan := "0"
					if o.DataMayNotBeNaN {
						fireOnNan = "1"
					}

					if a.GreaterOrEqual != nil {
						// By dividing the query value and the GreaterOrEqual value, we produce a
						// value of 1 when the query reaches the GreaterOrEqual value and < 1
						// otherwise. Examples:
						//
						// 	query_value=50 / GreaterOrEqual=50 == 1.0
						// 	query_value=25 / GreaterOrEqual=50 == 0.5
						// 	query_value=0 / GreaterOrEqual=50 == 0.0
						//
						alertQuery = fmt.Sprintf("(%s) / %v", o.Query, *a.GreaterOrEqual)

						// Replace no-data with zero values, so the alert does not fire, if desired.
						if o.DataMayNotExist {
							alertQuery = fmt.Sprintf("(%s) OR on() vector(0)", alertQuery)
						}

						alertQuery = fmt.Sprintf("((%s) >= 0) OR on() vector(%v)", alertQuery, fireOnNan)

						// Wrap the query in max() so that if there are multiple series (e.g. per-container) they
						// get flattened into a single one (we only support per-service alerts,
						// not per-container/replica).
						// More context: https://github.com/sourcegraph/sourcegraph/issues/11571#issuecomment-654571953
						group.AppendRow(fmt.Sprintf("max(%s)", alertQuery), makeLabels("high"), a.For)
					}
					if a.LessOrEqual != nil {
						//
						// 	LessOrEqual=50 / query_value=100 == 0.5
						// 	LessOrEqual=50 / query_value=50 == 1.0
						// 	LessOrEqual=50 / query_value=25 == 2.0
						// 	LessOrEqual=50 / query_value=0 (0.0000001) == 500000000
						// 	LessOrEqual=50 / query_value=-50 (0.0000001) == 500000000
						//
						alertQuery = fmt.Sprintf("%v / clamp_min(%s, 0.0000001)", *a.LessOrEqual, o.Query)

						// Replace no-data with zero values, so the alert does not fire, if desired.
						if o.DataMayNotExist {
							alertQuery = fmt.Sprintf("(%s) OR on() vector(0)", alertQuery)
						}

						alertQuery = fmt.Sprintf("((%s) >= 0) OR on() vector(%v)", alertQuery, fireOnNan)

						// Wrap the query in min() so that if there are multiple series (e.g. per-container) they
						// get flattened into a single one (we only support per-service alerts,
						// not per-container/replica).
						// More context: https://github.com/sourcegraph/sourcegraph/issues/11571#issuecomment-654571953
						group.AppendRow(fmt.Sprintf("min(%s)", alertQuery), makeLabels("low"), a.For)
					}
				}
			}
		}
	}
	f.Groups = append(f.Groups, group)
	return f
}

// isValidUID checks if the given string is a valid UID for entry into a Grafana dashboard. This is
// primarily used in the URL, e.g. /-/debug/grafana/d/syntect-server/<UID> and allows us to have
// static URLs we can document like:
//
// 	Go to https://sourcegraph.example.com/-/debug/grafana/d/syntect-server/syntect-server
//
// Instead of having to describe all the steps to navigate there because the UID is random.
func isValidUID(s string) bool {
	if s != strings.ToLower(s) {
		return false
	}
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-') {
			return false
		}
	}
	return true
}

// upperFirst returns s with an uppercase first rune.
func upperFirst(s string) string {
	return strings.ToUpper(string([]rune(s)[0])) + string([]rune(s)[1:])
}

// withPeriod returns s ending with a period.
func withPeriod(s string) string {
	if !strings.HasSuffix(s, ".") {
		return s + "."
	}
	return s
}

func generateDocs(containers []*Container) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, `# Alert solutions

This document contains possible solutions for when you find alerts are firing in Sourcegraph's monitoring.
If your alert isn't mentioned here, or if the solution doesn't help, [contact us](mailto:support@sourcegraph.com)
for assistance.

To learn more about Sourcegraph's alerting, see [our alerting documentation](https://docs.sourcegraph.com/admin/observability/alerting).

<!-- DO NOT EDIT: generated via: go generate ./monitoring -->

`)
	for _, c := range containers {
		for _, g := range c.Groups {
			for _, r := range g.Rows {
				for _, o := range r {
					if len(o.Alerts) == 0 {
						continue
					}

					fmt.Fprintf(&b, "## %s: %s\n\n", c.Name, o.Name)
					fmt.Fprintf(&b, `<p class="subtitle">%s: %s</p>`, o.Owner, o.Description)

					// Render descriptions of various levels of this alert
					fmt.Fprintf(&b, "**Descriptions:**\n\n")
					var prometheusAlertNames []string
					for _, alert := range o.Alerts {
						if alert.isEmpty() {
							continue
						}
						fmt.Fprintf(&b, "- _%s_\n", c.alertDescription(o, alert))
						prometheusAlertNames = append(prometheusAlertNames,
							fmt.Sprintf("  \"%s\"", prometheusAlertName(string(alert.Level), c.Name, o.Name)))

					}
					fmt.Fprint(&b, "\n")

					// Render solutions for dealing with this alert
					fmt.Fprintf(&b, "**Possible solutions:**\n\n")
					if o.PossibleSolutions != "none" {
						possibleSolutions, _ := goMarkdown(o.PossibleSolutions)
						fmt.Fprintf(&b, "%s\n", possibleSolutions)
					}

					// add silencing configuration as another solution
					fmt.Fprintf(&b, "- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:\n\n")
					fmt.Fprintf(&b, "```json\n%s\n```\n\n", fmt.Sprintf(`"observability.silenceAlerts": [
%s
]`, strings.Join(prometheusAlertNames, ",\n")))

					// Render break for readability
					fmt.Fprint(&b, "<br />\n")
				}
			}
		}
	}
	return b.Bytes()
}

func goMarkdown(m string) (string, error) {
	m = strings.TrimPrefix(m, "\n")

	// Replace single quotes with backticks.
	// Replace escaped single quotes with single quotes.
	m = strings.Replace(m, `\'`, `$ESCAPED_SINGLE_QUOTE`, -1)
	m = strings.Replace(m, `'`, "`", -1)
	m = strings.Replace(m, `$ESCAPED_SINGLE_QUOTE`, "'", -1)

	// Unindent based on the indention of the last line.
	lines := strings.Split(m, "\n")
	baseIndention := lines[len(lines)-1]
	if strings.TrimSpace(baseIndention) == "" {
		if strings.Contains(baseIndention, " ") {
			return "", errors.New("go string literal indention must be tabs")
		}
		indentionLevel := strings.Count(baseIndention, "\t")
		removeIndention := strings.Repeat("\t", indentionLevel+1)
		for i, l := range lines[:len(lines)-1] {
			newLine := strings.TrimPrefix(l, removeIndention)
			if l == newLine {
				return "", fmt.Errorf("inconsistent indention (line %d %q expected to start with %q)", i, l, removeIndention)
			}
			lines[i] = newLine
		}
		m = strings.Join(lines[:len(lines)-1], "\n")
	}

	// If result is not a list, make it a list, so we can add items.
	if !strings.HasPrefix(m, "-") && !strings.HasPrefix(m, "*") {
		m = fmt.Sprintf("- %s", m)
	}

	return m, nil
}

var isDev, _ = strconv.ParseBool(os.Getenv("DEV"))

const alertSuffix = "_alert_rules.yml"

func main() {
	grafanaDir, ok := os.LookupEnv("GRAFANA_DIR")
	if !ok {
		grafanaDir = "../docker-images/grafana/config/provisioning/dashboards/sourcegraph/"
	}
	prometheusDir, ok := os.LookupEnv("PROMETHEUS_DIR")
	if !ok {
		prometheusDir = "../docker-images/prometheus/config/"
	}
	docSolutionsFile, ok := os.LookupEnv("DOC_SOLUTIONS_FILE")
	if !ok {
		docSolutionsFile = "../doc/admin/observability/alert_solutions.md"
	}

	reloadValue, ok := os.LookupEnv("RELOAD")
	if !ok && isDev {
		reloadValue = "true"
	}
	reload, _ := strconv.ParseBool(reloadValue)

	containers := []*Container{
		Frontend(),
		GitServer(),
		GitHubProxy(),
		PreciseCodeIntelWorker(),
		QueryRunner(),
		RepoUpdater(),
		Searcher(),
		Symbols(),
		SyntectServer(),
		ZoektIndexServer(),
		ZoektWebServer(),
		Prometheus(),
		ExecutorAndExecutorQueue(),
	}
	var filelist []string
	for _, container := range containers {
		if err := container.validate(); err != nil {
			log.Fatal(fmt.Sprintf("container %q: %+v", container.Name, err))
		}
		if grafanaDir != "" {
			board := container.dashboard()
			data, err := json.MarshalIndent(board, "", "  ")
			if err != nil {
				log.Fatal(err)
			}
			// #nosec G306  prometheus runs as nobody
			err = ioutil.WriteFile(filepath.Join(grafanaDir, container.Name+".json"), data, 0666)
			if err != nil {
				log.Fatal(err)
			}
			filelist = append(filelist, container.Name+".json")

			if reload {
				ctx := context.Background()
				client := sdk.NewClient("http://127.0.0.1:3370", "admin:admin", sdk.DefaultHTTPClient)
				_, err := client.SetDashboard(ctx, *board, sdk.SetDashboardParams{Overwrite: true})
				if err != nil {
					log.Fatal("updating dashboard:", err)
				}
			}
		}

		if prometheusDir != "" {
			promAlertsFile := container.promAlertsFile()
			data, err := yaml.Marshal(promAlertsFile)
			if err != nil {
				log.Fatal(err)
			}
			fileName := strings.Replace(container.Name, "-", "_", -1) + alertSuffix
			filelist = append(filelist, fileName)
			// #nosec G306  grafana runs as UID 472
			err = ioutil.WriteFile(filepath.Join(prometheusDir, fileName), data, 0666)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	deleteRemnants(filelist, grafanaDir, prometheusDir)

	if prometheusDir != "" && reload {
		resp, err := http.Post("http://127.0.0.1:9090/-/reload", "", nil)
		if err != nil {
			log.Fatal("reloading Prometheus rules, got error:", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			log.Fatal("reloading Prometheus rules, got status code:", resp.StatusCode)
		}
	}
	if reload && grafanaDir != "" && prometheusDir != "" {
		fmt.Println("Reloaded Prometheus rules & Grafana dashboards")
	}

	if docSolutionsFile != "" {
		solutions := generateDocs(containers)
		// #nosec G306
		err := ioutil.WriteFile(docSolutionsFile, solutions, 0666)
		if err != nil {
			log.Fatal(err)
		}
	}
}
func deleteRemnants(filelist []string, grafanaDir, promDir string) {
	err := filepath.Walk(grafanaDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Print("Unable to access file: ", path)
			return nil
		}
		if filepath.Ext(path) != ".json" || info.IsDir() {
			return nil
		}
		for _, f := range filelist {
			if filepath.Ext(f) != ".json" || filepath.Ext(path) != ".json" || info.IsDir() {
				continue
			}
			if filepath.Base(path) == f {
				return nil
			}
		}
		err = os.Remove(path)
		log.Println("Removed orphan grafana file: ", path)
		return err
	})
	if err != nil {
		log.Fatal(err)
	}

	err = filepath.Walk(promDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Print("Unable to access file: ", path)
			return nil
		}
		if !strings.Contains(filepath.Base(path), alertSuffix) || info.IsDir() {
			return nil
		}

		for _, f := range filelist {
			if filepath.Ext(f) != ".yml" {
				continue
			}
			if filepath.Base(path) == f {
				return nil
			}
		}
		err = os.Remove(path)
		log.Println("Removed orphan prometheus alert file: ", path)
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
}

// promRulesFile represents a Prometheus recording rules file (which we use for defining our alerts)
// see:
//
// https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/
//
type promRulesFile struct {
	Groups []promGroup
}

type promGroup struct {
	Name  string
	Rules []promRule
}

func (g *promGroup) AppendRow(alertQuery string, labels map[string]string, duration time.Duration) {
	labels["alert_type"] = "builtin" // indicate alert is generated
	var forDuration *model.Duration
	if duration > 0 {
		d := model.Duration(duration)
		forDuration = &d
	}

	alertName := prometheusAlertName(labels["level"], labels["service_name"], labels["name"])
	g.Rules = append(g.Rules,
		// Native prometheus alert, based on alertQuery which returns 0 if not firing or 1 if firing.
		promRule{
			Alert:  alertName,
			Labels: labels,
			Expr:   fmt.Sprintf(`%s >= 1`, alertQuery),
			For:    forDuration,
		},
		// Record for generated alert, useful for indicating in Grafana dashboards if this alert
		// is defined at all. Prometheus's ALERTS metric does not track alerts with alertstate="inactive".
		//
		// Since ALERTS{alertname="value"} does not exist if the alert has never fired, we add set
		// the series to vector(0) instead.
		promRule{
			Record: "alert_count",
			Labels: labels,
			Expr:   fmt.Sprintf(`max(ALERTS{alertname=%q,alertstate="firing"} OR on() vector(0))`, alertName),
		})
}

type promRule struct {
	// either Record or Alert
	Record string `yaml:",omitempty"`
	Alert  string `yaml:",omitempty"`

	Labels map[string]string
	Expr   string

	// for Alert only
	For *model.Duration `yaml:",omitempty"`
}

// setPanelSize is a helper to set a panel's size.
func setPanelSize(p *sdk.Panel, width, height int) {
	p.GridPos.W = &width
	p.GridPos.H = &height
}

// setPanelSize is a helper to set a panel's position.
func setPanelPos(p *sdk.Panel, x, y int) {
	p.GridPos.X = &x
	p.GridPos.Y = &y
}

func stringPtr(s string) *string {
	return &s
}

// prometheusAlertName creates an alertname that is unique given the combination of parameters
func prometheusAlertName(level, service, name string) string {
	return fmt.Sprintf("%s_%s_%s", level, service, name)
}
