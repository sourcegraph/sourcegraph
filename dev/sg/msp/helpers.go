package msp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/clouddeploy"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/spec"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/stacks/cloudrun"
	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/terraformcloud"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	msprepo "github.com/sourcegraph/sourcegraph/dev/sg/msp/repo"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// useServiceArgument retrieves the service spec corresponding to the first
// argument.
func useServiceArgument(c *cli.Context) (*spec.Spec, error) {
	serviceID := c.Args().First()
	if serviceID == "" {
		return nil, errors.New("argument service is required")
	}
	serviceSpecPath := msprepo.ServiceYAMLPath(serviceID)

	s, err := spec.Open(serviceSpecPath)
	if err != nil {
		return nil, errors.Wrapf(err, "load service %q", serviceID)
	}
	return s, nil
}

// useServiceAndEnvironmentArguments retrieves the service and environment specs
// corresponding to the first and second arguments respectively. It should only
// be used if both arguments are required.
func useServiceAndEnvironmentArguments(c *cli.Context) (*spec.Spec, *spec.EnvironmentSpec, error) {
	svc, err := useServiceArgument(c)
	if err != nil {
		return nil, nil, err
	}

	environmentID := c.Args().Get(1)
	if environmentID == "" {
		return svc, nil, errors.New("second argument <environment ID> is required")
	}

	env := svc.GetEnvironment(environmentID)
	if env == nil {
		return svc, nil, errors.Newf("environment %q not found in service spec, available environments: %+v",
			environmentID, svc.ListEnvironmentIDs())
	}

	return svc, env, nil
}

func syncEnvironmentWorkspaces(c *cli.Context, tfc *terraformcloud.Client, service spec.ServiceSpec, env spec.EnvironmentSpec) error {
	if c.Bool("delete") {
		if !pointers.DerefZero(env.AllowDestroys) {
			return errors.Newf("environments[%s].allowDestroys must be 'true' to delete workspaces", env.ID)
		}

		std.Out.Promptf("[%s] Deleting workspaces for environment %q - are you sure? (y/N) ",
			service.ID, env.ID)
		var input string
		if _, err := fmt.Scan(&input); err != nil {
			return err
		}
		if input != "y" {
			return errors.New("aborting")
		}

		pending := std.Out.Pending(output.Styledf(output.StylePending,
			"[%s] Deleting Terraform Cloud workspaces for environment %q", service.ID, env.ID))

		// Destroy stacks in reverse order
		stacks := managedservicesplatform.StackNames()
		slices.Reverse(stacks)
		if errs := tfc.DeleteWorkspaces(c.Context, service, env, stacks); len(errs) > 0 {
			for _, err := range errs {
				std.Out.WriteWarningf(err.Error())
			}
			return errors.New("some errors occurred when deleting workspaces")
		}
		pending.Complete(output.Styledf(output.StyleSuccess,
			"[%s] Deleting Terraform Cloud workspaces for environment %q", service.ID, env.ID))

		return nil // exit early for deletion, we are done
	}

	pending := std.Out.Pending(output.Styledf(output.StylePending,
		"[%s] Synchronizing Terraform Cloud workspaces for environment %q", service.ID, env.ID))
	workspaces, err := tfc.SyncWorkspaces(c.Context, service, env, managedservicesplatform.StackNames())
	if err != nil {
		return errors.Wrap(err, "sync Terraform Cloud workspace")
	}
	pending.Complete(output.Styledf(output.StyleSuccess,
		"[%s] Synchronized Terraform Cloud workspaces for environment %q", service.ID, env.ID))

	var summary strings.Builder
	for _, ws := range workspaces {
		summary.WriteString(fmt.Sprintf("- %s: %s", ws.Name, ws.URL()))
		if ws.Created {
			summary.WriteString(" (created)")
		} else {
			summary.WriteString(" (updated)")
		}
		summary.WriteString("\n")
	}
	return std.Out.WriteMarkdown(summary.String())
}

type generateTerraformOptions struct {
	// targetEnv generates the specified env only, otherwise generates all
	targetEnv string
	// stableGenerate disables updating of any values that are evaluated at
	// generation time
	stableGenerate bool
}

func generateTerraform(serviceID string, opts generateTerraformOptions) error {
	serviceSpecPath := msprepo.ServiceYAMLPath(serviceID)

	service, err := spec.Open(serviceSpecPath)
	if err != nil {
		return errors.Wrapf(err, "load service %q", serviceID)
	}

	var envs []spec.EnvironmentSpec
	if opts.targetEnv != "" {
		deployEnv := service.GetEnvironment(opts.targetEnv)
		if deployEnv == nil {
			return errors.Newf("environment %q not found in service spec", opts.targetEnv)
		}
		envs = append(envs, *deployEnv)
	} else {
		envs = service.Environments
	}

	for _, env := range envs {
		env := env

		pending := std.Out.Pending(output.Styledf(output.StylePending,
			"[%s] Preparing Terraform for environment %q", serviceID, env.ID))
		renderer := managedservicesplatform.Renderer{
			OutputDir:      filepath.Join(filepath.Dir(serviceSpecPath), "terraform", env.ID),
			StableGenerate: opts.stableGenerate,
		}

		// CDKTF needs the output dir to exist ahead of time, even for
		// rendering. If it doesn't exist yet, create it
		if f, err := os.Lstat(renderer.OutputDir); err != nil {
			if !os.IsNotExist(err) {
				return errors.Wrap(err, "check output directory")
			}
			if err := os.MkdirAll(renderer.OutputDir, 0755); err != nil {
				return errors.Wrap(err, "prepare output directory")
			}
		} else if !f.IsDir() {
			return errors.Newf("output directory %q is not a directory", renderer.OutputDir)
		}

		// Render environment
		cdktf, err := renderer.RenderEnvironment(*service, env)
		if err != nil {
			return err
		}

		pending.Updatef("[%s] Generating Terraform assets in %q for environment %q...",
			serviceID, renderer.OutputDir, env.ID)
		if err := cdktf.Synthesize(); err != nil {
			return err
		}

		if rollout := service.BuildRolloutPipelineConfiguration(env); rollout != nil {
			pending.Updatef("[%s] Building rollout pipeline configurations for environment %q...", serviceID, env.ID)

			// region is currently fixed
			region := cloudrun.GCPRegion
			deploySpec, err := clouddeploy.RenderSpec(
				service.Service,
				service.Build,
				*rollout,
				region)
			if err != nil {
				return errors.Wrap(err, "render Cloud Deploy configuration file")
			}

			deploySpecFilename := fmt.Sprintf("rollout-%s.clouddeploy.yaml", region)
			comment := generateCloudDeployDocstring(env.ProjectID, serviceID, region, deploySpecFilename)
			if err := os.WriteFile(
				filepath.Join(filepath.Dir(serviceSpecPath), deploySpecFilename),
				append([]byte(comment), deploySpec.Bytes()...),
				0644,
			); err != nil {
				return errors.Wrap(err, "write Cloud Deploy configuration file")
			}

			skaffoldObject, err := clouddeploy.NewCloudRunCustomTargetSkaffoldAssetsArchive()
			if err != nil {
				return errors.Wrap(err, "create Cloud Deploy custom target skaffold YAML archive")
			}
			skaffoldObjectPath := filepath.Join(renderer.OutputDir, "stacks/cloudrun", cloudrun.ScaffoldSourceFile)
			if err := os.WriteFile(skaffoldObjectPath, skaffoldObject.Bytes(), 0644); err != nil {
				return errors.Wrap(err, "write Cloud Run custom target skaffold YAML archive")
			}

		}

		pending.Complete(output.Styledf(output.StyleSuccess,
			"[%s] Infrastructure assets generated in %q!", serviceID, renderer.OutputDir))
	}

	return nil
}

func isHandbookRepo(relPath string) error {
	path, err := filepath.Abs(relPath)
	if err != nil {
		return errors.Wrapf(err, "unable to infer absolute path of %q", relPath)
	}

	// https://sourcegraph.com/github.com/sourcegraph/handbook/-/blob/package.json?L2=
	const handbookPackageName = "@sourcegraph/handbook.sourcegraph.com"

	packageJSONData, err := os.ReadFile(filepath.Join(path, "package.json"))
	if err != nil {
		return errors.Wrap(err, "expected package.json")
	}

	var packageJSON struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(packageJSONData, &packageJSON); err != nil {
		return errors.Wrap(err, "parse package.json")
	}
	if packageJSON.Name == handbookPackageName {
		return nil
	}
	return errors.Newf("unexpected package %q", packageJSON.Name)
}

func generateCloudDeployDocstring(projectID, serviceID, gcpRegion, cloudDeployFilename string) string {
	return fmt.Sprintf(`# DO NOT EDIT; generated by 'sg msp generate'
#
# This file defines additional Cloud Deploy configuration that is not yet available in Terraform.
# Apply this using the following command:
#
#   gcloud deploy apply --project=%[1]s --region=%[3]s --file=%[4]s
#
# Releases can be created using the following command, which can be added to CI pipelines:
#
#   gcloud deploy releases create $RELEASE_NAME --labels="commit=$COMMIT,author=$AUTHOR" --deploy-parameters="customTarget/tag=$TAG" --project=%[1]s --region=%[3]s --delivery-pipeline=%[2]s-%[3]s-rollout --source='gs://%[1]s-cloudrun-skaffold/source.tar.gz'
#
# The secret 'cloud_deploy_releaser_service_account_id' provides the ID of a service account
# that can be used to provision workload auth, for example https://sourcegraph.sourcegraph.com/github.com/sourcegraph/infrastructure/-/blob/managed-services/continuous-deployment-pipeline/main.tf?L5-20
`, // TODO improve the releases DX
		projectID, serviceID, gcpRegion, cloudDeployFilename)
}
