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
const TitleProvisioningIndicators = "Provisioning indicators (not available on server)"

var (
	ProvisioningCPUUsageLongTerm sharedObservable = func(containerName string, owner monitoring.ObservableOwner) Observable {
		return Observable{
			Name:        "provisioning_container_cpu_usage_long_term",
			Description: "container cpu usage total (90th percentile over 1d) across all cores by instance",
			Query:       fmt.Sprintf(`quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{%s}[1d])`, CadvisorNameMatcher(containerName)),
			Warning:     monitoring.Alert().GreaterOrEqual(80, nil).For(14 * 24 * time.Hour),
			Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Percentage).Max(100).Min(0),
			Owner:       owner,
			PossibleSolutions: strings.ReplaceAll(`
			- **Kubernetes:** Consider increasing CPU limits in the 'Deployment.yaml' for the {{CONTAINER_NAME}} service.
			- **Docker Compose:** Consider increasing 'cpus:' of the {{CONTAINER_NAME}} container in 'docker-compose.yml'.
		`, "{{CONTAINER_NAME}}", containerName),
		}
	}

	ProvisioningMemoryUsageLongTerm sharedObservable = func(containerName string, owner monitoring.ObservableOwner) Observable {
		return Observable{
			Name:        "provisioning_container_memory_usage_long_term",
			Description: "container memory usage (1d maximum) by instance",
			Query:       fmt.Sprintf(`max_over_time(cadvisor_container_memory_usage_percentage_total{%s}[1d])`, CadvisorNameMatcher(containerName)),
			Warning:     monitoring.Alert().GreaterOrEqual(80, nil).For(14 * 24 * time.Hour),
			Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Percentage).Max(100).Min(0),
			Owner:       owner,
			PossibleSolutions: strings.ReplaceAll(`
			- **Kubernetes:** Consider increasing memory limits in the 'Deployment.yaml' for the {{CONTAINER_NAME}} service.
			- **Docker Compose:** Consider increasing 'memory:' of the {{CONTAINER_NAME}} container in 'docker-compose.yml'.
		`, "{{CONTAINER_NAME}}", containerName),
		}
	}

	ProvisioningCPUUsageShortTerm sharedObservable = func(containerName string, owner monitoring.ObservableOwner) Observable {
		return Observable{
			Name:        "provisioning_container_cpu_usage_short_term",
			Description: "container cpu usage total (5m maximum) across all cores by instance",
			Query:       fmt.Sprintf(`max_over_time(cadvisor_container_cpu_usage_percentage_total{%s}[5m])`, CadvisorNameMatcher(containerName)),
			Warning:     monitoring.Alert().GreaterOrEqual(90, nil).For(30 * time.Minute),
			Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Percentage).Interval(100).Max(100).Min(0),
			Owner:       owner,
			PossibleSolutions: strings.ReplaceAll(`
			- **Kubernetes:** Consider increasing CPU limits in the the relevant 'Deployment.yaml'.
			- **Docker Compose:** Consider increasing 'cpus:' of the {{CONTAINER_NAME}} container in 'docker-compose.yml'.
		`, "{{CONTAINER_NAME}}", containerName),
		}
	}

	ProvisioningMemoryUsageShortTerm sharedObservable = func(containerName string, owner monitoring.ObservableOwner) Observable {
		return Observable{
			Name:        "provisioning_container_memory_usage_short_term",
			Description: "container memory usage (5m maximum) by instance",
			Query:       fmt.Sprintf(`max_over_time(cadvisor_container_memory_usage_percentage_total{%s}[5m])`, CadvisorNameMatcher(containerName)),
			Warning:     monitoring.Alert().GreaterOrEqual(90, nil),
			Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Percentage).Interval(100).Max(100).Min(0),
			Owner:       owner,
			PossibleSolutions: strings.ReplaceAll(`
			- **Kubernetes:** Consider increasing memory limit in relevant 'Deployment.yaml'.
			- **Docker Compose:** Consider increasing 'memory:' of {{CONTAINER_NAME}} container in 'docker-compose.yml'.
		`, "{{CONTAINER_NAME}}", containerName),
		}
	}
)

type ContainerProvisioningIndicatorsGroupOptions struct {
	// LongTermCPUUsage transforms the default observable used to construct the long-term CPU usage panel.
	LongTermCPUUsage func(observable Observable) Observable

	// LongTermMemoryUsage transforms the default observable used to construct the long-term memory usage panel.
	LongTermMemoryUsage func(observable Observable) Observable

	// ShortTermCPUUsage transforms the default observable used to construct the short-term CPU usage panel.
	ShortTermCPUUsage func(observable Observable) Observable

	// ShortTermMemoryUsage transforms the default observable used to construct the short-term memory usage panel.
	ShortTermMemoryUsage func(observable Observable) Observable
}

// NewProvisioningIndicatorsGroup creates a group containing panels displaying
// provisioning indication metrics - long and short term usage for both CPU and
// memory usage - for the given container.
func NewProvisioningIndicatorsGroup(containerName string, owner monitoring.ObservableOwner, alerts *ContainerProvisioningIndicatorsGroupOptions) monitoring.Group {
	if alerts == nil {
		alerts = &ContainerProvisioningIndicatorsGroupOptions{}
	}
	if alerts.LongTermCPUUsage == nil {
		alerts.LongTermCPUUsage = NoopObservableTransformer
	}
	if alerts.LongTermMemoryUsage == nil {
		alerts.LongTermMemoryUsage = NoopObservableTransformer
	}
	if alerts.ShortTermCPUUsage == nil {
		alerts.ShortTermCPUUsage = NoopObservableTransformer
	}
	if alerts.ShortTermMemoryUsage == nil {
		alerts.ShortTermMemoryUsage = NoopObservableTransformer
	}

	return monitoring.Group{
		Title:  TitleProvisioningIndicators,
		Hidden: true,
		Rows: []monitoring.Row{
			{
				alerts.LongTermCPUUsage(ProvisioningCPUUsageLongTerm(containerName, owner)).Observable(),
				alerts.LongTermMemoryUsage(ProvisioningMemoryUsageLongTerm(containerName, owner)).Observable(),
			},
			{
				alerts.ShortTermCPUUsage(ProvisioningCPUUsageShortTerm(containerName, owner)).Observable(),
				alerts.ShortTermMemoryUsage(ProvisioningMemoryUsageShortTerm(containerName, owner)).Observable(),
			},
		},
	}
}
