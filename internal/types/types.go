// Package types defines types used by the frontend.
package types

import (
	"database/sql"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goware/urlx"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/awscodecommit"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitolite"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/schema"
)

// A SourceInfo represents a source a Repo belongs to (such as an external service).
type SourceInfo struct {
	ID       string
	CloneURL string
}

// ExternalServiceID returns the ID of the external service this
// SourceInfo refers to.
func (i SourceInfo) ExternalServiceID() int64 {
	_, id := extsvc.DecodeURN(i.ID)
	return id
}

// Repo represents a source code repository.
type Repo struct {
	// ID is the unique numeric ID for this repository.
	ID api.RepoID
	// Name is the name for this repository (e.g., "github.com/user/repo"). It
	// is the same as URI, unless the user configures a non-default
	// repositoryPathPattern.
	//
	// Previously, this was called RepoURI.
	Name api.RepoName
	// URI is the full name for this repository (e.g.,
	// "github.com/user/repo"). See the documentation for the Name field.
	URI string
	// Description is a brief description of the repository.
	Description string
	// Fork is whether this repository is a fork of another repository.
	Fork bool
	// Archived is whether the repository has been archived.
	Archived bool
	// Private is whether the repository is private.
	Private bool
	// Cloned is whether the repository is cloned.
	Cloned bool
	// CreatedAt is when this repository was created on Sourcegraph.
	CreatedAt time.Time
	// UpdatedAt is when this repository's metadata was last updated on Sourcegraph.
	UpdatedAt time.Time
	// DeletedAt is when this repository was soft-deleted from Sourcegraph.
	DeletedAt time.Time
	// ExternalRepo identifies this repository by its ID on the external service where it resides (and the external
	// service itself).
	ExternalRepo api.ExternalRepoSpec
	// Sources identifies all the repo sources this Repo belongs to.
	// The key is a URN created by extsvc.URN
	Sources map[string]*SourceInfo
	// Metadata contains the raw source code host JSON metadata.
	Metadata interface{}
}

// CloneURLs returns all the clone URLs this repo is clonable from.
func (r *Repo) CloneURLs() []string {
	urls := make([]string, 0, len(r.Sources))
	for _, src := range r.Sources {
		if src != nil && src.CloneURL != "" {
			urls = append(urls, src.CloneURL)
		}
	}
	return urls
}

// IsDeleted returns true if the repo is deleted.
func (r *Repo) IsDeleted() bool { return !r.DeletedAt.IsZero() }

// ExternalServiceIDs returns the IDs of the external services this
// repo belongs to.
func (r *Repo) ExternalServiceIDs() []int64 {
	ids := make([]int64, 0, len(r.Sources))
	for _, src := range r.Sources {
		ids = append(ids, src.ExternalServiceID())
	}
	return ids
}

// Update updates Repo r with the fields from the given newer Repo n,
// returning true if modified.
func (r *Repo) Update(n *Repo) (modified bool) {
	if r.Name != n.Name {
		r.Name, modified = n.Name, true
	}

	if r.URI != n.URI {
		r.URI, modified = n.URI, true
	}

	if r.Description != n.Description {
		r.Description, modified = n.Description, true
	}

	if n.ExternalRepo != (api.ExternalRepoSpec{}) &&
		!r.ExternalRepo.Equal(&n.ExternalRepo) {
		r.ExternalRepo, modified = n.ExternalRepo, true
	}

	if r.Archived != n.Archived {
		r.Archived, modified = n.Archived, true
	}

	if r.Fork != n.Fork {
		r.Fork, modified = n.Fork, true
	}

	if r.Private != n.Private {
		r.Private, modified = n.Private, true
	}

	if !reflect.DeepEqual(r.Sources, n.Sources) {
		r.Sources, modified = n.Sources, true
	}

	// As a special case, we clear out the value of ViewerPermission for GitHub repos as
	// the value is dependent on the token used to fetch it. We don't want to store this in the DB as it will
	// flip flop as we fetch the same repo from different external services.
	switch x := n.Metadata.(type) {
	case *github.Repository:
		cp := *x
		cp.ViewerPermission = ""
		n = n.With(func(clone *Repo) {
			// Repo.Clone does not currently clone metadata for any types as they could contain hard to clone
			// items such as maps. However, we know that copying github.Repository is safe as it only contains values.
			clone.Metadata = &cp
		})
	}

	if !reflect.DeepEqual(r.Metadata, n.Metadata) {
		r.Metadata, modified = n.Metadata, true
	}

	return modified
}

// Clone returns a clone of the given repo.
func (r *Repo) Clone() *Repo {
	if r == nil {
		return nil
	}
	clone := *r
	if r.Sources != nil {
		clone.Sources = make(map[string]*SourceInfo, len(r.Sources))
		for k, v := range r.Sources {
			clone.Sources[k] = v
		}
	}
	return &clone
}

// Apply applies the given functional options to the Repo.
func (r *Repo) Apply(opts ...func(*Repo)) {
	if r == nil {
		return
	}

	for _, opt := range opts {
		opt(r)
	}
}

// With returns a clone of the given repo with the given functional options applied.
func (r *Repo) With(opts ...func(*Repo)) *Repo {
	clone := r.Clone()
	clone.Apply(opts...)
	return clone
}

// Less compares Repos by the important fields (fields with constraints in our
// DB). Additionally it will compare on Sources to give a deterministic order
// on repos returned from a sourcer.
//
// NewDiff relies on Less to deterministically decide on the order to merge
// repositories, as well as which repository to keep on conflicts.
//
// Context on using other fields such as timestamps to order/resolve
// conflicts: We only want to rely on values that have constraints in our
// database. Timestamps have the following downsides:
//
//   - We need to assume the upstream codehost has reasonable values for them
//   - Not all codehosts set them to relevant values (eg gitolite or other)
//   - They could change often for codehosts that do set them.
func (r *Repo) Less(s *Repo) bool {
	if r.ID != s.ID {
		return r.ID < s.ID
	}
	if r.Name != s.Name {
		return r.Name < s.Name
	}
	if cmp := r.ExternalRepo.Compare(s.ExternalRepo); cmp != 0 {
		return cmp == -1
	}

	return sortedSliceLess(sourcesKeys(r.Sources), sourcesKeys(s.Sources))
}

func (r *Repo) String() string {
	eid := fmt.Sprintf("{%s %s %s}", r.ExternalRepo.ServiceID, r.ExternalRepo.ServiceType, r.ExternalRepo.ID)
	if r.IsDeleted() {
		return fmt.Sprintf("Repo{ID: %d, Name: %q, EID: %s, IsDeleted: true}", r.ID, r.Name, eid)
	}
	return fmt.Sprintf("Repo{ID: %d, Name: %q, EID: %s}", r.ID, r.Name, eid)
}

func sourcesKeys(m map[string]*SourceInfo) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedSliceLess returns true if a < b
func sortedSliceLess(a, b []string) bool {
	for i, v := range a {
		if i == len(b) {
			return false
		}
		if v != b[i] {
			return v < b[i]
		}
	}
	return len(a) != len(b)
}

// Repos is an utility type with convenience methods for operating on lists of Repos.
type Repos []*Repo

func (rs Repos) Len() int           { return len(rs) }
func (rs Repos) Less(i, j int) bool { return rs[i].Less(rs[j]) }
func (rs Repos) Swap(i, j int)      { rs[i], rs[j] = rs[j], rs[i] }

// IDs returns the list of ids from all Repos.
func (rs Repos) IDs() []api.RepoID {
	ids := make([]api.RepoID, len(rs))
	for i := range rs {
		ids[i] = rs[i].ID
	}
	return ids
}

// Names returns the list of names from all Repos.
func (rs Repos) Names() []string {
	names := make([]string, len(rs))
	for i := range rs {
		names[i] = string(rs[i].Name)
	}
	return names
}

// NamesSummary caps the number of repos to 20 when composing a space-separated list string.
// Used in logging statements.
func (rs Repos) NamesSummary() string {
	if len(rs) > 20 {
		return strings.Join(rs[:20].Names(), " ") + "..."
	}
	return strings.Join(rs.Names(), " ")
}

// Kinds returns the unique set of kinds from all Repos.
func (rs Repos) Kinds() (kinds []string) {
	set := map[string]bool{}
	for _, r := range rs {
		kind := strings.ToUpper(r.ExternalRepo.ServiceType)
		if !set[kind] {
			kinds = append(kinds, kind)
			set[kind] = true
		}
	}
	return kinds
}

// ExternalRepos returns the list of set ExternalRepoSpecs from all Repos.
func (rs Repos) ExternalRepos() []api.ExternalRepoSpec {
	specs := make([]api.ExternalRepoSpec, 0, len(rs))
	for _, r := range rs {
		specs = append(specs, r.ExternalRepo)
	}
	return specs
}

// Sources returns a map of all the sources per repo id.
func (rs Repos) Sources() map[api.RepoID][]SourceInfo {
	sources := make(map[api.RepoID][]SourceInfo)
	for i := range rs {
		for _, info := range rs[i].Sources {
			sources[rs[i].ID] = append(sources[rs[i].ID], *info)
		}
	}

	return sources
}

// Concat adds the given Repos to the end of rs.
func (rs *Repos) Concat(others ...Repos) {
	for _, o := range others {
		*rs = append(*rs, o...)
	}
}

// Clone returns a clone of Repos.
func (rs Repos) Clone() Repos {
	o := make(Repos, 0, len(rs))
	for _, r := range rs {
		o = append(o, r.Clone())
	}
	return o
}

// Apply applies the given functional options to the Repo.
func (rs Repos) Apply(opts ...func(*Repo)) {
	for _, r := range rs {
		r.Apply(opts...)
	}
}

// With returns a clone of the given repos with the given functional options applied.
func (rs Repos) With(opts ...func(*Repo)) Repos {
	clone := rs.Clone()
	clone.Apply(opts...)
	return clone
}

// Filter returns all the Repos that match the given predicate.
func (rs Repos) Filter(pred func(*Repo) bool) (fs Repos) {
	for _, r := range rs {
		if pred(r) {
			fs = append(fs, r)
		}
	}
	return fs
}

// RepoName represents a source code repository name and its ID.
type RepoName struct {
	ID   api.RepoID
	Name api.RepoName
}

func (r *RepoName) ToRepo() *Repo {
	return &Repo{
		ID:   r.ID,
		Name: r.Name,
	}
}

// RepoNames is an utility type with convenience methods for operating on lists of repo names
type RepoNames []*RepoName

func (rs RepoNames) Len() int           { return len(rs) }
func (rs RepoNames) Less(i, j int) bool { return rs[i].ID < rs[j].ID }
func (rs RepoNames) Swap(i, j int)      { rs[i], rs[j] = rs[j], rs[i] }

type CodeHostRepository struct {
	Name       string
	CodeHostID int64
	Private    bool
}

// RepoGitserverStatus includes basic repo data along with the current gitserver
// status for the repo, which may be unknown.
type RepoGitserverStatus struct {
	// ID is the unique numeric ID for this repository.
	ID api.RepoID
	// Name is the name for this repository (e.g., "github.com/user/repo").
	Name api.RepoName

	// GitserverRepo data if it exists
	*GitserverRepo
}

type CloneStatus string

const (
	CloneStatusUnknown   CloneStatus = ""
	CloneStatusNotCloned CloneStatus = "not_cloned"
	CloneStatusCloning   CloneStatus = "cloning"
	CloneStatusCloned    CloneStatus = "cloned"
)

func ParseCloneStatus(s string) CloneStatus {
	cs := CloneStatus(s)
	switch cs {
	case CloneStatusNotCloned, CloneStatusCloning, CloneStatusCloned:
		return cs
	default:
		return CloneStatusUnknown
	}
}

// GitserverRepo  represents the data gitserver knows about a repo
type GitserverRepo struct {
	RepoID api.RepoID
	// Usually represented by a gitserver hostname
	ShardID     string
	CloneStatus CloneStatus
	// The last external service used to sync or clone this repo
	LastExternalService int64
	// The last error that occured or empty if the last action was successful
	LastError string
	UpdatedAt time.Time
}

// ExternalService is a connection to an external service.
type ExternalService struct {
	ID              int64
	Kind            string
	DisplayName     string
	Config          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       time.Time
	LastSyncAt      time.Time
	NextSyncAt      time.Time
	NamespaceUserID int32
	Unrestricted    bool // Whether access to repositories belong to this external service is unrestricted.
	CloudDefault    bool // Whether this external service is our default public service on Cloud
}

// URN returns a unique resource identifier of this external service,
// used as the key in a repo's Sources map as well as the SourceInfo ID.
func (e *ExternalService) URN() string {
	return extsvc.URN(e.Kind, e.ID)
}

// IsDeleted returns true if the external service is deleted.
func (e *ExternalService) IsDeleted() bool { return !e.DeletedAt.IsZero() }

// Update updates ExternalService e with the fields from the given newer ExternalService n,
// returning true if modified.
func (e *ExternalService) Update(n *ExternalService) (modified bool) {
	if e.ID != n.ID {
		return false
	}

	if !strings.EqualFold(e.Kind, n.Kind) {
		e.Kind, modified = strings.ToUpper(n.Kind), true
	}

	if e.DisplayName != n.DisplayName {
		e.DisplayName, modified = n.DisplayName, true
	}

	if e.Config != n.Config {
		e.Config, modified = n.Config, true
	}

	if !e.UpdatedAt.Equal(n.UpdatedAt) {
		e.UpdatedAt, modified = n.UpdatedAt, true
	}

	if !e.DeletedAt.Equal(n.DeletedAt) {
		e.DeletedAt, modified = n.DeletedAt, true
	}

	return modified
}

// Configuration returns the external service config.
func (e *ExternalService) Configuration() (cfg interface{}, _ error) {
	return extsvc.ParseConfig(e.Kind, e.Config)
}

// Exclude changes the configuration of an external service to exclude the given
// repos from being synced.
func (e *ExternalService) Exclude(rs ...*Repo) error {
	switch strings.ToUpper(e.Kind) {
	case extsvc.KindGitHub:
		return e.excludeGithubRepos(rs...)
	case extsvc.KindGitLab:
		return e.excludeGitLabRepos(rs...)
	case extsvc.KindBitbucketServer:
		return e.excludeBitbucketServerRepos(rs...)
	case extsvc.KindAWSCodeCommit:
		return e.excludeAWSCodeCommitRepos(rs...)
	case extsvc.KindGitolite:
		return e.excludeGitoliteRepos(rs...)
	case extsvc.KindOther:
		return e.excludeOtherRepos(rs...)
	default:
		return errors.Errorf("external service kind %q doesn't have an exclude list", e.Kind)
	}
}

// excludeOtherRepos changes the configuration of an OTHER external service to exclude
// the given repos.
func (e *ExternalService) excludeOtherRepos(rs ...*Repo) error {
	if len(rs) == 0 {
		return nil
	}

	return e.config(extsvc.KindOther, func(v interface{}) (string, interface{}, error) {
		c := v.(*schema.OtherExternalServiceConnection)

		var base *url.URL
		if c.Url != "" {
			var err error
			if base, err = url.Parse(c.Url); err != nil {
				return "", nil, err
			}
		}

		set := make(map[string]bool, len(c.Repos))
		for _, name := range c.Repos {
			if name != "" {
				u, err := otherRepoCloneURL(base, name)
				if err != nil {
					return "", nil, err
				}

				if name = u.String(); base != nil {
					name = nameWithOwner(name)
				}

				set[name] = true
			}
		}

		for _, r := range rs {
			if r.ExternalRepo.ServiceType != extsvc.TypeOther {
				continue
			}

			u, err := url.Parse(r.ExternalRepo.ServiceID)
			if err != nil {
				return "", nil, err
			}

			name := u.Scheme + "://" + string(r.Name)
			if base != nil {
				name = nameWithOwner(string(r.Name))
			}

			delete(set, name)
		}

		repos := make([]string, 0, len(set))
		for name := range set {
			repos = append(repos, name)
		}

		sort.Strings(repos)

		return "repos", repos, nil
	})
}

func otherRepoCloneURL(base *url.URL, repo string) (*url.URL, error) {
	if base == nil {
		return url.Parse(repo)
	}
	return base.Parse(repo)
}

// excludeGitLabRepos changes the configuration of a GitLab external service to exclude the
// given repos from being synced.
func (e *ExternalService) excludeGitLabRepos(rs ...*Repo) error {
	if len(rs) == 0 {
		return nil
	}

	return e.config(extsvc.KindGitLab, func(v interface{}) (string, interface{}, error) {
		c := v.(*schema.GitLabConnection)
		set := make(map[string]bool, len(c.Exclude)*2)
		for _, ex := range c.Exclude {
			if ex.Id != 0 {
				set[strconv.Itoa(ex.Id)] = true
			}

			if ex.Name != "" {
				set[strings.ToLower(ex.Name)] = true
			}
		}

		for _, r := range rs {
			p, ok := r.Metadata.(*gitlab.Project)
			if !ok {
				continue
			}

			name := p.PathWithNamespace
			id := strconv.Itoa(p.ID)

			if !set[name] && !set[id] {
				c.Exclude = append(c.Exclude, &schema.ExcludedGitLabProject{
					Name: name,
					Id:   p.ID,
				})

				if id != "" {
					set[id] = true
				}

				if name != "" {
					set[name] = true
				}
			}
		}

		return "exclude", c.Exclude, nil
	})
}

// excludeBitbucketServerRepos changes the configuration of a BitbucketServer external service to exclude the
// given repos from being synced.
func (e *ExternalService) excludeBitbucketServerRepos(rs ...*Repo) error {
	if len(rs) == 0 {
		return nil
	}

	return e.config(extsvc.KindBitbucketServer, func(v interface{}) (string, interface{}, error) {
		c := v.(*schema.BitbucketServerConnection)
		set := make(map[string]bool, len(c.Exclude)*2)
		for _, ex := range c.Exclude {
			if ex.Id != 0 {
				set[strconv.Itoa(ex.Id)] = true
			}

			if ex.Name != "" {
				set[strings.ToLower(ex.Name)] = true
			}
		}

		for _, r := range rs {
			repo, ok := r.Metadata.(*bitbucketserver.Repo)
			if !ok {
				continue
			}

			id := strconv.Itoa(repo.ID)

			// The names in the exclude list do not abide by the
			// repositoryPathPattern setting. They have a fixed format.
			name := repo.Slug
			if repo.Project != nil {
				name = repo.Project.Key + "/" + name
			}

			if !set[name] && !set[id] {
				c.Exclude = append(c.Exclude, &schema.ExcludedBitbucketServerRepo{
					Name: name,
					Id:   repo.ID,
				})

				if id != "" {
					set[id] = true
				}

				if name != "" {
					set[name] = true
				}
			}
		}

		return "exclude", c.Exclude, nil
	})
}

// excludeGitoliteRepos changes the configuration of a Gitolite external service to exclude the
// given repos from being synced.
func (e *ExternalService) excludeGitoliteRepos(rs ...*Repo) error {
	if len(rs) == 0 {
		return nil
	}

	return e.config(extsvc.KindGitolite, func(v interface{}) (string, interface{}, error) {
		c := v.(*schema.GitoliteConnection)
		set := make(map[string]bool, len(c.Exclude))
		for _, ex := range c.Exclude {
			if ex.Name != "" {
				set[ex.Name] = true
			}
		}

		for _, r := range rs {
			repo, ok := r.Metadata.(*gitolite.Repo)
			if ok && repo.Name != "" && !set[repo.Name] {
				c.Exclude = append(c.Exclude, &schema.ExcludedGitoliteRepo{Name: repo.Name})
				set[repo.Name] = true
			}
		}

		return "exclude", c.Exclude, nil
	})
}

// excludeGithubRepos changes the configuration of a Github external service to exclude the
// given repos from being synced.
func (e *ExternalService) excludeGithubRepos(rs ...*Repo) error {
	if len(rs) == 0 {
		return nil
	}

	return e.config(extsvc.KindGitHub, func(v interface{}) (string, interface{}, error) {
		c := v.(*schema.GitHubConnection)
		set := make(map[string]bool, len(c.Exclude)*2)
		for _, ex := range c.Exclude {
			if ex.Id != "" {
				set[ex.Id] = true
			}

			if ex.Name != "" {
				set[strings.ToLower(ex.Name)] = true
			}
		}

		for _, r := range rs {
			repo, ok := r.Metadata.(*github.Repository)
			if !ok {
				continue
			}

			id := repo.ID
			name := repo.NameWithOwner

			if !set[name] && !set[id] {
				c.Exclude = append(c.Exclude, &schema.ExcludedGitHubRepo{
					Name: name,
					Id:   id,
				})

				if id != "" {
					set[id] = true
				}

				if name != "" {
					set[name] = true
				}
			}
		}

		return "exclude", c.Exclude, nil
	})
}

// excludeAWSCodeCommitRepos changes the configuration of a AWS CodeCommit
// external service to exclude the given repos from being synced.
func (e *ExternalService) excludeAWSCodeCommitRepos(rs ...*Repo) error {
	if len(rs) == 0 {
		return nil
	}

	return e.config(extsvc.KindAWSCodeCommit, func(v interface{}) (string, interface{}, error) {
		c := v.(*schema.AWSCodeCommitConnection)
		set := make(map[string]bool, len(c.Exclude)*2)
		for _, ex := range c.Exclude {
			if ex.Id != "" {
				set[ex.Id] = true
			}

			if ex.Name != "" {
				set[strings.ToLower(ex.Name)] = true
			}
		}

		for _, r := range rs {
			repo, ok := r.Metadata.(*awscodecommit.Repository)
			if !ok {
				continue
			}

			id := repo.ID
			name := repo.Name

			if !set[name] && !set[id] {
				c.Exclude = append(c.Exclude, &schema.ExcludedAWSCodeCommitRepo{
					Name: name,
					Id:   id,
				})

				if id != "" {
					set[id] = true
				}

				if name != "" {
					set[name] = true
				}
			}
		}

		return "exclude", c.Exclude, nil
	})
}

func nameWithOwner(name string) string {
	u, _ := urlx.Parse(name)
	if u != nil {
		name = strings.TrimPrefix(u.Path, "/")
	}
	return strings.ToLower(name)
}

func (e *ExternalService) config(kind string, opt func(c interface{}) (string, interface{}, error)) error {
	if !strings.EqualFold(kind, e.Kind) {
		return fmt.Errorf("config: unexpected external service kind %q", e.Kind)
	}

	c, err := e.Configuration()
	if err != nil {
		return errors.Wrap(err, "config")
	}

	path, val, err := opt(c)
	if err != nil {
		return errors.Wrap(err, "config")
	}

	if !reflect.ValueOf(val).IsNil() {
		edited, err := jsonc.Edit(e.Config, val, strings.Split(path, ".")...)
		if err != nil {
			return errors.Wrap(err, "edit")
		}
		e.Config = edited
	}

	return e.validateConfig()
}

func (e ExternalService) schema() string {
	switch strings.ToUpper(e.Kind) {
	case extsvc.KindAWSCodeCommit:
		return schema.AWSCodeCommitSchemaJSON
	case extsvc.KindBitbucketServer:
		return schema.BitbucketServerSchemaJSON
	case extsvc.KindGitHub:
		return schema.GitHubSchemaJSON
	case extsvc.KindGitLab:
		return schema.GitLabSchemaJSON
	case extsvc.KindGitolite:
		return schema.GitoliteSchemaJSON
	case extsvc.KindPhabricator:
		return schema.PhabricatorSchemaJSON
	case extsvc.KindOther:
		return schema.OtherExternalServiceSchemaJSON
	default:
		return ""
	}
}

// validateConfig validates the config of an external service
// against its JSON schema.
func (e *ExternalService) validateConfig() error {
	sl := gojsonschema.NewSchemaLoader()
	sc, err := sl.Compile(gojsonschema.NewStringLoader(e.schema()))
	if err != nil {
		return errors.Wrapf(err, "failed to compile schema for external service of kind %q", e.Kind)
	}

	normalized, err := jsonc.Parse(e.Config)
	if err != nil {
		return errors.Wrapf(err, "failed to normalize JSON")
	}

	res, err := sc.Validate(gojsonschema.NewBytesLoader(normalized))
	if err != nil {
		return errors.Wrap(err, "failed to validate config against schema")
	}

	errs := new(multierror.Error)
	for _, err := range res.Errors() {
		errs = multierror.Append(errs, errors.New(err.String()))
	}

	return errs.ErrorOrNil()
}

// Clone returns a clone of the given external service.
func (e *ExternalService) Clone() *ExternalService {
	clone := *e
	return &clone
}

// Apply applies the given functional options to the ExternalService.
func (e *ExternalService) Apply(opts ...func(*ExternalService)) {
	if e == nil {
		return
	}

	for _, opt := range opts {
		opt(e)
	}
}

// With returns a clone of the given repo with the given functional options applied.
func (e *ExternalService) With(opts ...func(*ExternalService)) *ExternalService {
	clone := e.Clone()
	clone.Apply(opts...)
	return clone
}

// ExternalServices is an utility type with
// convenience methods for operating on lists of ExternalServices.
type ExternalServices []*ExternalService

// IDs returns the list of ids from all ExternalServices.
func (es ExternalServices) IDs() []int64 {
	ids := make([]int64, len(es))
	for i := range es {
		ids[i] = es[i].ID
	}
	return ids
}

// DisplayNames returns the list of display names from all ExternalServices.
func (es ExternalServices) DisplayNames() []string {
	names := make([]string, len(es))
	for i := range es {
		names[i] = es[i].DisplayName
	}
	return names
}

// Kinds returns the unique set of Kinds in the given external services list.
func (es ExternalServices) Kinds() (kinds []string) {
	set := make(map[string]bool, len(es))
	for _, e := range es {
		if !set[e.Kind] {
			kinds = append(kinds, e.Kind)
			set[e.Kind] = true
		}
	}
	return kinds
}

// URNs returns the list of URNs from all ExternalServices.
func (es ExternalServices) URNs() []string {
	urns := make([]string, len(es))
	for i := range es {
		urns[i] = es[i].URN()
	}
	return urns
}

func (es ExternalServices) Len() int {
	return len(es)
}

func (es ExternalServices) Swap(i, j int) {
	es[i], es[j] = es[j], es[i]
}

func (es ExternalServices) Less(i, j int) bool {
	return es[i].ID < es[j].ID
}

// Clone returns a clone of the given external services.
func (es ExternalServices) Clone() ExternalServices {
	o := make(ExternalServices, 0, len(es))
	for _, r := range es {
		o = append(o, r.Clone())
	}
	return o
}

// Apply applies the given functional options to the ExternalService.
func (es ExternalServices) Apply(opts ...func(*ExternalService)) {
	for _, r := range es {
		r.Apply(opts...)
	}
}

// With returns a clone of the given external services with the given functional options applied.
func (es ExternalServices) With(opts ...func(*ExternalService)) ExternalServices {
	clone := es.Clone()
	clone.Apply(opts...)
	return clone
}

type GlobalState struct {
	SiteID      string
	Initialized bool // whether the initial site admin account has been created
}

// User represents a registered user.
type User struct {
	ID                    int32
	Username              string
	DisplayName           string
	AvatarURL             string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	SiteAdmin             bool
	BuiltinAuth           bool
	Tags                  []string
	InvalidatedSessionsAt time.Time
}

type Org struct {
	ID          int32
	Name        string
	DisplayName *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OrgMembership struct {
	ID        int32
	OrgID     int32
	UserID    int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PhabricatorRepo struct {
	ID       int32
	Name     api.RepoName
	URL      string
	Callsign string
}

type UserUsageStatistics struct {
	UserID                      int32
	PageViews                   int32
	SearchQueries               int32
	CodeIntelligenceActions     int32
	FindReferencesActions       int32
	LastActiveTime              *time.Time
	LastCodeHostIntegrationTime *time.Time
}

// UserUsageCounts captures the usage numbers of a user in a single day.
type UserUsageCounts struct {
	Date           time.Time
	UserID         uint32
	SearchCount    int32
	CodeIntelCount int32
}

// UserDates captures the created and deleted dates of a single user.
type UserDates struct {
	UserID    int32
	CreatedAt time.Time
	DeletedAt time.Time
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type SiteUsageStatistics struct {
	DAUs []*SiteActivityPeriod
	WAUs []*SiteActivityPeriod
	MAUs []*SiteActivityPeriod
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type SiteActivityPeriod struct {
	StartTime            time.Time
	UserCount            int32
	RegisteredUserCount  int32
	AnonymousUserCount   int32
	IntegrationUserCount int32
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type CampaignsUsageStatistics struct {
	// ViewCampaignApplyPageCount is the number of page views on the apply page
	// ("preview" page).
	ViewCampaignApplyPageCount int32
	// ViewCampaignDetailsPageAfterCreateCount is the number of page views on
	// the campaigns details page *after creating* the campaign on the apply
	// page by clicking "Apply spec".
	ViewCampaignDetailsPageAfterCreateCount int32
	// ViewCampaignDetailsPageAfterUpdateCount is the number of page views on
	// the campaigns details page *after updating* a campaign on the apply page
	// by clicking "Apply spec".
	ViewCampaignDetailsPageAfterUpdateCount int32

	// CampaignsCount is the number of campaigns on the instance. This can go
	// down when users delete a campaign.
	CampaignsCount int32
	// CampaignsClosedCount is the number of *closed* campaigns on the
	// instance. This can go down when users delete a campaign.
	CampaignsClosedCount int32

	// CampaignSpecsCreatedCount is the number of campaign specs that have been
	// created by running `src campaign [preview|apply]`. This number never
	// goes down since it's based on event logs, even if the campaign specs
	// were not used and cleaned up.
	CampaignSpecsCreatedCount int32
	// ChangesetSpecsCreatedCount is the number of changeset specs that have
	// been created by running `src campaign [preview|apply]`. This number
	// never goes down since it's based on event logs, even if the changeset
	// specs were not used and cleaned up.
	ChangesetSpecsCreatedCount int32

	// ActionChangesetsCount is the number of changesets published on code hosts by campaigns. This number
	// *could* go down, since it's not based on event logs, but so far
	// (Nov 2020) we never cleaned up changesets in the database.
	ActionChangesetsCount int32
	// ActionChangesetsDiffStatAddedSum is the total sum of lines added by
	// changesets published on the code host by campaigns.
	ActionChangesetsDiffStatAddedSum int32
	// ActionChangesetsDiffStatChangedSum is the total sum of lines changed by
	// changesets published on the code host by campaigns.
	ActionChangesetsDiffStatChangedSum int32
	// ActionChangesetsDiffStatDeletedSum is the total sum of lines deleted by
	// changesets published on the code host by campaigns.
	ActionChangesetsDiffStatDeletedSum int32

	// ActionChangesetsMergedCount is the number of changesets published on
	// code hosts by campaigns that have also been *merged*.
	// This number *could* go down, since it's not based on event logs, but
	// so far (Nov 2020) we never cleaned up changesets in the database.
	ActionChangesetsMergedCount int32
	// ActionChangesetsMergedDiffStatAddedSum is the total sum of lines added by
	// changesets published on the code host by campaigns and merged.
	ActionChangesetsMergedDiffStatAddedSum int32
	// ActionChangesetsMergedDiffStatChangedSum is the total sum of lines changed by
	// changesets published on the code host by campaigns and merged.
	ActionChangesetsMergedDiffStatChangedSum int32
	// ActionChangesetsMergedDiffStatDeletedSum is the total sum of lines deleted by
	// changesets published on the code host by campaigns and merged.
	ActionChangesetsMergedDiffStatDeletedSum int32

	// ManualChangesetsCount is the total number of changesets that have been
	// imported by a campaign to be tracked.
	// This number *could* go down, since it's not based on event logs, but
	// so far (Nov 2020) we never cleaned up changesets in the database.
	ManualChangesetsCount int32
	// ManualChangesetsCount is the total number of *merged* changesets that
	// have been imported by a campaign to be tracked.
	// This number *could* go down, since it's not based on event logs, but
	// so far (Nov 2020) we never cleaned up changesets in the database.
	ManualChangesetsMergedCount int32
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type SearchUsageStatistics struct {
	Daily   []*SearchUsagePeriod
	Weekly  []*SearchUsagePeriod
	Monthly []*SearchUsagePeriod
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type SearchUsagePeriod struct {
	StartTime          time.Time
	TotalUsers         int32
	Literal            *SearchEventStatistics
	Regexp             *SearchEventStatistics
	After              *SearchCountStatistics
	Archived           *SearchCountStatistics
	Author             *SearchCountStatistics
	Before             *SearchCountStatistics
	Case               *SearchCountStatistics
	Commit             *SearchEventStatistics
	Committer          *SearchCountStatistics
	Content            *SearchCountStatistics
	Count              *SearchCountStatistics
	Diff               *SearchEventStatistics
	File               *SearchEventStatistics
	Fork               *SearchCountStatistics
	Index              *SearchCountStatistics
	Lang               *SearchCountStatistics
	Message            *SearchCountStatistics
	PatternType        *SearchCountStatistics
	Repo               *SearchEventStatistics
	Repohascommitafter *SearchCountStatistics
	Repohasfile        *SearchCountStatistics
	Repogroup          *SearchCountStatistics
	Structural         *SearchEventStatistics
	Symbol             *SearchEventStatistics
	Timeout            *SearchCountStatistics
	Type               *SearchCountStatistics
	SearchModes        *SearchModeUsageStatistics
}

type SearchModeUsageStatistics struct {
	Interactive *SearchCountStatistics
	PlainText   *SearchCountStatistics
}

type SearchCountStatistics struct {
	UserCount   *int32
	EventsCount *int32
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type SearchEventStatistics struct {
	UserCount      *int32
	EventsCount    *int32
	EventLatencies *SearchEventLatencies
}

// NOTE: DO NOT alter this struct without making a symmetric change
// to the updatecheck handler. This struct is marshalled and sent to
// BigQuery, which requires the input match its schema exactly.
type SearchEventLatencies struct {
	P50 float64
	P90 float64
	P99 float64
}

// SiteUsageSummary is an alternate view of SiteUsageStatistics which is
// calculated in the database layer.
type SiteUsageSummary struct {
	Month                   time.Time
	Week                    time.Time
	Day                     time.Time
	UniquesMonth            int32
	UniquesWeek             int32
	UniquesDay              int32
	RegisteredUniquesMonth  int32
	RegisteredUniquesWeek   int32
	RegisteredUniquesDay    int32
	IntegrationUniquesMonth int32
	IntegrationUniquesWeek  int32
	IntegrationUniquesDay   int32
	ManageUniquesMonth      int32
	CodeUniquesMonth        int32
	VerifyUniquesMonth      int32
	MonitorUniquesMonth     int32
	ManageUniquesWeek       int32
	CodeUniquesWeek         int32
	VerifyUniquesWeek       int32
	MonitorUniquesWeek      int32
}

// AggregatedEvent represents the total events, unique users, and
// latencies over the current month, week, and day for a single event.
type AggregatedEvent struct {
	Name           string
	Month          time.Time
	Week           time.Time
	Day            time.Time
	TotalMonth     int32
	TotalWeek      int32
	TotalDay       int32
	UniquesMonth   int32
	UniquesWeek    int32
	UniquesDay     int32
	LatenciesMonth []float64
	LatenciesWeek  []float64
	LatenciesDay   []float64
}

type SurveyResponse struct {
	ID        int32
	UserID    *int32
	Email     *string
	Score     int32
	Reason    *string
	Better    *string
	CreatedAt time.Time
}

type Event struct {
	ID              int32
	Name            string
	URL             string
	UserID          *int32
	AnonymousUserID string
	Argument        string
	Source          string
	Version         string
	Timestamp       time.Time
}

// GrowthStatistics represents the total users that were created,
// deleted, resurrected, churned and retained over the current month.
type GrowthStatistics struct {
	DeletedUsers     int32
	CreatedUsers     int32
	ResurrectedUsers int32
	ChurnedUsers     int32
	RetainedUsers    int32
}

// SavedSearches represents the total number of saved searches, users
// using saved searches, and usage of saved searches.
type SavedSearches struct {
	TotalSavedSearches   int32
	UniqueUsers          int32
	NotificationsSent    int32
	NotificationsClicked int32
	UniqueUserPageViews  int32
	OrgSavedSearches     int32
}

// Panel homepage represents interaction data on the
// enterprise homepage panels.
type HomepagePanels struct {
	RecentFilesClickedPercentage           *float64
	RecentSearchClickedPercentage          *float64
	RecentRepositoriesClickedPercentage    *float64
	SavedSearchesClickedPercentage         *float64
	NewSavedSearchesClickedPercentage      *float64
	TotalPanelViews                        *float64
	UsersFilesClickedPercentage            *float64
	UsersSearchClickedPercentage           *float64
	UsersRepositoriesClickedPercentage     *float64
	UsersSavedSearchesClickedPercentage    *float64
	UsersNewSavedSearchesClickedPercentage *float64
	PercentUsersShown                      *float64
}

type WeeklyRetentionStats struct {
	WeekStart  time.Time
	CohortSize *int32
	Week0      *float64
	Week1      *float64
	Week2      *float64
	Week3      *float64
	Week4      *float64
	Week5      *float64
	Week6      *float64
	Week7      *float64
	Week8      *float64
	Week9      *float64
	Week10     *float64
	Week11     *float64
}

type RetentionStats struct {
	Weekly []*WeeklyRetentionStats
}

type SearchOnboarding struct {
	TotalOnboardingTourViews   *int32
	ViewedLangStep             *int32
	ViewedFilterRepoStep       *int32
	ViewedAddQueryTermStep     *int32
	ViewedSubmitSearchStep     *int32
	ViewedSearchReferenceStep  *int32
	CloseOnboardingTourClicked *int32
}

// Weekly usage statistics for the extensions platform
type ExtensionsUsageStatistics struct {
	WeekStart                  time.Time
	UsageStatisticsByExtension []*ExtensionUsageStatistics
	// Average number of non-default extensions used by users
	// that have used at least one non-default extension
	AverageNonDefaultExtensions *float64
	// The count of users that have activated a non-default extension this week
	NonDefaultExtensionUsers *int32
}

// Weekly statistics for an individual extension
type ExtensionUsageStatistics struct {
	// The count of users that have activated this extension
	UserCount *int32
	// The average number of activations for users that have
	// used this extension at least once
	AverageActivations *float64
	ExtensionID        *string
}

type CodeInsightsUsageStatistics struct {
	UsageStatisticsByInsight       []*InsightUsageStatistics
	InsightsPageViews              *int32
	InsightsUniquePageViews        *int32
	InsightConfigureClick          *int32
	InsightAddMoreClick            *int32
	WeekStart                      time.Time
	WeeklyInsightCreators          *int32
	WeeklyFirstTimeInsightCreators *int32
}

// Usage statistics for a type of code insight
type InsightUsageStatistics struct {
	InsightType      *string
	Additions        *int32
	Edits            *int32
	Removals         *int32
	Hovers           *int32
	UICustomizations *int32
	DataPointClicks  *int32
}

// Secret represents the secrets table
type Secret struct {
	ID int32

	// The table containing an object whose token is being encrypted.
	SourceType sql.NullString

	// The ID of the object in the SourceType table.
	SourceID sql.NullInt32

	// KeyName represents a unique key for the case where we're storing key-value pairs.
	KeyName sql.NullString

	// Value contains the encrypted string
	Value string
}

type SearchContext struct {
	ID int32
	// Name contains the non-prefixed part of the search context spec.
	// The name is a substring of the spec and it should NOT be used as the spec itself.
	// The spec contains additional information (such as the @ prefix and the context namespace)
	// that helps differentiate between different search contexts.
	// Example mappings from context spec to context name:
	// global -> global, @user -> user, @org -> org,
	// @user/ctx1 -> ctx1, @org/ctx2 -> ctx2.
	Name        string
	Description string
	UserID      int32 // if non-zero, the owner is this user. UserID/OrgID are mutually exclusive.
	OrgID       int32 // if non-zero, the owner is this organization. UserID/OrgID are mutually exclusive.
}
