package graphqlbackend

import (
	"context"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (r *schemaResolver) SettingsSubject(ctx context.Context, args *struct{ ID graphql.ID }) (*settingsSubjectResolver, error) {
	n, err := r.nodeByID(ctx, args.ID)
	if err != nil {
		return nil, err
	}

	return settingsSubjectForNode(ctx, n)
}

var errUnknownSettingsSubject = errors.New("unknown settings subject")

type settingsSubjectResolver struct {
	// Exactly 1 of these fields must be set.
	defaultSettings *defaultSettingsResolver
	site            *siteResolver
	org             *OrgResolver
	user            *UserResolver
}

func resolverForSubject(ctx context.Context, logger log.Logger, db database.DB, subject api.SettingsSubject) (*settingsSubjectResolver, error) {
	switch {
	case subject.Default:
		return &settingsSubjectResolver{defaultSettings: newDefaultSettingsResolver(db)}, nil
	case subject.Site:
		return &settingsSubjectResolver{site: NewSiteResolver(logger, db)}, nil
	case subject.Org != nil:
		org, err := OrgByIDInt32(ctx, db, *subject.Org)
		if err != nil {
			return nil, err
		}
		return &settingsSubjectResolver{org: org}, nil
	case subject.User != nil:
		user, err := UserByIDInt32(ctx, db, *subject.User)
		if err != nil {
			return nil, err
		}
		return &settingsSubjectResolver{user: user}, nil
	default:
		return nil, errors.New("subject must have exactly one field set")
	}
}

func resolversForSubjects(ctx context.Context, logger log.Logger, db database.DB, subjects []api.SettingsSubject) (_ []*settingsSubjectResolver, err error) {
	res := make([]*settingsSubjectResolver, len(subjects))
	for i, subject := range subjects {
		res[i], err = resolverForSubject(ctx, logger, db, subject)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}

// settingsSubjectForNode fetches the settings subject for the given Node. If
// the node is not a valid settings subject, an error is returned.
func settingsSubjectForNode(ctx context.Context, n Node) (*settingsSubjectResolver, error) {
	switch s := n.(type) {
	case *siteResolver:
		return &settingsSubjectResolver{site: s}, nil

	case *UserResolver:
		// 🚨 SECURITY: Only the authenticated user can view their settings on
		// Sourcegraph.com.
		var err error
		if dotcom.SiteAdminCanViewAllUserData() {
			// 🚨 SECURITY: Only the user and site admins are allowed to view the user's settings.
			err = auth.CheckSiteAdminOrSameUser(ctx, s.db, s.user.ID)
		} else {
			// 🚨 SECURITY: Only the user is allowed to view the user's settings.
			err = auth.CheckSameUser(ctx, s.user.ID)
		}
		if err != nil {
			return nil, err
		}
		return &settingsSubjectResolver{user: s}, nil

	case *OrgResolver:
		// 🚨 SECURITY: Check that the current user is a member of the org.
		if err := auth.CheckOrgAccessOrSiteAdmin(ctx, s.db, s.org.ID); err != nil {
			return nil, err
		}
		return &settingsSubjectResolver{org: s}, nil

	default:
		return nil, errUnknownSettingsSubject
	}
}

func (s *settingsSubjectResolver) ToDefaultSettings() (*defaultSettingsResolver, bool) {
	return s.defaultSettings, s.defaultSettings != nil
}

func (s *settingsSubjectResolver) ToSite() (*siteResolver, bool) {
	return s.site, s.site != nil
}

func (s *settingsSubjectResolver) ToOrg() (*OrgResolver, bool) { return s.org, s.org != nil }

func (s *settingsSubjectResolver) ToUser() (*UserResolver, bool) { return s.user, s.user != nil }

func (s *settingsSubjectResolver) toSubject() api.SettingsSubject {
	switch {
	case s.site != nil:
		return api.SettingsSubject{Site: true}
	case s.org != nil:
		return api.SettingsSubject{Org: &s.org.org.ID}
	case s.user != nil:
		return api.SettingsSubject{User: &s.user.user.ID}
	default:
		panic("invalid settings subject")
	}
}

func (s *settingsSubjectResolver) ID() (graphql.ID, error) {
	switch {
	case s.defaultSettings != nil:
		return s.defaultSettings.ID(), nil
	case s.site != nil:
		return s.site.ID(), nil
	case s.org != nil:
		return s.org.ID(), nil
	case s.user != nil:
		return s.user.ID(), nil
	default:
		return "", errUnknownSettingsSubject
	}
}

func (s *settingsSubjectResolver) LatestSettings(ctx context.Context) (*settingsResolver, error) {
	switch {
	case s.defaultSettings != nil:
		return s.defaultSettings.LatestSettings(ctx)
	case s.site != nil:
		return s.site.LatestSettings(ctx)
	case s.org != nil:
		return s.org.LatestSettings(ctx)
	case s.user != nil:
		return s.user.LatestSettings(ctx)
	default:
		return nil, errUnknownSettingsSubject
	}
}

func (s *settingsSubjectResolver) SettingsURL() (*string, error) {
	switch {
	case s.defaultSettings != nil:
		return s.defaultSettings.SettingsURL(), nil
	case s.site != nil:
		return s.site.SettingsURL(), nil
	case s.org != nil:
		return s.org.SettingsURL(), nil
	case s.user != nil:
		return s.user.SettingsURL(), nil
	default:
		return nil, errUnknownSettingsSubject
	}
}

func (s *settingsSubjectResolver) ViewerCanAdminister(ctx context.Context) (bool, error) {
	switch {
	case s.defaultSettings != nil:
		return s.defaultSettings.ViewerCanAdminister(ctx)
	case s.site != nil:
		return s.site.ViewerCanAdminister(ctx)
	case s.org != nil:
		return s.org.ViewerCanAdminister(ctx)
	case s.user != nil:
		return s.user.viewerCanAdministerSettings()
	default:
		return false, errUnknownSettingsSubject
	}
}

func (s *settingsSubjectResolver) SettingsCascade() (*settingsCascade, error) {
	switch {
	case s.defaultSettings != nil:
		return s.defaultSettings.SettingsCascade(), nil
	case s.site != nil:
		return s.site.SettingsCascade(), nil
	case s.org != nil:
		return s.org.SettingsCascade(), nil
	case s.user != nil:
		return s.user.SettingsCascade(), nil
	default:
		return nil, errUnknownSettingsSubject
	}
}

func (s *settingsSubjectResolver) ConfigurationCascade() (*settingsCascade, error) {
	return s.SettingsCascade()
}
