package deliverypipeline

import (
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/hashicorp/terraform-cdk-go/cdktf"

	"github.com/sourcegraph/managed-services-platform-cdktf/gen/google/clouddeploydeliverypipeline"

	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/internal/resourceid"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

type Config struct {
	// Location currently must also be the location of all targets being
	// deployed by this pipeline.
	Location string

	Name        string
	Description string

	// Stages lists target IDs in order.
	Stages []string
	// Suspended prevents releases and rollouts from being created, rolled back,
	// etc using this pipeline: https://cloud.google.com/deploy/docs/suspend-pipeline
	Suspended bool

	DependsOn []cdktf.ITerraformDependable
}

type Output struct {
	PipelineID string
}

// New provisions resources for a google_clouddeploy_delivery_pipeline:
// https://cloud.google.com/deploy/docs/overview
func New(scope constructs.Construct, id resourceid.ID, config Config) (*Output, error) {
	pipeline := clouddeploydeliverypipeline.NewClouddeployDeliveryPipeline(scope,
		id.TerraformID("pipeline"),
		&clouddeploydeliverypipeline.ClouddeployDeliveryPipelineConfig{
			Location: &config.Location,

			Name:        &config.Name,
			Description: &config.Description,
			Suspended:   &config.Suspended,

			SerialPipeline: &clouddeploydeliverypipeline.ClouddeployDeliveryPipelineSerialPipeline{
				Stages: pointers.Ptr(newStages(config)),
			},

			DependsOn: &config.DependsOn,
		})

	return &Output{
		PipelineID: *pipeline.Uid(),
	}, nil
}

func newStages(config Config) []*clouddeploydeliverypipeline.ClouddeployDeliveryPipelineSerialPipelineStages {
	var stages []*clouddeploydeliverypipeline.ClouddeployDeliveryPipelineSerialPipelineStages
	for _, target := range config.Stages {
		stages = append(stages, &clouddeploydeliverypipeline.ClouddeployDeliveryPipelineSerialPipelineStages{
			TargetId: pointers.Ptr(target),
		})
	}
	return stages
}
