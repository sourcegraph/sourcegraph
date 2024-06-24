package graphqlbackend

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type PersonResolver struct {
	db    database.DB
	name  string
	email string

	// fetch + serve sourcegraph stored user information
	includeUserInfo bool

	// cache result because it is used by multiple fields
	once sync.Once
	user *types.User
	err  error
}

func NewPersonResolver(db database.DB, name, email string, includeUserInfo bool) *PersonResolver {
	return &PersonResolver{
		db:              db,
		name:            name,
		email:           email,
		includeUserInfo: includeUserInfo,
	}
}

func NewPersonResolverFromUser(db database.DB, email string, user *types.User) *PersonResolver {
	return &PersonResolver{
		db:    db,
		user:  user,
		email: email,
		// We don't need to query for user.
		includeUserInfo: false,
	}
}

// resolveUser resolves the person to a user (using the email address). Not all persons can be
// resolved to a user.
func (r *PersonResolver) resolveUser(ctx context.Context) (*types.User, error) {
	r.once.Do(func() {
		if r.includeUserInfo && r.email != "" {
			r.user, r.err = r.db.Users().GetByVerifiedEmail(ctx, r.email)
			if errcode.IsNotFound(r.err) {
				r.err = nil
			}
		}
	})
	return r.user, r.err
}

func (r *PersonResolver) Name(ctx context.Context) (string, error) {
	user, err := r.resolveUser(ctx)
	if err != nil {
		return "", err
	}
	if user != nil && user.Username != "" {
		return user.Username, nil
	}

	// Fall back to provided username.
	return r.name, nil
}

func unicodeToAscii(input string) string {
	// Decompose the string
	t := norm.NFD.String(input)

	// Remove all non-spacing marks
	result := strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, t)

	// Recompose the string if necessary
	return norm.NFC.String(result)
}

func (r *PersonResolver) Email(ctx context.Context) (string, error) {
	if !dotcom.IsAbusePreventionEnabled() {
		return r.email, nil
	}

	// make sure we don't return valid emails to unauthenticated users
	user, err := auth.CurrentUser(ctx, r.db)
	if err != nil {
		return "", err
	}

	if user != nil {
		return r.email, nil
	}

	name, err := r.Name(ctx)
	if err != nil {
		return "", err
	}

	cleanedName := strings.ReplaceAll(strings.ToLower(name), " ", "")

	return url.PathEscape(unicodeToAscii(cleanedName)) + "@noreply.sourcegraph.com", nil
}

func (r *PersonResolver) DisplayName(ctx context.Context) (string, error) {
	user, err := r.resolveUser(ctx)
	if err != nil {
		return "", err
	}
	if user != nil && user.DisplayName != "" {
		return user.DisplayName, nil
	}

	if name := strings.TrimSpace(r.name); name != "" {
		return name, nil
	}
	if r.email != "" {
		return r.email, nil
	}
	return "unknown", nil
}

func (r *PersonResolver) AvatarURL(ctx context.Context) (*string, error) {
	user, err := r.resolveUser(ctx)
	if err != nil {
		return nil, err
	}
	if user != nil && user.AvatarURL != "" {
		return &user.AvatarURL, nil
	}
	return nil, nil
}

func (r *PersonResolver) User(ctx context.Context) (*UserResolver, error) {
	user, err := r.resolveUser(ctx)
	if user == nil || err != nil {
		return nil, err
	}
	return NewUserResolver(ctx, r.db, user), nil
}

func (r *PersonResolver) OwnerField() string {
	return EnterpriseResolvers.ownResolver.PersonOwnerField(r)
}
