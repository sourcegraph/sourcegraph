package shared

import (
	"fmt"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/monitoring/monitoring"
)

// Provisioning indicator overviews - these provide long-term overviews of container
// resource usage. The goal of these observables are to provide guidance on whether or not
// a service requires more or less resources.
//
// These observables should only use cAdvisor metrics, and are thus only available on
// Kubernetes and docker-compose deployments.
const TitleResourceUtilization = "Resource utilization (not available on server)"

var (
	ProvisioningCPUUsageLongTerm sharedObservable = func(containerName string, owner monitoring.ObservableOwner) Observable {
		return Observable{
			Name:        "provisioning_container_cpu_usage_long_term",
			Description: "container cpu usage total (90th percentile over 1d) across all cores by instance",
			Query:       fmt.Sprintf(`quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{%s}[1d])`, CadvisorContainerNameMatcher(containerName)),
			Warning:     monitoring.Alert().GreaterOrEqual(80).For(14 * 24 * time.Hour),
			Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Percentage).Max(100).Min(0),
			Owner:       owner,
			NextSteps: strings.ReplaceAll(`
			- **Kubernetes:** Consider increasing CPU limits in the 'Deployment.yaml' for the {{CONTAINER_NAME}} service.
			- **Docker Compose:** Consider increasing 'cpus:' of the {{CONTAINER_NAME}} container in 'docker-compose.yml'.
		`, "{{CONTAINER_NAME}}", containerName),
		}
	}

	ProvisioningMemoryUsageLongTerm sharedObservable = func(containerName string, owner monitoring.ObservableOwner) Observable {
		return Observable{
			Name:        "provisioning_container_memory_usage_long_term",
			Description: "container memory usage (1d maximum) by instance",
			Query:       fmt.Sprintf(`max_over_time(cadvisor_container_memory_usage_percentage_total{%s}[1d])`, CadvisorContainerNameMatcher(containerName)),
			Warning:     monitoring.Alert().GreaterOrEqual(80).For(14 * 24 * time.Hour),
			Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Percentage).Max(100).Min(0),
			Owner:       owner,
			NextSteps: strings.ReplaceAll(`
			- **Kubernetes:** Consider increasing memory limits in the 'Deployment.yaml' for the {{CONTAINER_NAME}} service.
			- **Docker Compose:** Consider increasing 'memory:' of the {{CONTAINER_NAME}} container in 'docker-compose.yml'.
		`, "{{CONTAINER_NAME}}", containerName),
		}
	}

	ProvisioningCPUUsageShortTerm sharedObservable = func(containerName string, owner monitoring.ObservableOwner) Observable {
		return Observable{
			Name:        "provisioning_container_cpu_usage_short_term",
			Description: "container cpu usage total (5m maximum) across all cores by instance",
			Query:       fmt.Sprintf(`max_over_time(cadvisor_container_cpu_usage_percentage_total{%s}[5m])`, CadvisorContainerNameMatcher(containerName)),
			Warning:     monitoring.Alert().GreaterOrEqual(90).For(30 * time.Minute),
			Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Percentage).Interval(100).Max(100).Min(0),
			Owner:       owner,
			NextSteps: strings.ReplaceAll(`
			- **Kubernetes:** Consider increasing CPU limits in the the relevant 'Deployment.yaml'.
			- **Docker Compose:** Consider increasing 'cpus:' of the {{CONTAINER_NAME}} container in 'docker-compose.yml'.
		`, "{{CONTAINER_NAME}}", containerName),
		}
	}

	ProvisioningMemoryUsageShortTerm sharedObservable = func(containerName string, owner monitoring.ObservableOwner) Observable {
		return Observable{
			Name:        "provisioning_container_memory_usage_short_term",
			Description: "container memory usage (5m maximum) by instance",
			Query:       fmt.Sprintf(`max_over_time(cadvisor_container_memory_usage_percentage_total{%s}[5m])`, CadvisorContainerNameMatcher(containerName)),
			Warning:     monitoring.Alert().GreaterOrEqual(90),
			Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Percentage).Interval(100).Max(100).Min(0),
			Owner:       owner,
			NextSteps: strings.ReplaceAll(`
			- **Kubernetes:** Consider increasing memory limit in relevant 'Deployment.yaml'.
			- **Docker Compose:** Consider increasing 'memory:' of {{CONTAINER_NAME}} container in 'docker-compose.yml'.
		`, "{{CONTAINER_NAME}}", containerName),
		}
	}

	ContainerOOMKILLEvents sharedObservable = func(containerName string, owner monitoring.ObservableOwner) Observable {
		return Observable{
			Name:        "container_oomkill_events_total",
			Description: "container OOMKILL events total by instance",
			Query:       fmt.Sprintf(`max by (name) (container_oom_events_total{%s})`, CadvisorContainerNameMatcher(containerName)),
			Warning:     monitoring.Alert().GreaterOrEqual(1),
			Panel:       monitoring.Panel().LegendFormat("{{name}}"),
			Owner:       owner,
			Interpretation: `
				This value indicates the total number of times the container main process or child processes were terminated by OOM killer.
				When it occurs frequently, it is an indicator of underprovisioning.
			`,
			NextSteps: strings.ReplaceAll(`
			- **Kubernetes:** Consider increasing memory limit in relevant 'Deployment.yaml'.
			- **Docker Compose:** Consider increasing 'memory:' of {{CONTAINER_NAME}} container in 'docker-compose.yml'.
		`, "{{CONTAINER_NAME}}", containerName),
		}
	}
)

type ContainerProvisioningIndicatorsGroupOptions struct {
	// LongTermCPUUsage transforms the default observable used to construct the long-term CPU usage panel.
	LongTermCPUUsage ObservableOption

	// LongTermMemoryUsage transforms the default observable used to construct the long-term memory usage panel.
	LongTermMemoryUsage ObservableOption

	// ShortTermCPUUsage transforms the default observable used to construct the short-term CPU usage panel.
	ShortTermCPUUsage ObservableOption

	// ShortTermMemoryUsage transforms the default observable used to construct the short-term memory usage panel.
	ShortTermMemoryUsage ObservableOption

	OOMKILLEvents ObservableOption

	// CustomTitle, if provided, provides a custom title for this provisioning group that will be displayed in Grafana.
	CustomTitle string
}

// NewProvisioningIndicatorsGroup creates a group containing panels displaying
// provisioning indication metrics - long and short term usage for both CPU and
// memory usage - for the given container.
func NewProvisioningIndicatorsGroup(containerName string, owner monitoring.ObservableOwner, options *ContainerProvisioningIndicatorsGroupOptions) monitoring.Group {
	if options == nil {
		options = &ContainerProvisioningIndicatorsGroupOptions{}
	}

	title := TitleResourceUtilization
	if options.CustomTitle != "" {
		title = options.CustomTitle
	}

	return monitoring.Group{
		Title:  title,
		Hidden: true,
		Rows: []monitoring.Row{
			{
				options.LongTermCPUUsage.safeApply(ProvisioningCPUUsageLongTerm(containerName, owner)).Observable(),
				options.LongTermMemoryUsage.safeApply(ProvisioningMemoryUsageLongTerm(containerName, owner)).Observable(),
			},
			{
				options.ShortTermCPUUsage.safeApply(ProvisioningCPUUsageShortTerm(containerName, owner)).Observable(),
				options.ShortTermMemoryUsage.safeApply(ProvisioningMemoryUsageShortTerm(containerName, owner)).Observable(),
				options.OOMKILLEvents.safeApply(ContainerOOMKILLEvents(containerName, owner)).Observable(),
			},
		},
	}
}
