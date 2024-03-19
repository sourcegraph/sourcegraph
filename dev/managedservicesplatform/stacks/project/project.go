package project

import (
	"strings"

	"github.com/aws/jsii-runtime-go"
	"github.com/grafana/regexp"
	"github.com/hashicorp/terraform-cdk-go/cdktf"

	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/project"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/projectservice"

	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resourceid"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack/options/googleprovider"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/spec"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

var gcpServices = []string{
	"run.googleapis.com",
	"containerregistry.googleapis.com",
	"cloudbuild.googleapis.com",
	"logging.googleapis.com",
	"monitoring.googleapis.com",
	"iam.googleapis.com",
	"secretmanager.googleapis.com",
	"redis.googleapis.com",
	"compute.googleapis.com",
	"networkmanagement.googleapis.com",
	"vpcaccess.googleapis.com",
	"servicenetworking.googleapis.com",
	"storage-api.googleapis.com",
	"storage-component.googleapis.com",
	"bigquery.googleapis.com",
	"cloudprofiler.googleapis.com",
	"cloudscheduler.googleapis.com",
	"sqladmin.googleapis.com",
	"clouddeploy.googleapis.com",
	"bigqueryconnection.googleapis.com",
}

const (
	BillingAccountID = "017005-C370B2-0E3030"
	// DefaultProjectFolderID points to the 'Managed Services' folder:
	// https://console.cloud.google.com/welcome?folder=26336759932
	DefaultProjectFolderID = "folders/26336759932"
)

var EnvironmentCategoryFolders = map[spec.EnvironmentCategory]string{
	// Engineering Projects - https://console.cloud.google.com/welcome?folder=795981974432
	spec.EnvironmentCategoryTest: "folders/795981974432",

	// Internal Projects - https://console.cloud.google.com/welcome?folder=387815626940
	spec.EnvironmentCategoryInternal: "folders/387815626940",

	// Use default folder - see DefaultProjectFolderID
	spec.EnvironmentCategoryExternal: DefaultProjectFolderID,
	spec.EnvironmentCategory(""):     DefaultProjectFolderID,
}

type CrossStackOutput struct {
	// Project is created with a generated project ID.
	Project project.Project
}

type Variables struct {
	// ProjectIDPrefix is the generated project ID. A suffix of the format
	// '-${randomizedSuffix}' should be added, as project IDs must be unique.
	ProjectID string

	// DisplayName is a display name for the project. It does not need to be unique.
	DisplayName string

	// Labels to apply to the project.
	Labels map[string]string

	// Category determines what folder the project will be created in.
	Category spec.EnvironmentCategory

	// EnableAuditLogs ships GCP audit logs to security cluster.
	// TODO: Not yet implemented
	EnableAuditLogs bool

	// Services is a list of additional GCP services to enable.
	Services []string

	// PreventDestroys indicates if destroys should be allowed on core components of
	// this resource.
	PreventDestroys bool
}

const StackName = "project"

// NewStack creates a stack that provisions a GCP project.
func NewStack(stacks *stack.Set, vars Variables) (*CrossStackOutput, error) {
	stack, locals, err := stacks.New(StackName,
		// The project we want might not exist yet, so omit it when initializing
		// the provider.
		googleprovider.With(""))
	if err != nil {
		return nil, err
	}

	// Name all stack resources after the desired project ID.
	// HACK: For consistency with what used to be here, we extract the "prefix"
	// (everything before the last component, '-${randomsuffix}') and use that
	// as the root resourceid.
	parts := strings.Split(vars.ProjectID, "-")
	prefix := parts[:len(parts)-1]
	id := resourceid.New(strings.Join(prefix, "-"))

	project := project.NewProject(stack,
		id.TerraformID("project"),
		&project.ProjectConfig{
			Name:              pointers.Ptr(vars.DisplayName),
			ProjectId:         &vars.ProjectID,
			AutoCreateNetwork: false,
			BillingAccount:    pointers.Ptr(BillingAccountID),
			FolderId: func() *string {
				folder, ok := EnvironmentCategoryFolders[vars.Category]
				if ok {
					return &folder
				}
				return pointers.Ptr(string(DefaultProjectFolderID))
			}(),
			Labels: func(input map[string]string) *map[string]*string {
				labels := make(map[string]*string)
				for k, v := range input {
					labels[sanitizeName(k)] = pointers.Ptr(v)
				}

				// For security purposes enable audit logging for production projects (external and internal)
				if vars.Category.IsProduction() {
					labels["collect-security-audit-logs"] = pointers.Ptr("true")
				}

				return &labels
			}(vars.Labels),

			// Critical resource that cannot be replaced.
			Lifecycle: &cdktf.TerraformResourceLifecycle{
				PreventDestroy: &vars.PreventDestroys,
			},
		})

	for _, service := range append(gcpServices, vars.Services...) {
		projectservice.NewProjectService(stack,
			id.TerraformID("project-service-%s", strings.ReplaceAll(service, ".", "-")),
			&projectservice.ProjectServiceConfig{
				Project:                  project.ProjectId(),
				Service:                  pointers.Ptr(service),
				DisableDependentServices: jsii.Bool(false),
				// prevent accidental deletion of services
				DisableOnDestroy: jsii.Bool(false),
			})
	}

	// Collect outputs
	locals.Add("project_id", *project.ProjectId(), "Generated project ID")
	return &CrossStackOutput{Project: project}, nil
}

var regexpMatchNonLowerAlphaNumericUnderscoreDash = regexp.MustCompile(`[^a-z0-9_-]`)

// sanitizeName ensures the name contains only lowercase letters, numerals, underscores, and dashes.
// non-compliant characters are replaced with underscores.
func sanitizeName(name string) string {
	s := strings.ToLower(name)
	s = regexpMatchNonLowerAlphaNumericUnderscoreDash.ReplaceAllString(s, "_")
	return s
}
