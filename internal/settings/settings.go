package settings

import (
	"context"
	"reflect"
	"sort"

	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/schema"
)

func defaultSettings() *schema.Settings {
	return &schema.Settings{
		ExperimentalFeatures: &schema.SettingsExperimentalFeatures{},
	}
}

// Deprecated: use Mock
var MockCurrentUserFinal *schema.Settings

// Service calculates settings for users and other subjects.
type Service interface {
	// UserFromContext returns the merged settings for the current user. If
	// there is no active user, it returns the site settings.
	UserFromContext(context.Context) (*schema.Settings, error)

	// ForSubject returns the merged settings for the given subject.
	//
	// A "subject" is either a user, an org, the site, or the default. Each
	// subject has a set of relevant subjects. To calculate a user's final
	// settings, the settings a user specifically sets are needed, as are the
	// settings for all orgs a user belongs to, as is the global site
	// settings, as is the default settings. These are the "relevant
	// subjects." The settings for all these "relevant subjects" are merged
	// together to get the final set of settings.
	ForSubject(context.Context, api.SettingsSubject) (*schema.Settings, error)

	// RelevantSubjects returns a list of subjects whose settings are
	// applicable to the given subject.
	//
	// These are returned in priority order, with the lowest priority first.
	// The order of priority is default < site < org < user.
	RelevantSubjects(context.Context, api.SettingsSubject) ([]api.SettingsSubject, error)
}

func NewService(db database.DB) Service {
	return &service{db: db}
}

type service struct {
	db database.DB
}

func (s *service) UserFromContext(ctx context.Context) (*schema.Settings, error) {
	if MockCurrentUserFinal != nil {
		return MockCurrentUserFinal, nil
	}

	currentUser := actor.FromContext(ctx)
	if !currentUser.IsAuthenticated() {
		// An unauthenticated user has no user-specific or org-specific
		// settings, so its relevant settings subject is the site subject.
		return s.ForSubject(ctx, api.SettingsSubject{Site: true})
	}
	return s.ForSubject(ctx, api.SettingsSubject{User: &currentUser.UID})
}

func (s *service) ForSubject(ctx context.Context, subject api.SettingsSubject) (_ *schema.Settings, err error) {
	tr, ctx := trace.New(ctx, "settings.ForSubject")
	defer func() {
		tr.SetError(err)
		tr.End()
	}()

	subjects, err := s.RelevantSubjects(ctx, subject)
	if err != nil {
		return nil, err
	}

	allSettings := make([]*schema.Settings, len(subjects))
	for i, subject := range subjects {
		allSettings[i], err = latest(ctx, s.db, subject)
		if err != nil {
			return nil, err
		}
	}

	return mergeSettings(allSettings...), nil
}

// latest gets the latest settings specific to a given subject. If no settings
// have been defined for a subject, latest will return nil.
func latest(ctx context.Context, db database.DB, subject api.SettingsSubject) (*schema.Settings, error) {
	// The store does not handle the default settings subject
	if subject.Default {
		return defaultSettings(), nil
	}

	settings, err := db.Settings().GetLatest(ctx, subject)
	if err != nil {
		return nil, err
	}

	if settings == nil {
		// No settings have been defined for this subject
		return nil, nil
	}

	var unmarshalled schema.Settings
	if err := jsonc.Unmarshal(settings.Contents, &unmarshalled); err != nil {
		return nil, err
	}
	return &unmarshalled, nil
}

func (s *service) RelevantSubjects(ctx context.Context, subject api.SettingsSubject) ([]api.SettingsSubject, error) {
	switch {
	case subject.Default:
		return []api.SettingsSubject{
			subject,
		}, nil
	case subject.Site:
		return []api.SettingsSubject{
			{Default: true},
			subject,
		}, nil
	case subject.Org != nil:
		return []api.SettingsSubject{
			{Default: true},
			{Site: true},
			subject,
		}, nil
	case subject.User != nil:
		subjects := []api.SettingsSubject{
			{Default: true},
			{Site: true},
		}

		orgs, err := s.db.Orgs().GetByUserID(ctx, *subject.User)
		if err != nil {
			return nil, err
		}

		// Stable-sort the orgs so that the priority of their settings is stable.
		sort.Slice(orgs, func(i, j int) bool { return orgs[i].ID < orgs[j].ID })

		// Apply the user's orgs' settings.
		for _, org := range orgs {
			subjects = append(subjects, api.SettingsSubject{Org: &org.ID})
		}

		// Apply the user's own settings last (it has highest priority).
		subjects = append(subjects, subject)
		return subjects, nil
	default:
		return nil, errors.New("subject must have exactly one field set")
	}
}

func mergeSettings(allSettings ...*schema.Settings) *schema.Settings {
	var merged *schema.Settings
	for _, subjectSettings := range allSettings {
		merged = mergeSettingsLeft(merged, subjectSettings)
	}
	return merged
}

func mergeSettingsLeft(left, right *schema.Settings) *schema.Settings {
	return mergeLeft(reflect.ValueOf(left), reflect.ValueOf(right), 1).Interface().(*schema.Settings)
}

var settingsFieldMergeDepths = map[string]int{
	"SearchScopes":         1,
	"SearchSavedQueries":   1,
	"Motd":                 1,
	"Notices":              1,
	"Extensions":           1,
	"ExperimentalFeatures": 1,
}

// mergeLeft takes two values of the same type and merges them if possible, ignoring
// any struct fields not listed in deeplyMergedSettingsFieldNames. Its depth parameter
// specifies how many layers deeper to merge, and will be overridden if the struct
// field name matches a name in settingsFieldMergeDepths.
func mergeLeft(left, right reflect.Value, depth int) reflect.Value {
	if left.IsZero() {
		return right
	}

	if right.IsZero() {
		return left
	}

	switch left.Kind() {
	case reflect.Struct:
		if depth <= 0 {
			return right
		}
		leftType := left.Type()
		for i := range left.NumField() {
			fieldName := leftType.Field(i).Name
			leftField := left.Field(i)
			rightField := right.Field(i)

			fieldDepth := settingsFieldMergeDepths[fieldName]
			leftField.Set(mergeLeft(leftField, rightField, fieldDepth))
		}
		return left
	case reflect.Map:
		if depth <= 0 {
			return right
		}
		iter := right.MapRange()
		for iter.Next() {
			k := iter.Key()
			rightVal := iter.Value()
			leftVal := left.MapIndex(k)
			if (leftVal != reflect.Value{}) {
				left.SetMapIndex(k, mergeLeft(leftVal, rightVal, depth-1))
			} else {
				left.SetMapIndex(k, rightVal)
			}
		}
		return left
	case reflect.Ptr:
		if depth <= 0 {
			return right
		}
		// Don't decrement depth for pointer deref
		left.Elem().Set(mergeLeft(left.Elem(), right.Elem(), depth))
		return left
	case reflect.Slice:
		if depth <= 0 {
			return right
		}
		return reflect.AppendSlice(left, right)
	}

	// Type is not mergeable, so clobber existing value
	return right
}

// Mock will return itself for UserFromContext and ForSubject.
func Mock(settings *schema.Settings) Service {
	return mock{settings: settings}
}

type mock struct {
	settings *schema.Settings
}

func (m mock) UserFromContext(ctx context.Context) (*schema.Settings, error) {
	return m.settings, nil
}
func (m mock) ForSubject(ctx context.Context, subject api.SettingsSubject) (*schema.Settings, error) {
	return m.settings, nil
}
func (m mock) RelevantSubjects(ctx context.Context, subject api.SettingsSubject) ([]api.SettingsSubject, error) {
	return nil, nil
}

// CurrentUserFinal returns the merged settings for the current user.
// If there is no active user, it returns the site settings.
//
// NOTE: use a settings.Service instead.
func CurrentUserFinal(ctx context.Context, db database.DB) (*schema.Settings, error) {
	return NewService(db).UserFromContext(ctx)
}

// Final returns the merged settings for the given subject.
//
// NOTE: use a settings.Service instead.
func Final(ctx context.Context, db database.DB, subject api.SettingsSubject) (*schema.Settings, error) {
	return NewService(db).ForSubject(ctx, subject)
}

// RelevantSubjects returns a list of subjects whose settings are applicable to the given subject.
// These are returned in priority order, with the lowest priority first.
// The order of priority is default < site < org < user.
//
// NOTE: use a settings.Service instead.
func RelevantSubjects(ctx context.Context, db database.DB, subject api.SettingsSubject) ([]api.SettingsSubject, error) {
	return NewService(db).RelevantSubjects(ctx, subject)
}
