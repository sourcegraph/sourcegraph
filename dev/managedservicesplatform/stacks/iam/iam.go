package iam

import (
	"fmt"
	"sort"
	"strings"

	"github.com/grafana/regexp"
	"golang.org/x/exp/maps"

	"github.com/aws/jsii-runtime-go"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/projectiamcustomrole"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/projectiammember"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/secretmanagersecretiammember"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/serviceaccountiammember"
	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google_beta/googleprojectserviceidentity"
	google_beta "github.com/sourcegraph/managed-services-platform-cdktf/gen/google_beta/provider"

	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/random"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resource/serviceaccount"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resourceid"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack/options/googleprovider"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack/options/randomprovider"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/spec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

type CrossStackOutput struct {
	CloudRunWorkloadServiceAccount *serviceaccount.Output
	OperatorAccessServiceAccount   *serviceaccount.Output
	// CloudDeployExecutionServiceAccount is only provisioned if
	// IsFinalStageOfRollout is true for this environment.
	CloudDeployExecutionServiceAccount *serviceaccount.Output
	CloudDeployReleaserServiceAccount  *serviceaccount.Output
}

type Variables struct {
	ProjectID string
	Image     string
	Service   spec.ServiceSpec

	// SecretEnv should be the environment config that sources from secrets.
	SecretEnv map[string]string
	// SecretVolumes should be the environment config that mounts volumes from secrets.
	SecretVolumes map[string]spec.EnvironmentSecretVolume

	// IsFinalStageOfRollout should be true if BuildRolloutPipelineConfiguration
	// provides a non-nil configuration for an environment.
	IsFinalStageOfRollout bool

	// PreventDestroys indicates if destroys should be allowed on core components of
	// this resource.
	PreventDestroys bool
}

const StackName = "iam"

const (
	OutputCloudRunServiceAccount = "cloud_run_service_account"
	OutputOperatorServiceAccount = "operator_access_service_account"

	OutputCloudDeployReleaserServiceAccountID = "cloud_deploy_releaser_service_account_id"

	// tfcRobotMember is the service account used as the identity for our
	// Terraform Cloud runners.
	tfcRobotMember = "serviceAccount:terraform-cloud@sourcegraph-ci.iam.gserviceaccount.com"
)

func NewStack(stacks *stack.Set, vars Variables) (*CrossStackOutput, error) {
	stack, locals, err := stacks.New(StackName,
		googleprovider.With(vars.ProjectID),
		randomprovider.With())
	if err != nil {
		return nil, err
	}

	id := resourceid.New("iam")

	// serviceAccountRoles are granted to the service account for the Cloud Run service.
	serviceAccountRoles := []serviceaccount.Role{
		// Allow env vars to source from secrets
		{ID: resourceid.New("role_secret_accessor"), Role: "roles/secretmanager.secretAccessor"},
		// Allow service to access private networks
		{ID: resourceid.New("role_compute_networkuser"), Role: "roles/compute.networkUser"},
		// Allow service to emit observability
		{ID: resourceid.New("role_cloudtrace_agent"), Role: "roles/cloudtrace.agent"},
		{ID: resourceid.New("role_monitoring_metricwriter"), Role: "roles/monitoring.metricWriter"},
		// Allow service to publish Cloud Profiler profiles
		{ID: resourceid.New("role_cloudprofiler_agent"), Role: "roles/cloudprofiler.agent"},
		// Allow service to connect to Cloud SQL
		{ID: resourceid.New("role_cloudsql_client"), Role: "roles/cloudsql.client"},
		{ID: resourceid.New("role_cloudsql_instanceuser"), Role: "roles/cloudsql.instanceUser"},
	}

	// Grant configured roles to the workload identity
	if vars.Service.IAM != nil && len(vars.Service.IAM.Roles) > 0 {
		var rs []serviceaccount.Role
		for _, r := range vars.Service.IAM.Roles {
			rs = append(rs, serviceaccount.Role{
				ID:   resourceid.New(matchNonAlphaNumericRegex.ReplaceAllString(r, "_")),
				Role: r,
			})
		}
		serviceAccountRoles = append(rs, serviceAccountRoles...)
	}

	// Configure a role with custom permissions and grant it to the workload
	// identity
	if vars.Service.IAM != nil && len(vars.Service.IAM.Permissions) > 0 {
		customRole := projectiamcustomrole.NewProjectIamCustomRole(stack,
			id.TerraformID("custom-role"),
			&projectiamcustomrole.ProjectIamCustomRoleConfig{
				RoleId:      pointers.Ptr("msp_workload_custom_role"),
				Title:       pointers.Ptr("Managed Services Platform workload custom role"),
				Project:     &vars.ProjectID,
				Permissions: jsii.Strings(vars.Service.IAM.Permissions...),
			})
		serviceAccountRoles = append(serviceAccountRoles, serviceaccount.Role{
			ID:   resourceid.New("msp_workload_custom_role"),
			Role: *customRole.Name(),
		})
	}

	// Create a service account for the workload identity in Cloud Run
	workloadServiceAccount := serviceaccount.New(stack,
		id.Group("workload"),
		serviceaccount.Config{
			ProjectID: vars.ProjectID,
			// These might be used to grant access across other projects so
			// a human-usable ID and name are preferred over random values.
			AccountID:   fmt.Sprintf("%s-sa", vars.Service.ID),
			DisplayName: fmt.Sprintf("%s Service Account", vars.Service.GetName()),
			Roles:       serviceAccountRoles,

			// There may be external references to the service account to allow
			// the workload access to external resources, so guard it from deletes.
			PreventDestroys: vars.PreventDestroys,
		})

	// Let the TFC robot impersonate the workload service account to provision
	// things on its behalf if needed.
	{
		id := id.Group("tfc_impersonate_workload")
		workloadSAID := pointers.Stringf("projects/%s/serviceAccounts/%s",
			vars.ProjectID, workloadServiceAccount.Email)
		_ = serviceaccountiammember.NewServiceAccountIamMember(stack,
			id.TerraformID("serviceaccountuser"),
			&serviceaccountiammember.ServiceAccountIamMemberConfig{
				ServiceAccountId: workloadSAID,
				Role:             pointers.Ptr("roles/iam.serviceAccountUser"),
				Member:           pointers.Ptr(tfcRobotMember),
			})
		_ = serviceaccountiammember.NewServiceAccountIamMember(stack,
			id.TerraformID("serviceaccounttokencreator"),
			&serviceaccountiammember.ServiceAccountIamMemberConfig{
				ServiceAccountId: workloadSAID,
				Role:             pointers.Ptr("roles/iam.serviceAccountTokenCreator"),
				Member:           pointers.Ptr(tfcRobotMember),
			})
	}

	// Create a service account for operators to impersonate to access other
	// provisioned MSP resources. We use a randomized ID for more predictable
	// ID lengths and to indicate this is only used by human operators for MSP
	// tooling.
	operatorAccessAccountID := random.New(stack, id.Group("operatoraccess_account_id"), random.Config{
		Prefix:     "operatoraccess", // 15 charcters with '-'
		ByteLength: 3,                // 6 chars
	})
	operatorAccessServiceAccount := serviceaccount.New(stack,
		id.Group("operatoraccess"),
		serviceaccount.Config{
			ProjectID:   vars.ProjectID,
			AccountID:   operatorAccessAccountID.HexValue,
			DisplayName: fmt.Sprintf("%s Operator Access Service Account", vars.Service.GetName()),
			Roles: []serviceaccount.Role{
				// Roles for connecting to Cloud SQL
				{
					ID:   resourceid.New("role_cloudsql_client"),
					Role: "roles/cloudsql.client",
				}, {
					ID:   resourceid.New("role_cloudsql_instanceuser"),
					Role: "roles/cloudsql.instanceUser",
				},
				// Add roles here for operator READONLY access. Write access
				// should be granted by asking operators to impersonate the
				// workload service account.
			},
		},
	)

	googleBeta := google_beta.NewGoogleBetaProvider(stack, pointers.Ptr("google_beta"), &google_beta.GoogleBetaProviderConfig{
		Project: &vars.ProjectID,
	})

	// Provision the default Cloud Run robot account so that we can grant it
	// access to prerequisite resources.
	cloudRunIdentity := googleprojectserviceidentity.NewGoogleProjectServiceIdentity(stack,
		id.TerraformID("cloudrun-identity"),
		&googleprojectserviceidentity.GoogleProjectServiceIdentityConfig{
			Project: &vars.ProjectID,
			Service: pointers.Ptr("run.googleapis.com"),
			// Only available via beta provider:
			// https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/project_service_identity
			Provider: googleBeta,
		})
	identityMember := pointers.Ptr(fmt.Sprintf("serviceAccount:%s", *cloudRunIdentity.Email()))

	// If the service image is published to a private image repository, grant
	// the Cloud Run robot account access to the target project to pull the
	// image.
	if imageProject := extractImageGoogleProject(vars.Image); imageProject != nil {
		imageAccessID := id.Group("image_access")
		for _, r := range []serviceaccount.Role{{
			ID:   resourceid.New("object_viewer"),
			Role: "roles/storage.objectViewer", // for gcr.io
		}, {
			ID:   resourceid.New("artifact_reader"),
			Role: "roles/artifactregistry.reader", // for artifact registry
		}} {
			projectiammember.NewProjectIamMember(stack,
				imageAccessID.Append(r.ID).TerraformID("member"),
				&projectiammember.ProjectIamMemberConfig{
					Project: imageProject,
					Role:    pointers.Ptr(r.Role),
					Member:  identityMember,
				})
		}
	}

	// If any secret env secrets are external to this project, grant access to
	// the referenced secrets.
	if externalSecrets, err := extractExternalSecrets(vars.SecretEnv, vars.SecretVolumes); err != nil {
		return nil, errors.Wrap(err, "extracting secret projects")
	} else {
		secretAccessID := id.Group("external_secret_access")
		for _, p := range externalSecrets {
			secretmanagersecretiammember.NewSecretManagerSecretIamMember(stack,
				secretAccessID.Group(p.key).TerraformID("member"),
				&secretmanagersecretiammember.SecretManagerSecretIamMemberConfig{
					Project:  &p.projectID,
					SecretId: &p.secretID,
					Role:     pointers.Ptr("roles/secretmanager.secretAccessor"),
					Member:   &workloadServiceAccount.Member,
				})
		}
	}

	// Only referenced if vars.IsFinalStageOfRollout is true anyway, so safe
	// to leave as nil.
	var cloudDeployExecutorServiceAccount *serviceaccount.Output
	var cloudDeployReleaserServiceAccount *serviceaccount.Output
	if vars.IsFinalStageOfRollout {
		cloudDeployExecutorServiceAccount = serviceaccount.New(stack,
			id.Group("clouddeploy-executor"),
			serviceaccount.Config{
				ProjectID:   vars.ProjectID,
				AccountID:   "clouddeploy-executor",
				DisplayName: fmt.Sprintf("%s Cloud Deploy Executor Service Account", vars.Service.GetName()),
				Roles: []serviceaccount.Role{
					{
						ID:   resourceid.New("role_clouddeploy_job_runner"),
						Role: "roles/clouddeploy.jobRunner",
					},
				},
			},
		)

		cloudDeployReleaserServiceAccount = serviceaccount.New(stack,
			id.Group("clouddeploy-releaser"),
			serviceaccount.Config{
				ProjectID:   vars.ProjectID,
				AccountID:   "clouddeploy-releaser",
				DisplayName: fmt.Sprintf("%s Cloud Deploy Releases Service Account", vars.Service.GetName()),
				// Roles are configured in the cloudrun stack to scope access to required resources
				Roles: nil,
			},
		)

		// For use in e.g. https://sourcegraph.sourcegraph.com/github.com/sourcegraph/infrastructure/-/blob/managed-services/continuous-deployment-pipeline/main.tf?L5-20
		// For now, just provide the ID and ask users to configure the GH action
		// workload identity pool elsewhere. This can be referenced directly from
		// GSM of the environment secrets.
		locals.Add(OutputCloudDeployReleaserServiceAccountID, cloudDeployReleaserServiceAccount.Email,
			"Service Account ID for Cloud Deploy release creation - intended for workload identity federation in CI")
	}

	// Collect outputs
	locals.Add(OutputCloudRunServiceAccount, workloadServiceAccount.Email,
		"Service Account email used as Cloud Run resource workload identity")
	locals.Add(OutputOperatorServiceAccount, operatorAccessServiceAccount.Email,
		"Service Account email used for operator access to other resources")

	return &CrossStackOutput{
		CloudRunWorkloadServiceAccount:     workloadServiceAccount,
		OperatorAccessServiceAccount:       operatorAccessServiceAccount,
		CloudDeployExecutionServiceAccount: cloudDeployExecutorServiceAccount,
		CloudDeployReleaserServiceAccount:  cloudDeployReleaserServiceAccount,
	}, nil
}

var matchNonAlphaNumericRegex = regexp.MustCompile("[^a-zA-Z0-9]+")

func extractImageGoogleProject(image string) *string {
	// Private images in our GCP projects have patterns like:
	// - us-central1-docker.pkg.dev/control-plane-5e9ee072/docker/apiserver
	// - us.gcr.io/sourcegraph-dev/abuse-banbot
	// If the root matches particular patterns, the second component is the
	// project ID.
	imageRepoParts := strings.SplitN(image, "/", 3)
	if len(imageRepoParts) != 3 {
		return nil
	}
	repoRoot := imageRepoParts[0]
	if strings.HasSuffix(repoRoot, ".gcr.io") ||
		strings.HasSuffix(repoRoot, "-docker.pkg.dev") {
		return &imageRepoParts[1]
	}
	return nil
}

type externalSecret struct {
	key string // original configuration key, lowercased

	projectID string
	secretID  string
}

func extractExternalSecrets(secretEnv map[string]string, secretVolumes map[string]spec.EnvironmentSecretVolume) ([]externalSecret, error) {
	var externalSecrets []externalSecret

	// Add secretEnv
	secretKeys := maps.Keys(secretEnv)
	sort.Strings(secretKeys)
	for _, k := range secretKeys {
		secretName := secretEnv[k]
		es, err := getExternalSecretFromSecretName(k, secretName)
		if err != nil {
			return nil, err
		}
		if es != nil {
			externalSecrets = append(externalSecrets, *es)
		}
	}

	// Add secretVolumes
	secretKeys = maps.Keys(secretVolumes)
	sort.Strings(secretKeys)
	for _, k := range secretKeys {
		secretName := secretVolumes[k].Secret
		es, err := getExternalSecretFromSecretName(fmt.Sprintf("volume_%s", k), secretName)
		if err != nil {
			return nil, err
		}
		if es != nil {
			externalSecrets = append(externalSecrets, *es)
		}
	}

	return externalSecrets, nil
}

func getExternalSecretFromSecretName(key, secretName string) (*externalSecret, error) {
	// Error on easy-to-make oopsies
	if strings.HasPrefix(secretName, "project/") {
		return nil, errors.Newf("invalid secret name %q: 'project/'-prefixed name provided, did you mean 'projects/'?",
			secretName)
	}
	// Crude check to tell users that they shouldn't include versions in their secrets
	if strings.Contains(secretName, "/versions/") {
		return nil, errors.Newf("invalid secret name %q: secrets should not be versioned with '/version/'",
			secretName)
	}
	// Check for 'projects/{project}/secrets/{secretName}'
	if strings.HasPrefix(secretName, "projects/") {
		secretNameParts := strings.SplitN(secretName, "/", 4)
		if len(secretNameParts) != 4 {
			return nil, errors.Newf("invalid secret name %q: expected 'projects/'-prefixed name to have 4 '/'-delimited parts",
				secretName)
		}
		// Error on easy-to-make oopsies
		if secretNameParts[2] != "secrets" {
			return nil, errors.Newf("invalid secret name %q: found '/secret/' segment, did you mean '/secrets/'?",
				secretName)
		}

		return &externalSecret{
			key:       strings.ToLower(key),
			projectID: secretNameParts[1],
			secretID:  secretNameParts[3],
		}, nil
	}

	return nil, nil
}
