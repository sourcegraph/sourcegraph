package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/dev/scaletesting/internal/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

var app = &cli.App{
	Usage:       "Edit repository settings in bulk",
	Description: "https://handbook.sourcegraph.com/departments/engineering/dev/tools/scaletesting/",
	Compiled:    time.Now(),
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "github.token",
			Usage:    "GitHub token",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "github.org",
			Usage:    "Organization holding the repositories that are to be edited",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "github.url",
			Usage: "Base URL to the GitHub instance",
			Value: "https://github.com",
		},
		&cli.StringFlag{
			Name:  "state",
			Usage: "Path to a database file to store the state (will be created if doesn't exist)",
			Value: "bulkreposettings.db",
		},
		&cli.IntFlag{
			Name:  "retry",
			Usage: "Max retry count",
			Value: 3,
		},
	},
	Commands: []*cli.Command{
		{
			Name:        "visibility",
			Description: "change visibility of repositories",
			Subcommands: []*cli.Command{
				{
					Name:        "private",
					Description: "Set repo visibility to private",
					Action: func(cmd *cli.Context) error {
						logger := log.Scoped("runner")
						ctx := context.Background()
						tc := oauth2.NewClient(ctx, oauth2.StaticTokenSource(
							&oauth2.Token{AccessToken: cmd.String("github.token")},
						))
						baseURL, err := url.Parse(cmd.String("github.url"))
						if err != nil {
							return err
						}
						baseURL.Path = "/api/v3"
						gh, err := github.NewClient(tc).WithEnterpriseURLs(baseURL.String(), baseURL.String())
						if err != nil {
							logger.Fatal("failed to sign-in to GitHub", log.Error(err))
						}

						org := cmd.String("github.org")

						s, err := store.New(cmd.String("state"))
						if err != nil {
							logger.Fatal("failed to init state", log.Error(err))
						}

						repos, err := s.Load()
						if err != nil {
							logger.Error("failed to open state database", log.Error(err))
							return err
						}

						var repoIter Iter[[]*store.Repo]
						var total int64
						if len(repos) == 0 {
							logger.Info("Using GithubRepoFetcher")
							repoIter = &GithubRepoFetcher{
								client:   gh,
								repoType: "public", // we're only interested in public repos to change visibility
								org:      org,
								page:     0,
								done:     false,
								err:      nil,
							}

							t, err := getTotalPublicRepos(ctx, gh, org)
							if err != nil {
								logger.Fatal("failed to get total public repos size for org", log.String("org", org), log.Error(err))
							}
							logger.Info("Estimated public repos from API", log.Int("total", t))
							total = int64(t)
						} else {
							logger.Info("Using StaticRepoFecther")
							repoIter = &MockRepoFetcher{
								repos:    repos,
								iterSize: 10,
								start:    0,
							}
							total = int64(len(repos))
						}

						out := output.NewOutput(os.Stdout, output.OutputOpts{})
						pending := out.Pending(output.Line(output.EmojiHourglass, output.StylePending, "Updating repos"))
						defer pending.Destroy()

						var done int64

						p := pool.NewWithResults[error]().WithMaxGoroutines(20)
						for !repoIter.Done() && repoIter.Err() == nil {
							for _, r := range repoIter.Next(ctx) {
								r := r
								if err := s.SaveRepo(r); err != nil {
									logger.Fatal("could not save repo", log.Error(err), log.String("repo", r.Name))
								}

								p.Go(func() error {
									if r.Pushed {
										return nil
									}
									var err error
									settings := &github.Repository{Private: github.Bool(true)}
									for range cmd.Int("retry") {
										_, _, err = gh.Repositories.Edit(cmd.Context, org, r.Name, settings)
										if err != nil {
											r.Failed = err.Error()
										} else {
											r.Failed = ""
											r.Pushed = true
											break
										}
									}

									if err := s.SaveRepo(r); err != nil {
										logger.Fatal("could not save repo", log.Error(err), log.String("repo", r.Name))
									}
									atomic.AddInt64(&done, 1)
									pending.Update(fmt.Sprintf("%d repos updated (estimated total: %d)", done, total))
									return err
								})
							}
							// The total we get from Github is not correct (ie. 50k when we know the org as 200k)
							// So when done reaches the total, we attempt to get the total again and double the Max
							// of the bar
							if atomic.LoadInt64(&done) == total {
								t, err := getTotalPublicRepos(ctx, gh, org)
								if err != nil {
									logger.Fatal("failed to get updated public repos count", log.Error(err))
								}
								atomic.AddInt64(&total, int64(t))
								pending.Update(fmt.Sprintf("%d repos updated (estimated total: %d)", done, total))
							}
						}

						if err := repoIter.Err(); err != nil {
							logger.Error("repo iterator encountered an error", log.Error(err))
						}

						results := p.Wait()

						// Check that we actually got errors
						errs := []error{}
						for _, r := range results {
							if r != nil {
								errs = append(errs, r)
							}
						}

						if len(errs) > 0 {
							pending.Complete(output.Line(output.EmojiFailure, output.StyleBold, fmt.Sprintf("%d errors occured while updating repos", len(errs))))
							out.Writef("Printing first 5 errros")
							for i := range min(len(errs), 5) {
								logger.Error("Error updating repo", log.Error(errs[i]))
							}
							return errs[0]
						}
						pending.Complete(output.Line(output.EmojiOk, output.StyleBold, fmt.Sprintf("%d repos updated", done)))
						return nil
					},
				},
			},
		},
	},
}

type Iter[T any] interface {
	Err() error
	Next(ctx context.Context) T
	Done() bool
}

var _ Iter[[]*store.Repo] = (*GithubRepoFetcher)(nil)

// StaticRepoFetcher satisfies the Iter interface allowing one to iterate over a static array of repos. To change
// how many repos are returned per invocation of next, set iterSize (default 10). To start iterating at a different
// index, set start to a different value.
//
// The iteration is considered done when start >= len(repos)
type MockRepoFetcher struct {
	repos    []*store.Repo
	iterSize int
	start    int
}

// Err returns the last error (if any) encountered by Iter. For MockRepoFetcher, this retuns nil always
func (m *MockRepoFetcher) Err() error {
	return nil
}

// Done determines whether this Iter can produce more items. When start >= length of repos, then this will return true
func (m *MockRepoFetcher) Done() bool {
	return m.start >= len(m.repos)
}

// Next returns the next set of Repos. The amount of repos returned is determined by iterSize. When Done() is true,
// nil is returned.
func (m *MockRepoFetcher) Next(_ context.Context) []*store.Repo {
	if m.iterSize == 0 {
		m.iterSize = 10
	}
	if m.Done() {
		return nil
	}
	if m.start+m.iterSize > len(m.repos) {
		results := m.repos[m.start:]
		m.start = len(m.repos)
		return results
	}

	results := m.repos[m.start : m.start+m.iterSize]
	// advance the start index
	m.start += m.iterSize
	return results
}

type GithubRepoFetcher struct {
	client   *github.Client
	repoType string
	org      string
	page     int
	perPage  int
	done     bool
	err      error
}

// Done determines whether more repos can be retrieved from Github.
func (g *GithubRepoFetcher) Done() bool {
	return g.done
}

// Err returns the last error encountered by Iter
func (g *GithubRepoFetcher) Err() error {
	return g.err
}

// Next retrieves the next set of repos by contact Github. The amount of repos fetched is determined by pageSize.
// The next page start is automatically advanced based on the response received from Github. When the next page response
// from Github is 0, it means there are no more repos to fetch and this Iter is done, thus done is then set to true and
// Done() will also return true.
//
// If any error is encountered during retrieval of Repos the err value will be set and can be retrieved with Err()
func (g *GithubRepoFetcher) Next(ctx context.Context) []*store.Repo {
	if g.done {
		return nil
	}

	results, next, err := g.listRepos(ctx, g.org, g.page, g.perPage)
	if err != nil {
		g.err = err
		return nil
	}

	// when next is 0, it means the Github api returned the nextPage as 0, which indicates that there are not more pages to fetch
	if next > 0 {
		// Ensure that the next request starts at the next page
		g.page = next
	} else {
		g.done = true
	}

	return results
}

func (g *GithubRepoFetcher) listRepos(ctx context.Context, org string, start int, size int) ([]*store.Repo, int, error) {
	opts := github.RepositoryListByOrgOptions{
		Type:        g.repoType,
		ListOptions: github.ListOptions{Page: start, PerPage: size},
	}

	repos, resp, err := g.client.Repositories.ListByOrg(ctx, org, &opts)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode >= 300 {
		return nil, 0, errors.Newf("failed to list repos for org %s. Got status %d code", org, resp.StatusCode)
	}

	res := make([]*store.Repo, 0, len(repos))
	for _, repo := range repos {
		res = append(res, &store.Repo{
			Name:   repo.GetName(),
			GitURL: repo.GetGitURL(),
		})
	}

	next := resp.NextPage
	// If next page is 0 we're at the last page, so set the last page
	if next == 0 && g.page != resp.LastPage {
		next = resp.LastPage
	}

	return res, next, nil
}

func getTotalPublicRepos(ctx context.Context, client *github.Client, org string) (int, error) {
	orgRes, resp, err := client.Organizations.Get(ctx, org)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode >= 300 {
		return 0, errors.Newf("failed to get org %s. Got status %d code", org, resp.StatusCode)
	}

	return *orgRes.PublicRepos, nil
}

func main() {
	cb := log.Init(log.Resource{
		Name: "codehostcopy",
	})
	defer cb.Sync()
	logger := log.Scoped("main")

	if err := app.RunContext(context.Background(), os.Args); err != nil {
		logger.Fatal("failed to run", log.Error(err))
	}
}
