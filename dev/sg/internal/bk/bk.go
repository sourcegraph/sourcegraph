package bk

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/buildkite/go-buildkite/v3/buildkite"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/secrets"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

// BuildkiteOrg is a Sourcegraph org in Buildkite. See: is https://buildkite.com/sourcegraph
const BuildkiteOrg = "sourcegraph"

type Build struct {
	buildkite.Build
}

// AnnotationArtifact contains the annotation artifact that was uploaded as part of a job step. The content of the artifact
// is stored in Content and is expected to be markdown.
type AnnotationArtifact struct {
	buildkite.Artifact
	Content string
}

// JobAnnotations maps Job IDs to a annotation artifacts
type JobAnnotations map[string]AnnotationArtifact

// retrieveToken obtains a token either from the cached configuration or by asking the user for it.
func retrieveToken(ctx context.Context, out *std.Output) (string, error) {
	if tok := os.Getenv("BUILDKITE_API_TOKEN"); tok != "" {
		// If the token is provided by the environment, use that one.
		return tok, nil
	}

	store, err := secrets.FromContext(ctx)
	if err != nil {
		return "", err
	}

	token, err := store.GetExternal(ctx, secrets.ExternalSecret{
		Project: "sourcegraph-local-dev",
		Name:    "SG_BUILDKITE_TOKEN",
	}, func(_ context.Context) (string, error) {
		return getTokenFromUser(out)
	})

	if err != nil {
		return "", err
	}
	return token, nil
}

// getTokenFromUser prompts the user for a slack OAuth token.
func getTokenFromUser(out *std.Output) (string, error) {
	out.WriteLine(output.Linef(output.EmojiLightbulb, output.StyleSuggestion, `Please create and copy a new token from %shttps://buildkite.com/user/api-access-tokens%s with the following scopes:

- Organization access to %q
- read_artifacts
- read_builds
- read_build_logs
- read_pipelines
- (optional) write_builds

To use functionality that manipulates builds, you must also have the 'write_builds' scope.
`, output.StyleOrange, output.StyleSuggestion, BuildkiteOrg))
	return out.PromptPasswordf(os.Stdin, "Paste your token here:")
}

type Client struct {
	bk *buildkite.Client
}

// NewClient returns an authenticated client that can perform various operation on
// the organization assigned to buildkiteOrg.
// If there is no token assigned yet, it will be asked to the user.
func NewClient(ctx context.Context, out *std.Output) (*Client, error) {
	token, err := retrieveToken(ctx, out)
	if err != nil {
		return nil, err
	}

	config, err := buildkite.NewTokenConfig(token, false)
	if err != nil {
		return nil, errors.Newf("failed to init buildkite config: %w", err)
	}
	return &Client{bk: buildkite.NewClient(config.Client())}, nil
}

// GetMostRecentBuild returns a list of most recent builds for the given pipeline and branch.
// If no builds are found, an error will be returned.
func (c *Client) GetMostRecentBuild(ctx context.Context, pipeline, branch string) (*buildkite.Build, error) {
	builds, _, err := c.bk.Builds.ListByPipeline(BuildkiteOrg, pipeline, &buildkite.BuildsListOptions{
		Branch: branch,
	})
	if err != nil {
		if strings.Contains(err.Error(), "404 Not Found") {
			return nil, errors.New("no build found")
		}
		return nil, err
	}
	if len(builds) == 0 {
		return nil, errors.New("no builds found")
	}

	// Newest is returned first https://buildkite.com/docs/apis/rest-api/builds#list-builds-for-a-pipeline
	return &builds[0], nil
}

// GetBuildByNumber returns a single build from a given pipeline and a given build number.
// If no build is found, an error will be returned.
func (c *Client) GetBuildByNumber(ctx context.Context, pipeline string, number string) (*buildkite.Build, error) {
	b, _, err := c.bk.Builds.Get(BuildkiteOrg, pipeline, number, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404 Not Found") {
			return nil, errors.New("no build found")
		}
		return nil, err
	}
	return b, nil
}

func (c *Client) GetBuildByCommit(ctx context.Context, pipeline string, commit string) (*buildkite.Build, error) {
	b, _, err := c.bk.Builds.ListByPipeline(BuildkiteOrg, pipeline, &buildkite.BuildsListOptions{
		Commit: commit,
	})
	if err != nil {
		if strings.Contains(err.Error(), "404 Not Found") {
			return nil, errors.New("no build found")
		}
		return nil, err
	}
	if len(b) == 0 {
		return nil, errors.New("no build found")
	}
	// Newest is returned first https://buildkite.com/docs/apis/rest-api/builds#list-builds-for-a-pipeline
	return &b[0], nil
}

// ListArtifactsByBuildNumber queries the Buildkite API and retrieves all the artifacts for a particular build
func (c *Client) ListArtifactsByBuildNumber(ctx context.Context, pipeline string, number string) ([]buildkite.Artifact, error) {
	artifacts, _, err := c.bk.Artifacts.ListByBuild(BuildkiteOrg, pipeline, number, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404 Not Found") {
			return nil, errors.New("no artifacts because no build found")
		}
		return nil, err
	}

	return artifacts, nil
}

// ListArtifactsByJob queries the Buildkite API and retrieves all the artifacts for a particular job
func (c *Client) ListArtifactsByJob(ctx context.Context, pipeline string, buildNumber string, jobID string) ([]buildkite.Artifact, error) {
	artifacts, _, err := c.bk.Artifacts.ListByJob(BuildkiteOrg, pipeline, buildNumber, jobID, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404 Not Found") {
			return nil, errors.New("no artifacts because no build or job found")
		}
		return nil, err
	}

	return artifacts, nil
}

// DownloadArtifact downloads the Buildkite artifact into the provider io.Writer
func (c *Client) DownloadArtifact(artifact buildkite.Artifact, w io.Writer) error {
	url := artifact.DownloadURL
	if url == nil {
		return errors.New("unable to download artifact, nil download url")
	}
	_, err := c.bk.Artifacts.DownloadArtifactByURL(*url, w)
	if err != nil {
		return err
	}
	return nil
}

// GetJobAnnotationByBuildNumber retrieves all annotations that are present on a build and maps them to the job ID that the
// annotation is for. Each annotation is retrieved by looking at all the artifacts on a build. If a Job has a annoation, then
// an artifact will be uploaded by the job. The annotation artifact's name will have the following format "annoations/{BUILDKITE_JOB_ID}-annotation.md"
func (c *Client) GetJobAnnotationsByBuildNumber(ctx context.Context, pipeline string, number string) (JobAnnotations, error) {
	artifacts, err := c.ListArtifactsByBuildNumber(ctx, pipeline, number)
	if err != nil {
		return nil, err
	}

	result := make(JobAnnotations, 0)
	for _, a := range artifacts {
		if strings.Contains(*a.Dirname, "annotations") && strings.HasSuffix(*a.Filename, "-annotation.md") {
			var buf bytes.Buffer
			_, err := c.bk.Artifacts.DownloadArtifactByURL(*a.DownloadURL, &buf)
			if err != nil {
				return nil, errors.Newf("failed to download artifact %q at %s: %w", *a.Filename, *a.DownloadURL, err)
			}

			result[*a.JobID] = AnnotationArtifact{
				Artifact: a,
				Content:  strings.TrimSpace(buf.String()),
			}
		}
	}

	return result, nil
}

// TriggerBuild request a build on Buildkite API and returns that build.
func (c *Client) TriggerBuild(ctx context.Context, pipeline, branch, commit string, opts ...CreateBuildOpt) (*buildkite.Build, error) {
	newBuild := &buildkite.CreateBuild{
		Branch: branch,
		Commit: commit,
	}

	for _, opt := range opts {
		opt(newBuild)
	}
	build, _, err := c.bk.Builds.Create(BuildkiteOrg, pipeline, newBuild)

	return build, err
}

type ExportLogsOpts struct {
	JobStepKey string
	JobQuery   string
	State      string
}

type JobLogs struct {
	JobMeta JobMeta

	Content *string
}

// Used as labels to identify a log stream
type JobMeta struct {
	Build int    `json:"build"`
	Job   string `json:"job"`

	Name    *string `json:"name,omitempty"`
	Label   *string `json:"label,omitempty"`
	StepKey *string `json:"step_key,omitempty"`
	Command *string `json:"command,omitempty"`
	Type    *string `json:"type,omitempty"`

	State        *string    `json:"state,omitempty"`
	ExitStatus   *int       `json:"exit_status,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	RetriesCount int        `json:"retries_count"`
}

func maybeTime(ts *buildkite.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	return &ts.Time
}

func newJobMeta(build int, j *buildkite.Job) JobMeta {
	return JobMeta{
		Build: build,
		Job:   *j.ID,

		Name:    j.Name,
		Label:   j.Label,
		StepKey: j.StepKey,
		Command: j.Command,
		Type:    j.Type,

		State:        j.State,
		ExitStatus:   j.ExitStatus,
		StartedAt:    maybeTime(j.StartedAt),
		FinishedAt:   maybeTime(j.FinishedAt),
		RetriesCount: j.RetriesCount,
	}
}

func hasState(job *buildkite.Job, state string) bool {
	if state == "" {
		return true
	}
	return job.State != nil && *job.State == state
}

func (c *Client) ExportLogs(ctx context.Context, pipeline string, build int, opts ExportLogsOpts) ([]*JobLogs, error) {
	buildID := strconv.Itoa(build)
	buildDetails, _, err := c.bk.Builds.Get(BuildkiteOrg, pipeline, buildID, nil)
	if err != nil {
		return nil, err
	}

	if opts.JobStepKey != "" {
		var job *buildkite.Job
		for _, j := range buildDetails.Jobs {
			if j.StepKey != nil && *j.StepKey == opts.JobStepKey {
				job = j
				break
			}
		}
		if job == nil {
			return nil, errors.Newf("no job matching stepkey %q found in build %d", opts.JobStepKey, build)
		}

		l, _, err := c.bk.Jobs.GetJobLog(BuildkiteOrg, pipeline, buildID, *job.ID)
		if err != nil {
			return nil, err
		}
		return []*JobLogs{{
			JobMeta: newJobMeta(build, job),
			Content: l.Content,
		}}, nil
	}

	if opts.JobQuery != "" {
		var job *buildkite.Job
		for _, j := range buildDetails.Jobs {
			idMatch := j.ID != nil && *j.ID == opts.JobQuery
			nameMatch := j.Name != nil && strings.Contains(strings.ToLower(*j.Name), strings.ToLower(opts.JobQuery))
			if idMatch || nameMatch {
				job = j
				break
			}
		}
		if job == nil {
			return nil, errors.Newf("no job matching query %q found in build %d", opts.JobQuery, build)
		}
		if !hasState(job, opts.State) {
			return []*JobLogs{}, nil
		}

		l, _, err := c.bk.Jobs.GetJobLog(BuildkiteOrg, pipeline, buildID, *job.ID)
		if err != nil {
			return nil, err
		}
		return []*JobLogs{{
			JobMeta: newJobMeta(build, job),
			Content: l.Content,
		}}, nil
	}

	logs := []*JobLogs{}
	for _, job := range buildDetails.Jobs {
		if !hasState(job, opts.State) {
			continue
		}

		if opts.State == "failed" && job.SoftFailed {
			// Soft fails are not a state, but an attribute of failed jobs.
			// Ignore them, so we don't count them as failures.
			continue
		}

		l, _, err := c.bk.Jobs.GetJobLog(BuildkiteOrg, pipeline, buildID, *job.ID)
		if err != nil {
			return nil, err
		}
		logs = append(logs, &JobLogs{
			JobMeta: newJobMeta(build, job),
			Content: l.Content,
		})
	}

	return logs, nil
}
