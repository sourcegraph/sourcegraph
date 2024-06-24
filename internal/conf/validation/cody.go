package validation

import (
	"fmt"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
)

func init() {
	contributeWarning(func(c conftypes.SiteConfigQuerier) (problems conf.Problems) {
		if c.SiteConfig().CodyRestrictUsersFeatureFlag != nil {
			problems = append(problems, conf.NewSiteProblem("cody.restrictUsersFeatureFlag has been deprecated. Please remove it from your site config and use cody.permissions instead: https://sourcegraph.com/docs/cody/overview/enable-cody-enterprise#enable-cody-only-for-some-users"))
		}
		return
	})
	conf.ContributeValidator(completionsConfigValidator)
}

const bedrockArnMessageTemplate = "completions.%s is invalid. Provisioned Capacity IDs must be formatted like \"model_id/provisioned_capacity_arn\".\nFor example \"anthropic.claude-instant-v1/%s\""

func completionsConfigValidator(q conftypes.SiteConfigQuerier) conf.Problems {
	problems := []string{}
	completionsConf := q.SiteConfig().Completions
	if completionsConf == nil {
		return nil
	}

	if completionsConf.Enabled != nil && q.SiteConfig().CodyEnabled == nil {
		problems = append(problems, "'completions.enabled' has been superceded by 'cody.enabled', please migrate to the new configuration.")
	}

	// Check for bedrock Provisioned Capacity ARNs which should instead be
	// formatted like:
	// "anthropic.claude-v2/arn:aws:bedrock:us-west-2:012345678901:provisioned-model/xxxxxxxx"
	if completionsConf.Provider == string(conftypes.CompletionsProviderNameAWSBedrock) {
		type modelID struct {
			value string
			field string
		}
		allModelIds := []modelID{
			{value: completionsConf.ChatModel, field: "chatModel"},
			{value: completionsConf.FastChatModel, field: "fastChatModel"},
			{value: completionsConf.CompletionModel, field: "completionModel"},
		}
		var modelIdsToCheck []modelID
		for _, modelId := range allModelIds {
			if modelId.value != "" {
				modelIdsToCheck = append(modelIdsToCheck, modelId)
			}
		}

		for _, modelId := range modelIdsToCheck {
			// When using provisioned capacity we expect an admin would just put the ARN
			// here directly, but we need both the model AND the ARN. Hence the check.
			if strings.HasPrefix(modelId.value, "arn:aws:") {
				problems = append(problems, fmt.Sprintf(bedrockArnMessageTemplate, modelId.field, modelId.value))
			}
		}
	}

	if len(problems) > 0 {
		return conf.NewSiteProblems(problems...)
	}

	return nil
}
