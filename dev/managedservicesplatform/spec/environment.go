package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/alecthomas/units"
	"github.com/hashicorp/cronexpr"

	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/imageupdater"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

type EnvironmentSpec struct {
	// ID is an all-lowercase alphanumeric identifier for the deployment
	// environment, e.g. "prod" or "dev".
	ID string `yaml:"id"`

	// ProjectID is the generated Google Project ID for this environment,
	// provided by either 'sg msp init' or 'sg msp init -env'.
	//
	// The format is:
	//
	// 	$SERVICE_ID-$ENVIRONMENT_ID-$RANDOM_SUFFIX
	//
	// ❗ This value cannot be changed after your environment has been
	// initialized!
	ProjectID string `yaml:"projectID"`

	// Category is either "test", "internal", or "external". It informs the
	// which GCP project folder the environment lives in:
	//
	// - 'test': 'Engineering Projects' folder (liberal access)
	// - 'internal': 'Internal Services' folder (restricted access)
	// - 'external': 'Managed Services' folder (restricted access)
	//
	// It also informs what kind of notification channels are set up out-of-the-box:
	//
	// 1. 'test' services only generate Slack notifications.
	// 2. 'internal' and 'external' services generate Slack and Opsgenie notifications.
	//
	// Slack channels are expected to be named '#alerts-<service>-<environmentName>'.
	// Opsgenie teams are expected to correspond to service owners.
	//
	// Both Slack channels and Opsgenie teams are currently expected to be manually
	// configured.
	Category EnvironmentCategory `yaml:"category"`

	// Deploy specifies how to deploy revisions of the service image.
	Deploy EnvironmentDeploySpec `yaml:"deploy"`

	// Locations specifies details for the desired geographical location of
	// resources provisioned for this environment. If omitted, default
	// locations are used (namely, the GCP region 'us-central1'). If provided,
	// all fields must be specified.
	Locations *EnvironmentLocationsSpec `yaml:"locations,omitempty"`

	// EnvironmentServiceSpec carries service-specific configuration.
	*EnvironmentServiceSpec `yaml:",inline"`
	// EnvironmentJobSpec carries job-specific configuration.
	*EnvironmentJobSpec `yaml:",inline"`

	// Instances describes how machines running the service are deployed.
	Instances EnvironmentInstancesSpec `yaml:"instances"`

	// Env are key-value pairs of environment variables to set on the service.
	//
	// Values can be subsituted with supported runtime values with gotemplate, e.g., "{{ .ProjectID }}"
	// 	- ProjectID: The project ID of the service.
	//	- ServiceDnsName: The DNS name of the service.
	Env map[string]string `yaml:"env,omitempty"`

	// SecretEnv are key-value pairs of environment variables sourced from
	// secrets set on the service, where the value is the name of the secret
	// in the service's project to populate in the environment.
	//
	// To point to a secret in another project, use the format
	// 'projects/{project}/secrets/{secretName}' in the value. Access to the
	// target project will be automatically granted.
	SecretEnv map[string]string `yaml:"secretEnv,omitempty"`

	// SecretVolumes configures volumes to mount from secrets. Keys are used
	// as volume names.
	SecretVolumes map[string]EnvironmentSecretVolume `yaml:"secretVolumes,omitempty"`

	// Resources configures additional resources that a service may depend on.
	Resources *EnvironmentResourcesSpec `yaml:"resources,omitempty"`

	// Alerting configures alerting and notifications for the environment.
	Alerting *EnvironmentAlertingSpec `yaml:"alerting,omitempty"`

	// AllowDestroys, if false, configures Terraform lifecycle guards against
	// deletion of potentially critical resources. This includes things like the
	// environment project and databases, and also guards against the deletion
	// of Terraform Cloud workspaces as well.
	// https://developer.hashicorp.com/terraform/tutorials/state/resource-lifecycle#prevent-resource-deletion
	//
	// To tear down an environment, or to apply a change that intentionally
	// causes deletion on guarded resources, set this to true and apply the
	// generated Terraform first.
	AllowDestroys *bool `yaml:"allowDestroys,omitempty"`
}

func (s EnvironmentSpec) Validate() []error {
	var errs []error

	// Validate basic configuration
	if s.ID == "" {
		errs = append(errs, errors.New("id is required"))
	}
	if s.ProjectID == "" {
		errs = append(errs, errors.New("projectID is required"))
	}
	if len(s.ProjectID) > 30 {
		errs = append(errs, errors.Newf("projectID %q must be less than 30 characters", s.ProjectID))
	}
	if !strings.Contains(s.ProjectID, fmt.Sprintf("-%s-", s.ID)) {
		errs = append(errs, errors.Newf("projectID %q must contain environment ID: expecting format '$SERVICE_ID-$ENVIRONMENT_ID-$RANDOM_SUFFIX'",
			s.ProjectID))
	}
	if err := s.Category.Validate(); err != nil {
		return append(errs, errors.Wrap(err, "category"))
	}

	// Validate other shared sub-specs
	errs = append(errs, s.Deploy.Validate()...)
	errs = append(errs, s.GetLocationSpec().Validate()...)
	errs = append(errs, s.Resources.Validate()...)
	errs = append(errs, s.Instances.Validate()...)
	for k, v := range s.SecretVolumes {
		if k == "" {
			errs = append(errs, errors.New("secretVolumes key cannot be empty"))
		}
		errs = append(errs, v.Validate()...)
	}

	// Validate service-specific specs
	errs = append(errs, s.EnvironmentServiceSpec.Validate()...)

	return errs
}

type EnvironmentCategory string

const (
	// EnvironmentCategoryTest should be used for testing and development
	// environments.
	EnvironmentCategoryTest EnvironmentCategory = "test"
	// EnvironmentCategoryInternal should be used for internal environments.
	EnvironmentCategoryInternal EnvironmentCategory = "internal"
	// EnvironmentCategoryExternal is the default category if none is specified.
	EnvironmentCategoryExternal EnvironmentCategory = "external"
)

func (c EnvironmentCategory) Validate() error {
	switch c {
	case EnvironmentCategoryTest,
		EnvironmentCategoryInternal,
		EnvironmentCategoryExternal:
	case "":
		return errors.New("no category provided")
	default:
		return errors.Newf("invalid category %q", c)
	}
	return nil
}

func (c EnvironmentCategory) IsProduction() bool {
	return c == EnvironmentCategoryExternal || c == EnvironmentCategoryInternal
}

type EnvironmentDeployType string

const (
	EnvironmentDeployTypeManual       = "manual"
	EnvironmentDeployTypeSubscription = "subscription"
	EnvironmentDeployTypeRollout      = "rollout"
)

type EnvironmentDeploySpec struct {
	// Type specifies the deployment method for the environment. There are
	// 3 supported types:
	//
	//  - 'manual': Revisions are deployed manually by configuring it in 'deploy.manual.tag'
	//  - 'subscription': Revisions are deployed via GitHub Action, which pins to the latest image SHA of 'deploy.subscription.tag'.
	//  - 'rollout': Revisions are deployed via Cloud Deploy - an env-level 'rollout' spec is required, and a 'rollout.clouddeploy.yaml' is rendered with further instructions.
	Type         EnvironmentDeployType                  `yaml:"type"`
	Manual       *EnvironmentDeployManualSpec           `yaml:"manual,omitempty"`
	Subscription *EnvironmentDeployTypeSubscriptionSpec `yaml:"subscription,omitempty"`
}

func (s EnvironmentDeploySpec) Validate() []error {
	var errs []error
	switch s.Type {
	case EnvironmentDeployTypeManual:
		if s.Subscription != nil {
			errs = append(errs, errors.New("subscription deploy spec provided when type is manual"))
		}
	case EnvironmentDeployTypeSubscription:
		if s.Manual != nil {
			errs = append(errs, errors.New("manual deploy spec provided when type is subscription"))
		} else if s.Subscription == nil {
			errs = append(errs, errors.New("no subscription specified when deploy type is subscription"))
		} else if s.Subscription.Tag == "" {
			errs = append(errs, errors.New("no tag in image subscription specified"))
		}
	case EnvironmentDeployTypeRollout:
		// no validation
	default:
		errs = append(errs, errors.Newf("invalid deploy type %q", s.Type))
	}
	return errs
}

type EnvironmentDeployManualSpec struct {
	// Tag is the tag to deploy. If empty, defaults to "insiders".
	Tag string `yaml:"tag,omitempty"`
}

// GetTag returns the tag to deploy. If empty, defaults to "insiders".
func (s *EnvironmentDeployManualSpec) GetTag() string {
	if s == nil {
		return "insiders"
	}
	return s.Tag
}

type EnvironmentDeployTypeSubscriptionSpec struct {
	// Tag is the tag to subscribe to.
	Tag string `yaml:"tag,omitempty"`
	// TODO: In the future, we may support subscribing by semver constraints.
}

// ResolveTag fetches the latest digest for the target imageRepo and configured
// subscription tag, and returns it.
func (s EnvironmentDeployTypeSubscriptionSpec) ResolveTag(imageRepo string) (string, error) {
	updater, err := imageupdater.New()
	if err != nil {
		return "", errors.Wrapf(err, "create image updater")
	}
	tagAndDigest, err := updater.ResolveTagAndDigest(imageRepo, s.Tag)
	if err != nil {
		return "", errors.Wrapf(err, "resolve digest for tag %q", "insiders")
	}
	return tagAndDigest, nil
}

type EnvironmentServiceSpec struct {
	// Domain configures where the resource is externally accessible. There
	// may be additional considerations based on your service's chosen protocol;
	// refer to the 'service.protocol' docstring for more details.
	//
	// Only supported for services of 'kind: service'.
	Domain *EnvironmentServiceDomainSpec `yaml:"domain,omitempty"`
	// HealthProbes configures both startup and continuous liveness probes.
	// If nil or explicitly disabled, no MSP-standard '/-/healthz' probes will
	// be configured.
	//
	// Only supported for services of 'kind: service'.
	HealthProbes *EnvironmentServiceHealthProbesSpec `yaml:"healthProbes,omitempty"`
	// Authentication configures access to the service. By default, the service
	// is publically available, and the service should handle any required
	// authentication by itself. Set this field to an empty value to not
	// configure any access to the service at all.
	//
	// More complex strategies, such as granting access to specific groups,
	// should add custom resources in the MSP IAM module defining
	// 'google_cloud_run_v2_service_iam_member' for 'roles/run.invoker' on
	// 'local.cloud_run_resource_name' and 'local.cloud_run_location' as 'name'
	// and 'location' respectively:
	// https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/cloud_run_v2_service_iam
	//
	// Only supported for services of 'kind: service'.
	Authentication *EnvironmentServiceAuthenticationSpec `yaml:"authentication,omitempty"`
}

func (s *EnvironmentServiceSpec) Validate() []error {
	if s == nil {
		return nil
	}
	var errs []error
	errs = append(errs, s.HealthProbes.Validate()...)
	return errs
}

type EnvironmentServiceDomainSpec struct {
	// Type is one of 'none' or 'cloudflare'. If empty, defaults to 'none'.
	Type       EnvironmentDomainType            `yaml:"type"`
	Cloudflare *EnvironmentDomainCloudflareSpec `yaml:"cloudflare,omitempty"`

	// Networking configures additional networking configuration.
	// Only applicable if a domain 'type' is configured.
	Networking *EnvironmentDomainNetworkingSpec `yaml:"networking,omitempty"`
}

// GetDNSName generates the DNS name for the environment. If nil or not configured,
// am empty string is returned.
func (s *EnvironmentServiceDomainSpec) GetDNSName() string {
	if s == nil {
		return ""
	}
	if s.Cloudflare != nil {
		return fmt.Sprintf("%s.%s", s.Cloudflare.Subdomain, s.Cloudflare.Zone)
	}
	return ""
}

type EnvironmentDomainType string

const (
	EnvironmentDomainTypeNone       = "none"
	EnvironmentDomainTypeCloudflare = "cloudflare"
)

type EnvironmentDomainCloudflareSpec struct {
	Subdomain string `yaml:"subdomain"`
	Zone      string `yaml:"zone"`

	// Proxied configures whether Cloudflare should proxy all traffic to get
	// WAF protection instead of only DNS resolution.
	//
	// Default: true
	Proxied *bool `yaml:"proxied,omitempty"`
}

// ShouldProxy evaluates whether Cloudflare WAF proxying should be used.
func (e *EnvironmentDomainCloudflareSpec) ShouldProxy() bool {
	if e == nil {
		return false
	}
	return pointers.Deref(e.Proxied, true)
}

type EnvironmentDomainNetworkingSpec struct {
	// LoadBalancerLogging enables logs on load balancers:
	// https://cloud.google.com/load-balancing/docs/https/https-logging-monitoring#viewing_logs
	//
	// Defaults to false. When enabled, no sampling is configured.
	LoadBalancerLogging *bool `yaml:"loadBalancerLogging,omitempty"`
}

type EnvironmentInstancesSpec struct {
	// Resources specifies the resources available to each service instance.
	Resources EnvironmentInstancesResourcesSpec `yaml:"resources"`
	// Scaling specifies the scaling behavior of the service.
	//
	// Currently only used for services of 'kind: service'.
	Scaling *EnvironmentInstancesScalingSpec `yaml:"scaling,omitempty"`
}

func (s EnvironmentInstancesSpec) Validate() []error {
	var errs []error
	errs = append(errs, s.Resources.Validate()...)
	return errs
}

type EnvironmentInstancesResourcesSpec struct {
	// CPU specifies the CPU available to each instance. Must be a value
	// bewteen 1 to 8.
	CPU int `yaml:"cpu"`
	// Memory specifies the memory available to each instance. Must be between
	// 512MiB and 32GiB.
	Memory string `yaml:"memory"`
	// CloudRunGeneration is either 1 or 2, corresponding to the generations
	// outlined in https://cloud.google.com/run/docs/about-execution-environments.
	// By default, we use the Cloud Run default.
	CloudRunGeneration *int `yaml:"cloudRunGeneration,omitempty"`
}

func (s *EnvironmentInstancesResourcesSpec) Validate() []error {
	if s == nil {
		return nil
	}

	var errs []error

	// https://cloud.google.com/run/docs/configuring/services/cpu
	if s.CPU < 1 {
		errs = append(errs, errors.New("resources.cpu must be >= 1"))
	} else if s.CPU > 8 {
		errs = append(errs,
			errors.New("resources.cpu > 8 not supported - consider decreasing scaling.maxRequestConcurrency and increasing scaling.maxCount instead"))
	}

	// https://cloud.google.com/run/docs/configuring/services/memory-limits
	// NOTE: Cloud Run documentation uses 'MiB' as the unit but the configuration
	// only accepts 'Mi', 'Gi', etc. Make sure our errors are in terms of the
	// format the configuration accepts to avoid confusion.
	bytes, err := units.ParseUnit(s.Memory, units.MakeUnitMap("i", "B", 1024))
	if err != nil {
		errs = append(errs, errors.Wrap(err, "resources.memory is invalid"))

		// Exit early - all following checks rely on knowing the memory that
		// was configured, so there's not point continuing validation if we
		// couldn't parse the memory field.
		return errs
	}
	if units.Base2Bytes(bytes)/units.MiB < 512 {
		errs = append(errs, errors.New("resources.memory must be >= 512Mi"))
	}
	gib := units.Base2Bytes(bytes) / units.GiB
	if gib > 32 {
		errs = append(errs,
			errors.New("resources.memory > 32Gi not supported - consider decreasing scaling.maxRequestConcurrency and increasing scaling.maxCount instead"))
	}

	// Enforce min CPUs: https://cloud.google.com/run/docs/configuring/services/memory-limits#cpu-minimum
	if gib > 24 && s.CPU < 8 {
		errs = append(errs, errors.New("resources.memory > 24Gi requires resources.cpu >= 8"))
	} else if gib > 16 && s.CPU < 6 {
		errs = append(errs, errors.New("resources.memory > 16Gi requires resources.cpu >= 6"))
	} else if gib > 8 && s.CPU < 4 {
		errs = append(errs, errors.New("resources.memory > 8Gi requires resources.cpu >= 4"))
	} else if gib > 4 && s.CPU < 2 {
		errs = append(errs, errors.New("resources.memory > 4Gi requires resources.cpu >= 2"))
	}

	// Enforce min memory: https://cloud.google.com/run/docs/configuring/services/cpu#cpu-memory
	if s.CPU > 6 && gib < 4 {
		errs = append(errs, errors.New("resources.cpu > 6 requires resources.memory >= 4Gi"))
	} else if s.CPU > 4 && gib < 2 {
		errs = append(errs, errors.New("resources.cpu > 4 requires resources.memory >= 2Gi"))
	}

	return errs
}

type EnvironmentInstancesScalingSpec struct {
	// MaxRequestConcurrency is the maximum number of concurrent requests that
	// each instance is allowed to serve. Before this concurrency is reached,
	// Cloud Run will begin scaling up additional instances, up to MaxCount.
	//
	// If not provided, the defualt is defaultMaxConcurrentRequests
	MaxRequestConcurrency *int `yaml:"maxRequestConcurrency,omitempty"`
	// MinCount is the minimum number of instances that will be running at all
	// times. Set this to >0 to avoid service warm-up delays.
	MinCount int `yaml:"minCount"`
	// MaxCount is the maximum number of instances that Cloud Run is allowed to
	// scale up to. When this value is >= the default of 5, then we also provision
	// an alert that fires when Cloud Run scaling approaches the max instance
	// count.
	//
	// If not provided, the default is 5.
	MaxCount *int `yaml:"maxCount,omitempty"`
}

// GetMaxCount returns nil if no scaling options are relevant, or the default,
// or the max value.
func (e *EnvironmentInstancesScalingSpec) GetMaxCount() *int {
	if e == nil {
		return nil
	}
	return pointers.Ptr(pointers.Deref(e.MaxCount, 5)) // builder.DefaultMaxInstances
}

type EnvironmentServiceAuthenticationSpec struct {
	// Sourcegraph enables access to everyone in the sourcegraph.com GSuite
	// domain.
	Sourcegraph *bool `yaml:"sourcegraph,omitempty"`
}

type EnvironmentServiceHealthProbesSpec struct {
	// HealthzProbes configures whether the MSP-standard '/-/healthz' service
	// probes should be disabled. When disabling, you should explicitly set
	// 'healthzProbes: false'.
	//
	// - When disabled, the default probe is a very generous one that waits 240s
	//   for your service to respond with anything at all on '/'. If your service
	//   is externally available, it MUST respond with status 200 on HTTP requests
	//   to '/'.
	// - When enabled, the MSP-standard '/-/healthz' diagnostic check is used
	//   with a generated diagnostics secret enforcing Timeout and Interval.
	//
	// We recommend disabling it when creating a service, and re-enabling it
	// once the service is confirmed to be deployed and healthy. Disabling the
	// probe on first startup prevents the first Terraform apply from failing if
	// your healthcheck is comprehensive, or if you haven't implemented
	// '/-/healthz' yet.
	HealthzProbes *bool `yaml:"healthzProbes,omitempty"`

	// Timeout configures the period of time after which a health probe times
	// out, in seconds.
	//
	// Defaults to 3 seconds.
	Timeout *int `yaml:"timeout,omitempty"`

	// StartupInterval configures the frequency, in seconds, at which to
	// probe the deployed service on startup. Must be greater than or equal to
	// timeout.
	//
	// Defaults to timeout.
	StartupInterval *int `yaml:"startupInterval,omitempty"`

	// StartupInterval configures the frequency, in seconds, at which to
	// probe the deployed service after startup to continuously check its health.
	// Must be greater than or equal to timeout.
	//
	// Defaults to timeout * 10.
	LivenessInterval *int `yaml:"livenessInterval,omitempty"`
}

func (s *EnvironmentServiceHealthProbesSpec) Validate() []error {
	if s == nil {
		return nil
	}
	var errs []error
	if !s.UseHealthzProbes() {
		if s.Timeout != nil || s.StartupInterval != nil || s.LivenessInterval != nil {
			errs = append(errs,
				errors.New("timeout, startupInterval and livenessInterval can only be configured when healthzProbes is enabled"))
		}

		// Nothing else to check
		return errs
	}

	if s.GetTimeoutSeconds() > s.GetStartupIntervalSeconds() {
		errs = append(errs, errors.New("startupInterval must be greater than or equal to timeout"))
	}
	if s.GetTimeoutSeconds() > s.GetLivenessIntervalSeconds() {
		errs = append(errs, errors.New("livenessInterval must be greater than or equal to timeout"))
	}
	if s.GetLivenessIntervalSeconds() > 3600 {
		errs = append(errs, errors.New("livenessInterval must be less than or equal to 3600 seconds"))
	}

	return errs
}

// UseHealthzProbes indicates whether the MSP-standard '/-/healthz' probes
// with diagnostics secrets should be used.
func (s *EnvironmentServiceHealthProbesSpec) UseHealthzProbes() bool {
	// No config == disabled
	if s == nil {
		return false
	}
	// If config is provided, must be explicitly disabled with 'enabled: false'
	return pointers.Deref(s.HealthzProbes, true)
}

// MaximumStartupLatencySeconds infers the overal maximum latency for a
// healthcheck to return healthy when the service is starting up.
func (s *EnvironmentServiceHealthProbesSpec) MaximumStartupLatencySeconds() int {
	if !s.UseHealthzProbes() {
		return 240 // maximum Cloud Run timeout
	}
	// Maximum startup latency is retries x interval.
	const maxRetries = 3
	return maxRetries * s.GetStartupIntervalSeconds()
}

// GetStartupIntervalSeconds returns the configured value, the default, or 0 if the spec is nil.
func (s *EnvironmentServiceHealthProbesSpec) GetStartupIntervalSeconds() int {
	if s == nil {
		return 0
	}
	return pointers.Deref(s.StartupInterval, s.GetTimeoutSeconds())
}

// GetLivenessIntervalSeconds returns the configured value, the default, or 0 if the spec is nil.
func (s *EnvironmentServiceHealthProbesSpec) GetLivenessIntervalSeconds() int {
	if s == nil {
		return 0
	}
	return pointers.Deref(s.LivenessInterval, s.GetTimeoutSeconds()*10) // 10x timeout default
}

// GetTimeoutSeconds returns the configured value, the default, or 0 if the spec is nil.
func (s *EnvironmentServiceHealthProbesSpec) GetTimeoutSeconds() int {
	if s == nil {
		return 0
	}
	return pointers.Deref(s.Timeout, 3)
}

type EnvironmentJobSpec struct {
	// DeadlineSeconds of each job execution, in seconds. Defaults to 300.
	DeadlineSeconds *int `yaml:"deadlineSeconds,omitempty"`
	// Schedule configures a cron schedule for the service.
	//
	// Only supported for services of 'kind: job'.
	Schedule *EnvironmentJobScheduleSpec `yaml:"schedule,omitempty"`
}

func (s *EnvironmentJobSpec) Validate() []error {
	if s == nil {
		return nil
	}

	var errs []error
	errs = append(errs, s.Schedule.Validate()...)
	return errs
}

type EnvironmentJobScheduleSpec struct {
	// Cron is a cron schedule in the form of "* * * * *".
	//
	// The smallest interval must be greater than 15 minutes, and more frequent
	// than once a week.
	//
	// Protip: use https://crontab.guru
	Cron string `yaml:"cron"`
}

func (s *EnvironmentJobScheduleSpec) Validate() []error {
	if s == nil {
		return nil
	}

	var errs []error
	if _, err := s.FindMaxCronInterval(time.Now()); err != nil {
		errs = append(errs, errors.Wrap(err, "schedule.cron: invalid schedule"))
	}
	return errs
}

// FindMaxCronInterval tries to find the largest gap between events in the cron
// schedule. It may return 'nil, nil' if no configuration is available.
func (s *EnvironmentJobScheduleSpec) FindMaxCronInterval(now time.Time) (*time.Duration, error) {
	if s == nil {
		return nil, nil
	}

	expr, err := cronexpr.Parse(s.Cron)
	if err != nil {
		return nil, errors.Wrap(err, "invalid cron schedule")
	}

	// Get nScheduled events to try and see what the largest gap is - this
	// is not performance sensitive, we just need to be able to reliably find
	// the largest interval. some silly crons won't generate reliable intervals
	// but this will hopefully give us a realistic indicator that we can error
	// out on below.
	nScheduled := 24 * 7 * 4 // up to every 4 times per hour (15 minutes) every day per week
	scheduled := expr.NextN(now, uint(nScheduled))

	// scheduled is in chronological order, so we can compare subsequent events
	// to find the largest gap in this cron.
	var maxGap time.Duration
	for i := 0; i < len(scheduled)-2; i += 1 {
		t1 := scheduled[i]
		t2 := scheduled[i+1]
		gap := t2.Sub(t1)
		if gap > maxGap {
			maxGap = gap
		}
	}

	// should not be possible to have <15m schedule for costs
	if maxGap < 15*time.Minute {
		return nil, errors.Newf("the longest interval must be >15m, got %s", maxGap.String())
	}

	// once we get into the longer-than-weekly territory, things might get funky;
	// forbid these very long intervals for now
	if maxGap > 8*24*time.Hour {
		return nil, errors.Newf("the longest interval must be <8 days, got %s", maxGap.String())
	}

	return &maxGap, nil
}

type EnvironmentSecretVolume struct {
	// MountPath is the path within the container where the secret will be
	// mounted. The mounted file is read-only.
	MountPath string `yaml:"mountPath"`
	// Secret is name of the secret in the service's project to populate in the
	// environment.
	//
	// To point to a secret in another project, use the format
	// 'projects/{project}/secrets/{secretName}' in the value. Access to the
	// target project will be automatically granted.
	Secret string `yaml:"secret"`
}

func (v EnvironmentSecretVolume) Validate() []error {
	var errs []error
	if v.MountPath == "" {
		errs = append(errs, errors.New("mountPath is required"))
	}
	if !filepath.IsAbs(v.MountPath) {
		errs = append(errs, errors.Newf("mountPath must be abs file path, got %q", v.MountPath))
	}
	if dir, file := filepath.Split(v.MountPath); dir == "" || file == "" {
		errs = append(errs, errors.Newf("mountPath may be malformed, got %q", v.MountPath))
	}
	if v.Secret == "" {
		errs = append(errs, errors.New("secret is required"))
	}
	return errs
}

type EnvironmentResourcesSpec struct {
	// Redis, if provided, provisions a Redis instance backed by Cloud Memorystore.
	// Details for using this Redis instance is automatically provided in
	// environment variables:
	//
	//  - REDIS_ENDPOINT
	//
	// Sourcegraph Redis libraries (i.e. internal/redispool) will automatically
	// use the given configuration.
	Redis *EnvironmentResourceRedisSpec `yaml:"redis,omitempty"`
	// PostgreSQL, if provided, provisions a PostgreSQL database instance backed
	// by Cloud SQL.
	//
	// To connect to the database, use
	// (lib/managedservicesplatform/service.Contract).GetPostgreSQLDB().
	PostgreSQL *EnvironmentResourcePostgreSQLSpec `yaml:"postgreSQL,omitempty"`
	// BigQueryDataset, if provided, provisions a dataset for the service to write
	// to. Details for writing to the dataset are automatically provided in
	// environment variables:
	//
	//  - BIGQUERY_PROJECT
	//  - BIGQUERY_DATASET
	//
	// Only one dataset can be provisioned using MSP per MSP service, but the
	// dataset may contain more than one table.
	BigQueryDataset *EnvironmentResourceBigQueryDatasetSpec `yaml:"bigQueryDataset,omitempty"`
}

// List collects a list of provisioned resources for human reference.
func (s *EnvironmentResourcesSpec) List() []string {
	if s == nil {
		return nil
	}

	var resources []string
	if s.Redis != nil {
		resources = append(resources, s.Redis.ResourceKind())
	}
	if s.PostgreSQL != nil {
		resources = append(resources, s.PostgreSQL.ResourceKind())
	}
	if s.BigQueryDataset != nil {
		resources = append(resources, s.BigQueryDataset.ResourceKind())
	}
	return resources
}

func (s *EnvironmentResourcesSpec) Validate() []error {
	if s == nil {
		return nil
	}
	var errs []error
	errs = append(errs, s.PostgreSQL.Validate()...)
	errs = append(errs, s.BigQueryDataset.Validate()...)
	return errs
}

type EnvironmentResourceRedisSpec struct {
	// MemoryGB defaults to 1.
	MemoryGB *int `yaml:"memoryGB,omitempty"`
	// HighAvailability is disabled by default. Enabling it toggles regional
	// replicas for the Redis instance without adding read replicas for ~double
	// the price. It should be enabled for our most critical services, but as
	// Redis is fairly affordable, if you run into Redis stability issues there
	// is no blocker to enabling this.
	//
	//  - https://cloud.google.com/memorystore/docs/redis/high-availability-for-memorystore-for-redis
	//  - https://cloud.google.com/memorystore/docs/redis/pricing#instance_pricing_with_no_read_replicas
	//
	// Also see: https://sourcegraph.notion.site/655e89d164b24727803f5e5a603226d8
	HighAvailability *bool `yaml:"highAvailability,omitempty"`
}

func (EnvironmentResourceRedisSpec) ResourceKind() string { return "Redis" }

type EnvironmentResourcePostgreSQLSpec struct {
	// Databases to provision - required.
	Databases []string `yaml:"databases"`
	// CPU defaults to 1. Must be 1, or an even number between 2 and 96.
	CPU *int `yaml:"cpu,omitempty"`
	// MemoryGB defaults to 4 (to meet CloudSQL minimum). You must request 0.9
	// to 6.5 GB per vCPU.
	MemoryGB *int `yaml:"memoryGB,omitempty"`
	// MaxConnections defaults to whatever CloudSQL provides. Must be between
	// 14 and 262143.
	MaxConnections *int `yaml:"maxConnections,omitempty"`
	// HighAvailability is disabled by default. Enabling it provisions Cloud SQL
	// HA configuration for ~double the price and additional point-in-time-recovery
	// backup expenses, and should only be enabled for our most critical services.
	//
	//  - https://cloud.google.com/sql/docs/postgres/high-availability
	//  - https://cloud.google.com/sql/pricing
	//
	// Also see: https://sourcegraph.notion.site/655e89d164b24727803f5e5a603226d8
	//
	// Toggling highAvailability will incur a small amount of downtime.
	HighAvailability *bool `yaml:"highAvailability,omitempty"`
	// LogicalReplication configures native logical replication for PostgreSQL:
	// https://www.postgresql.org/docs/current/logical-replication.html
	//
	// Enabling logicalReplication will incur a small amount of downtime. If you
	// plan to use logical replication, you should configure an empty
	// 'logicalReplication' block to initialize the database instance with the
	// prerequisite configuration:
	//
	//  logicalReplication: {}
	//
	// The primary use case for logicalReplication is to integrate with GCP
	// Datastream to make tables available in BigQuery:
	// https://cloud.google.com/datastream/docs/sources-postgresql
	LogicalReplication *EnvironmentResourcePostgreSQLLogicalReplicationSpec `yaml:"logicalReplication,omitempty"`
}

func (EnvironmentResourcePostgreSQLSpec) ResourceKind() string { return "PostgreSQL instance" }

func (s *EnvironmentResourcePostgreSQLSpec) Validate() []error {
	if s == nil {
		return nil
	}
	var errs []error
	if s.CPU != nil {
		if *s.CPU < 1 {
			errs = append(errs, errors.New("postgreSQL.cpu must be >= 1"))
		}
		if *s.CPU > 1 && *s.CPU%2 != 0 {
			errs = append(errs, errors.New("postgreSQL.cpu must be 1 or a multiple of 2"))
		}
		if *s.CPU > 96 {
			errs = append(errs, errors.New("postgreSQL.cpu must be <= 96"))
		}
	}
	if s.MemoryGB != nil {
		cpu := pointers.Deref(s.CPU, 1)
		if *s.MemoryGB < 4 {
			errs = append(errs, errors.New("postgreSQL.memoryGB must be >= 4"))
		}
		if *s.MemoryGB < cpu {
			errs = append(errs, errors.New("postgreSQL.memoryGB must be >= postgreSQL.cpu"))
		}
		if *s.MemoryGB > 6*cpu {
			errs = append(errs, errors.New("postgreSQL.memoryGB must be <= 6*postgreSQL.cpu"))
		}
	}
	if s.LogicalReplication != nil {
		errs = append(errs, s.LogicalReplication.Validate()...)
	}
	return errs
}

type EnvironmentResourcePostgreSQLLogicalReplicationSpec struct {
	// Publications configure PostgreSQL logical replication publications for
	// consumption in tools like GCP Datastream.
	Publications []EnvironmentResourcePostgreSQLLogicalReplicationPublicationsSpec `yaml:"publications,omitempty"`
}

func (s *EnvironmentResourcePostgreSQLLogicalReplicationSpec) Validate() []error {
	if s == nil {
		return nil
	}

	var errs []error
	seenPublications := map[string]struct{}{}
	for i, p := range s.Publications {
		if p.Name == "" {
			errs = append(errs, errors.Newf("publication[%d].name is required", i))
		}
		if _, ok := seenPublications[p.Name]; ok {
			errs = append(errs, errors.Newf("publication[%d].name must be unique", i))
		}
		seenPublications[p.Name] = struct{}{}

		if p.Database == "" {
			errs = append(errs, errors.Newf("publication[%d].database is required", i))
		}
		if len(p.Tables) == 0 {
			errs = append(errs, errors.Newf("publication[%d].tables is required", i))
		}
		for ti, t := range p.Tables {
			if t == "" {
				errs = append(errs, errors.Newf("publication[%d].tables[%d] must not be empty", i, ti))
			}
		}
	}
	return errs
}

type EnvironmentResourcePostgreSQLLogicalReplicationPublicationsSpec struct {
	// Name of the publication. Must be machine-friendly and unique. Required.
	Name string `yaml:"name"`
	// Database containing the tables you want to replicate and publish. Required.
	Database string `yaml:"database"`
	// Tables to replicate and publish. Required.
	//
	// Note that curerntly, referenced tables MUST exist BEFORE a publication
	// is provisioned on them. Database tables should be created and owned by
	// the application workload identity.
	Tables []string `yaml:"tables"`
}

type EnvironmentResourceBigQueryDatasetSpec struct {
	// Tables are the IDs of tables to create within the service's BigQuery
	// dataset. Required.
	//
	// For EACH table, a BigQuery JSON schema MUST be provided alongside the
	// service specification file, in `${tableID}.bigquerytable.json`. Learn
	// more about BigQuery table schemas here:
	// https://cloud.google.com/bigquery/docs/schemas#specifying_a_json_schema_file
	Tables []string `yaml:"tables"`
	// rawSchemaFiles are the `${tableID}.bigquerytable.json` files adjacent
	// to the service specification.
	// Loaded by (EnvironmentResourceBigQueryTableSpec).LoadSchemas().
	rawSchemaFiles map[string][]byte

	// DatasetID, if provided, configures a custom dataset ID to place all tables
	// into. By default, we use the service ID as the dataset ID (with underscores
	// replacing illegal characters).
	//
	// Dataset IDs must be alphanumeric (plus underscores).
	DatasetID *string `yaml:"datasetID,omitempty"`
	// ProjectID can be used to specify a separate project ID from the service's
	// project for BigQuery resources. If not provided, resources are created
	// within the service's project.
	ProjectID *string `yaml:"projectID,omitempty"`
}

func (EnvironmentResourceBigQueryDatasetSpec) ResourceKind() string { return "BigQuery dataset" }

func (s *EnvironmentResourceBigQueryDatasetSpec) GetDatasetID(serviceID string) string {
	return pointers.Deref(s.DatasetID,
		// Dataset IDs must be alphanumeric (plus underscores)
		strings.ReplaceAll(serviceID, "-", "_"))
}

func (s *EnvironmentResourceBigQueryDatasetSpec) Validate() []error {
	if s == nil {
		return nil
	}
	var errs []error
	if len(s.Tables) == 0 {
		errs = append(errs, errors.New("bigQueryDataset.tables must be non-empty"))
	}
	return errs
}

// LoadSchemas populates rawSchemaFiles by convention, looking for
// `bigquery.${tableID}.schema.json` files in dir.
func (s *EnvironmentResourceBigQueryDatasetSpec) LoadSchemas(dir string) error {
	s.rawSchemaFiles = make(map[string][]byte, len(s.Tables))

	// Make sure all tables have a schema file
	for _, table := range s.Tables {
		// Open by convention
		schema, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("%s.bigquerytable.json", table)))
		if err != nil {
			return errors.Wrapf(err, "read schema for BigQuery table %s", table)
		}

		// Parse and marshal for consistent formatting. Note that the table
		// must be a JSON array.
		var schemaData []any
		if err := json.Unmarshal(schema, &schemaData); err != nil {
			return errors.Wrapf(err, "parse schema for BigQuery table %s", table)
		}
		s.rawSchemaFiles[table], err = json.MarshalIndent(schemaData, "", "  ")
		if err != nil {
			return errors.Wrapf(err, "marshal schema for BigQuery table %s", table)
		}
	}

	return nil
}

// GetSchema returns the schema for the given tableID as loaded by LoadSchemas().
// LoadSchemas will ensure that each table has a corresponding schema file.
func (s *EnvironmentResourceBigQueryDatasetSpec) GetSchema(tableID string) []byte {
	return s.rawSchemaFiles[tableID]
}

type EnvironmentAlertingSpec struct {
	// Opsgenie, if true, disables suppression of Opsgenie alerts. Note that
	// only critical alerts are delivered to Opsgenie - this is a curated set
	// of alerts that are considered high-signal indicators that something is
	// definitely wrong with your service.
	Opsgenie *bool `yaml:"opsgenie,omitempty"`
}

type EnvironmentLocationsSpec struct {
	// GCPRegion is the GCP region where all regional resources resources should
	// be deployed for this environment, for example:
	//
	// - https://cloud.google.com/about/locations#americas
	// - https://cloud.google.com/about/locations#asia-pacific
	GCPRegion string `yaml:"gcpRegion"`
	// GCPLocation is the GCP location where all multi-regional resources should
	// be deployed for this environment, e.g. BigQuery:
	// https://cloud.google.com/about/locations#multi-region
	//
	// Note that the names of valid locations are not consistent across GCP
	// products, callsites should check that the allowed values in
	// 'EnvironmentLocationsSpec.Validate()' match the actual values supported
	// by the relevant GCP product.
	GCPLocation string `yaml:"gcpLocation"`
}

// GetLocationSpec returns the appropriate location spec for the environment,
// returning defaults if none is configured.
func (s EnvironmentSpec) GetLocationSpec() EnvironmentLocationsSpec {
	if s.Locations == nil {
		// Provide our defaults
		return EnvironmentLocationsSpec{
			GCPRegion:   "us-central1",
			GCPLocation: "US", // original default for BigQuery
		}
	}
	return *s.Locations
}

func (s EnvironmentLocationsSpec) Validate() []error {
	var (
		allowedRegions = []string{
			// Starter list, same as Cloud controller
			"us-central1",
			"us-west1",
			"asia-northeast1",
			"australia-southeast1",
			"europe-west2",
			"europe-west3",
			"northamerica-northeast1",
		}
		allowedLocations = []string{
			// Only support locations that have BigQuery (i.e. not APAC)
			"us",
			"europe",
		}
	)

	var errs []error

	if s.GCPRegion == "" {
		errs = append(errs, errors.New("locations.gcpRegion must be non-empty"))
	} else if !slices.Contains(allowedRegions, s.GCPRegion) {
		errs = append(errs, errors.Newf("locations.gcpRegion %q is not valid, allowed: [%s]",
			s.GCPRegion, strings.Join(allowedRegions, ", ")))
	}

	if s.GCPLocation == "" {
		errs = append(errs, errors.New("locations.gcpLocation must be non-empty"))
	} else if !slices.Contains(allowedLocations, strings.ToLower(s.GCPLocation)) {
		errs = append(errs, errors.Newf("locations.gcpLocation %q is not valid, allowed: [%s]",
			s.GCPLocation, strings.Join(allowedLocations, ", ")))
	}

	return errs
}
