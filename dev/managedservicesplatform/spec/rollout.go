package spec

type RolloutSpec struct {
	// Stages specifies the order and environments through which releases
	// progress.
	Stages []RolloutStageSpec `yaml:"stages"`
	// Suspended prevents releases and rollouts from being created, rolled back,
	// etc using this rollout pipeline pipeline: https://cloud.google.com/deploy/docs/suspend-pipeline
	//
	// Set to true to prevent all deployments from being created through Cloud
	// Deploy. Note that this does NOT prevent manual deploys from happening
	// directly in Cloud Run.
	Suspended *bool `yaml:"suspended,omitempty"`
	// ServiceAccount is the email address of the service account to provision IAM access to create
	// releases for. Can be used to give access to the Service Account used in your CI pipeline,
	// instead of using the default releaser SA that MSP provisions.
	ServiceAccount *string `yaml:"serviceAccount,omitempty"`
}

func (r *RolloutSpec) GetStageByEnvironment(id string) *RolloutStageSpec {
	if r == nil {
		return nil
	}
	for _, stage := range r.Stages {
		if stage.EnvironmentID == id {
			return &stage
		}
	}
	return nil
}

type RolloutStageSpec struct {
	// EnvironmentID is the ID of the environment to use in this stage.
	// The specified environment MUST have 'deploy: { type: "rollout" }' configured.
	EnvironmentID string `yaml:"environment"`
}

// RolloutPipelineConfiguration is rendered from BuildPipelineConfiguration for use in
// stacks.
type RolloutPipelineConfiguration struct {
	// Stages is evaluated from OriginalSpec.Stages to include attributes
	// required to actually configure the stages.
	Stages []rolloutPipelineTargetConfiguration

	OriginalSpec RolloutSpec
}

// rolloutPipelineTargetConfiguration is an internal type that extends
// RolloutStageSpec with other top-level environment spec.
type rolloutPipelineTargetConfiguration struct {
	RolloutStageSpec
	// ProjectID is the project the target environmet lives in.
	ProjectID string
}

// BuildRolloutPipelineConfiguration evaluates a configuration for use in
// configuring a Cloud Deploy pipeline in the final environment of a rollout
// spec's stages.
func (s Spec) BuildRolloutPipelineConfiguration(env EnvironmentSpec) *RolloutPipelineConfiguration {
	if s.Rollout == nil {
		return nil
	}

	// We only need the configuration
	if s.Rollout.Stages[len(s.Rollout.Stages)-1].EnvironmentID != env.ID {
		return nil
	}

	var targets []rolloutPipelineTargetConfiguration
	for _, stage := range s.Rollout.Stages {
		env := s.GetEnvironment(stage.EnvironmentID)
		targets = append(targets, rolloutPipelineTargetConfiguration{
			ProjectID:        env.ProjectID,
			RolloutStageSpec: stage,
		})
	}
	return &RolloutPipelineConfiguration{
		Stages:       targets,
		OriginalSpec: *s.Rollout,
	}
}
