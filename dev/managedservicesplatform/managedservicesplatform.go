// Package managedservicesplatform manages infrastructure-as-code using CDKTF
// for Managed Services Platform (MSP) services.
package managedservicesplatform

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"

	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/stacks/cloudrun"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/stacks/iam"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/stacks/monitoring"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/stacks/project"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/stacks/tfcworkspaces"

	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack/options/terraformversion"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/stack/options/tfcbackend"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/terraform"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/spec"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/terraformcloud"
)

// StackNames lists the names of all Terraform stacks that are included in a
// typical MSP deployment, in the order they are provisioned.
//
// This MUST line up with the implementation in (Renderer).RenderEnvironment()
func StackNames() []string {
	return []string{
		project.StackName,
		iam.StackName,
		cloudrun.StackName,
		monitoring.StackName,
		tfcworkspaces.StackName,
	}
}

// Renderer takes MSP service specifications
type Renderer struct {
	// OutputDir is the target directory for generated CDKTF assets.
	OutputDir string
	// StableGenerate, if true, is propagated to stacks to indicate that any values
	// populated at generation time should not be regenerated.
	StableGenerate bool
}

// RenderEnvironment sets up a CDKTF application comprised of stacks that define
// the infrastructure required to deploy an environment as specified.
//
// Each stack is expected to be backed by a Terraform Cloud workspace with the
// following naming format:
//
//	msp-${svc.id}-${env.id}-${stackName}
//
// The required workspaces are managed by 'sg msp tfc sync'.
func (r *Renderer) RenderEnvironment(
	svc spec.Spec,
	env spec.EnvironmentSpec,
) (*CDKTF, error) {
	terraformVersion := terraform.Version
	stacks := stack.NewSet(r.OutputDir,
		// Enforce Terraform versions on all stacks
		terraformversion.With(terraformVersion),
		// Use a Terraform Cloud backend on all stacks - these should be
		// provisioned separately.
		tfcbackend.With(tfcbackend.Config{
			Workspace: func(stackName string) string {
				return terraformcloud.WorkspaceName(svc.Service, env, stackName)
			},
		}))

	// If destroys are not allowed, configure relevant resources to prevent
	// destroys.
	preventDestroys := !pointers.DerefZero(env.AllowDestroys)

	// Only non-nil if this is the last stage in a rollout spec.
	rolloutPipeline := svc.BuildRolloutPipelineConfiguration(env)

	// Render all required CDKTF stacks for this environment.
	//
	// This MUST line up with managedservicesplatform.StackNames() in this
	// package.
	projectOutput, err := project.NewStack(stacks, project.Variables{
		ProjectID:   env.ProjectID,
		DisplayName: fmt.Sprintf("%s - %s", svc.Service.GetName(), env.ID),

		Category: env.Category,
		Labels: map[string]string{
			"service":     svc.Service.ID,
			"environment": env.ID,
			"category":    string(env.Category),
			"msp":         "true",
		},
		Services: func() []string {
			if svc.Service.IAM != nil && len(svc.Service.IAM.Services) > 0 {
				return svc.Service.IAM.Services
			}
			return nil
		}(),
		PreventDestroys: preventDestroys,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create project stack")
	}
	iamOutput, err := iam.NewStack(stacks, iam.Variables{
		ProjectID:       *projectOutput.Project.ProjectId(),
		Image:           svc.Build.Image,
		Service:         svc.Service,
		SecretEnv:       env.SecretEnv,
		SecretVolumes:   env.SecretVolumes,
		PreventDestroys: preventDestroys,

		IsFinalStageOfRollout: rolloutPipeline != nil,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create IAM stack")
	}
	cloudrunOutput, err := cloudrun.NewStack(stacks, cloudrun.Variables{
		ProjectID: *projectOutput.Project.ProjectId(),
		IAM:       *iamOutput,

		Service:     svc.Service,
		Image:       svc.Build.Image,
		Environment: env,

		RolloutPipeline: rolloutPipeline,

		StableGenerate: r.StableGenerate,

		PreventDestroys: preventDestroys,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cloudrun stack")
	}
	if _, err := monitoring.NewStack(stacks, monitoring.Variables{
		ProjectID:           *projectOutput.Project.ProjectId(),
		Service:             svc.Service,
		EnvironmentCategory: env.Category,
		EnvironmentID:       env.ID,
		Alerting:            pointers.DerefZero(env.Alerting),

		Monitoring:            *svc.Monitoring,
		MaxInstanceCount:      env.Instances.Scaling.GetMaxCount(), // returns nil if not relevant
		ExternalDomain:        pointers.DerefZero(env.EnvironmentServiceSpec).Domain,
		ServiceAuthentication: pointers.DerefZero(env.EnvironmentServiceSpec).Authentication,
		DiagnosticsSecret:     cloudrunOutput.DiagnosticsSecret,
		RedisInstanceID:       cloudrunOutput.RedisInstanceID,
		CloudSQLInstanceID:    cloudrunOutput.CloudSQLInstanceID,
		CloudSQLMaxConections: pointers.DerefZero(pointers.DerefZero(env.Resources).PostgreSQL).MaxConnections,
		ServiceHealthProbes:   pointers.DerefZero(env.EnvironmentServiceSpec).HealthProbes,
		SentryProject:         cloudrunOutput.SentryProject,
		JobSchedule:           pointers.DerefZero(env.EnvironmentJobSpec).Schedule,
	}); err != nil {
		return nil, errors.Wrap(err, "failed to create monitoring stack")
	}

	// The tfcworkspaces stack manages initial applies/teardowns and other
	// workspace configuration not covered by 'sg msp tfc sync'.
	if _, err := tfcworkspaces.NewStack(stacks, tfcworkspaces.Variables{
		PreviousStacks: stack.ExtractStacks(stacks),
		// TODO: Maybe include spec option to disable notifications
		EnableNotifications: true,
	}); err != nil {
		return nil, errors.Wrap(err, "failed to create TFC workspace runs stack")
	}

	// Return CDKTF representation for caller to synthesize
	return &CDKTF{
		app:              stack.ExtractApp(stacks),
		stacks:           stack.ExtractStacks(stacks),
		terraformVersion: terraformVersion,
	}, nil
}
