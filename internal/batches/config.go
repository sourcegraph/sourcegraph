package batches

import (
	"github.com/sourcegraph/sourcegraph/internal/conf"
)

func init() {
	conf.ContributeValidator(func(c conf.Unified) (problems conf.Problems) {
		if c.CampaignsEnabled != nil {
			problems = append(problems, conf.NewSiteProblem("The `automation.readAccess.enabled` property is deprecated and will be removed in a future release. Use `campaigns.enabled` to enable or disable the campaigns feature instead."))
		}

		if c.CampaignsRestrictToAdmins != nil {
			problems = append(problems, conf.NewSiteProblem("The `campaigns.readAccess.enabled` property is deprecated and will be removed in a future release. Use `campaigns.enabled` to enable or disable the campaigns feature instead."))
		}

		return
	})
}
