package clouddeploy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"html/template"
	"io"

	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/spec"
)

//go:embed skaffold.yaml
var skaffoldAssets embed.FS

// NewCloudRunCustomTargetSkaffoldAssetsArchive generates an archive of assets
// required for 'gcloud deploy releases create', to be provided via the
// '--source' flag: https://cloud.google.com/sdk/gcloud/reference/deploy/releases/create#--source
func NewCloudRunCustomTargetSkaffoldAssetsArchive() (*bytes.Buffer, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	files, err := skaffoldAssets.ReadDir(".")
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() {
			return nil, errors.New("unexpected dir")
		}
		info, err := file.Info()
		if err != nil {
			return nil, err
		}
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return nil, err
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, err
		}

		f, err := skaffoldAssets.Open(file.Name())
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(tw, f); err != nil {
			return nil, err
		}
	}

	return &buf, nil
}

//go:embed target.template.yaml
var cloudDeployTargetTemplateRaw string

// cloudDeployTargetTemplate is used to generate a clouddeploy_target
// using 'customTarget', which is not yet suppported by the Terraform provider:
// https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/clouddeploy_target
var cloudDeployTargetTemplate = template.Must(template.New("cloudDeployTargetTemplate").
	Parse(cloudDeployTargetTemplateRaw))

// RenderSpec renders a Cloud Deploy pipeline specification for use with 'gcloud deploy apply'.
// It sumplements the in-Terraform configuration in dev/managedservicesplatform/internal/resource/deliverypipeline
// with additional configuration that is not yet available in Terraform:
//
// - clouddeploy_target with custom target type
func RenderSpec(
	service spec.ServiceSpec,
	build spec.BuildSpec,
	config spec.RolloutPipelineConfiguration,
	region string,
) (*bytes.Buffer, error) {
	var targetsSpec bytes.Buffer
	for i, stage := range config.Stages {
		if i != 0 { // if not first
			if _, err := targetsSpec.WriteString("\n---\n"); err != nil {
				return nil, err
			}
		}

		var b bytes.Buffer
		if err := cloudDeployTargetTemplate.Execute(&b, map[string]any{
			"Stage":   stage,
			"Service": service,
			"Build":   build,
			"Region":  region,

			// Stable naming: always a SA in the last stage's project.
			"CloudDeployServiceAccount": fmt.Sprintf("clouddeploy-executor@%s.iam.gserviceaccount.com",
				config.Stages[len(config.Stages)-1].ProjectID),
		}); err != nil {
			return nil, err
		}
		if _, err := targetsSpec.Write(b.Bytes()); err != nil {
			return nil, err
		}
	}

	return &targetsSpec, nil
}
