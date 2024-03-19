package main

import (
	"flag"
	"fmt"
	"testing"

	"github.com/buildkite/go-buildkite/v3/buildkite"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/dev/build-tracker/build"
	"github.com/sourcegraph/sourcegraph/dev/build-tracker/config"
	"github.com/sourcegraph/sourcegraph/dev/build-tracker/notify"
	"github.com/sourcegraph/sourcegraph/dev/team"
)

var RunSlackIntegrationTest = flag.Bool("RunSlackIntegrationTest", false, "Run Slack integration tests")
var RunGitHubIntegrationTest = flag.Bool("RunGitHubIntegrationTest", false, "Run Github integration tests")

type TestJobLine struct {
	title string
	url   string
}

func (l *TestJobLine) Title() string {
	return l.title
}

func (l *TestJobLine) LogURL() string {
	return l.url
}

func newJob(t *testing.T, name string, exit int) *build.Job {
	t.Helper()

	state := build.JobFailedState
	if exit == 0 {
		state = build.JobPassedState
	}
	return &build.Job{
		Job: buildkite.Job{
			Name:       &name,
			ExitStatus: &exit,
			State:      &state,
		},
	}
}

func TestLargeAmountOfFailures(t *testing.T) {
	num := 160000
	commit := "ca7c44f79984ff8d645b580bfaaf08ce9a37a05d"
	url := "http://www.google.com"
	pipelineID := "sourcegraph"
	msg := "Large amount of failures test"
	info := &notify.BuildNotification{
		BuildNumber:        num,
		ConsecutiveFailure: 0,
		PipelineName:       pipelineID,
		AuthorEmail:        "william.bezuidenhout@sourcegraph.com",
		Message:            msg,
		Commit:             commit,
		BuildURL:           url,
		BuildStatus:        "Failed",
		Fixed:              []notify.JobLine{},
		Failed:             []notify.JobLine{},
	}
	for i := 1; i <= 30; i++ {
		info.Failed = append(info.Failed, &TestJobLine{
			title: fmt.Sprintf("Job %d", i),
			url:   "http://example.com",
		})
	}

	flag.Parse()
	if !*RunSlackIntegrationTest {
		t.Skip("Slack Integration test not enabled")
	}
	logger := logtest.NoOp(t)

	conf, err := config.NewFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	client := notify.NewClient(logger, conf.SlackToken, conf.GithubToken, config.DefaultChannel)

	err = client.Send(info)
	if err != nil {
		t.Fatalf("failed to send build: %s", err)
	}
}

func TestSlackMention(t *testing.T) {
	t.Run("If SlackID is empty, ask people to update their team.yml", func(t *testing.T) {
		result := notify.SlackMention(&team.Teammate{
			SlackID: "",
			Name:    "Bob Burgers",
			Email:   "bob@burgers.com",
			GitHub:  "bobbyb",
		})

		require.Equal(t, "Bob Burgers (bob@burgers.com) - We could not locate your Slack ID. Please check that your information in the Handbook team.yml file is correct", result)
	})
	t.Run("Use SlackID if it exists", func(t *testing.T) {
		result := notify.SlackMention(&team.Teammate{
			SlackID: "USE_ME",
			Name:    "Bob Burgers",
			Email:   "bob@burgers.com",
			GitHub:  "bobbyb",
		})
		require.Equal(t, "<@USE_ME>", result)
	})
}

func TestGetTeammateFromBuild(t *testing.T) {
	flag.Parse()
	if !*RunGitHubIntegrationTest {
		t.Skip("Github Integration test not enabled")
	}

	logger := logtest.NoOp(t)
	conf, err := config.NewFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	conf.SlackChannel = config.DefaultChannel

	t.Run("with nil author, commit author is still retrieved", func(t *testing.T) {
		client := notify.NewClient(logger, conf.SlackToken, conf.GithubToken, conf.SlackChannel)

		num := 160000
		commit := "ca7c44f79984ff8d645b580bfaaf08ce9a37a05d"
		pipelineID := "sourcegraph"
		build := &build.Build{
			Build: buildkite.Build{
				Pipeline: &buildkite.Pipeline{
					ID:   &pipelineID,
					Name: &pipelineID,
				},
				Number: &num,
				Commit: &commit,
			},
			Pipeline: &build.Pipeline{buildkite.Pipeline{
				Name: &pipelineID,
			}},
			Steps: map[string]*build.Step{},
		}

		teammate, err := client.GetTeammateForCommit(build.GetCommit())
		require.NoError(t, err)
		require.NotEqual(t, teammate.SlackID, "")
		require.Equal(t, teammate.Name, "Leo Papaloizos")
	})
	t.Run("commit author preferred over build author", func(t *testing.T) {
		client := notify.NewClient(logger, conf.SlackToken, conf.GithubToken, conf.SlackChannel)

		num := 160000
		commit := "78926a5b3b836a8a104a5d5adf891e5626b1e405"
		pipelineID := "sourcegraph"
		build := &build.Build{
			Build: buildkite.Build{
				Pipeline: &buildkite.Pipeline{
					ID:   &pipelineID,
					Name: &pipelineID,
				},
				Number: &num,
				Commit: &commit,
				Author: &buildkite.Author{
					Name:  "William Bezuidenhout",
					Email: "william.bezuidenhout@sourcegraph.com",
				},
			},
			Pipeline: &build.Pipeline{buildkite.Pipeline{
				Name: &pipelineID,
			}},
			Steps: map[string]*build.Step{},
		}

		teammate, err := client.GetTeammateForCommit(build.GetCommit())
		require.NoError(t, err)
		require.Equal(t, teammate.Name, "Ryan Slade")
	})
	t.Run("retrieving teammate for build populates cache", func(t *testing.T) {
		client := notify.NewClient(logger, conf.SlackToken, conf.GithubToken, conf.SlackChannel)

		num := 160000
		commit := "78926a5b3b836a8a104a5d5adf891e5626b1e405"
		pipelineID := "sourcegraph"
		build := &build.Build{
			Build: buildkite.Build{
				Pipeline: &buildkite.Pipeline{
					ID:   &pipelineID,
					Name: &pipelineID,
				},
				Number: &num,
				Commit: &commit,
				Author: &buildkite.Author{
					Name:  "William Bezuidenhout",
					Email: "william.bezuidenhout@sourcegraph.com",
				},
			},
			Pipeline: &build.Pipeline{buildkite.Pipeline{
				Name: &pipelineID,
			}},
			Steps: map[string]*build.Step{},
		}

		teammate, err := client.GetTeammateForCommit(build.GetCommit())
		require.NoError(t, err)
		require.NotNil(t, teammate)
	})
}

func TestSlackNotification(t *testing.T) {
	flag.Parse()
	if !*RunSlackIntegrationTest {
		t.Skip("Slack Integration test not enabled")
	}
	logger := logtest.NoOp(t)

	conf, err := config.NewFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	client := notify.NewClient(logger, conf.SlackToken, conf.GithubToken, config.DefaultChannel)

	// Each child test needs to increment this number, otherwise notifications will be overwritten
	buildNumber := 160000
	url := "http://www.google.com"
	commit := "78926a5b3b836a8a104a5d5adf891e5626b1e405"
	pipelineID := "sourcegraph"
	exit := 999
	msg := "this is a test"
	b := &build.Build{
		Build: buildkite.Build{
			Message: &msg,
			WebURL:  &url,
			Creator: &buildkite.Creator{
				AvatarURL: "https://www.gravatar.com/avatar/7d4f6781b10e48a94d1052c443d13149",
			},
			Pipeline: &buildkite.Pipeline{
				ID:   &pipelineID,
				Name: &pipelineID,
			},
			Author: &buildkite.Author{
				Name:  "William Bezuidenhout",
				Email: "william.bezuidenhout@sourcegraph.com",
			},
			Number: &buildNumber,
			URL:    &url,
			Commit: &commit,
		},
		Pipeline: &build.Pipeline{buildkite.Pipeline{
			Name: &pipelineID,
		}},
	}
	t.Run("send new notification", func(t *testing.T) {
		b.Steps = map[string]*build.Step{
			":one: fake step":   build.NewStepFromJob(newJob(t, ":one: fake step", exit)),
			":two: fake step":   build.NewStepFromJob(newJob(t, ":two: fake step", exit)),
			":three: fake step": build.NewStepFromJob(newJob(t, ":three: fake step", exit)),
			":four: fake step":  build.NewStepFromJob(newJob(t, ":four: fake step", exit)),
		}

		info := determineBuildStatusNotification(logtest.NoOp(t), b)
		err := client.Send(info)
		if err != nil {
			t.Fatalf("failed to send slack notification: %v", err)
		}

		notification := client.GetNotification(b.GetNumber())
		if notification == nil {
			t.Fatalf("expected not nil notificaiton after new notification")
		}
		if notification.ID == "" {
			t.Error("expected notification id to not be empty")
		}
		if notification.ChannelID == "" {
			t.Error("expected notification channel id to not be empty")
		}
	})
	t.Run("update notification", func(t *testing.T) {
		// setup the build
		msg := "notification gets updated"
		b.Message = &msg
		buildNumber++
		b.Number = &buildNumber
		b.Steps = map[string]*build.Step{
			":one: fake step": build.NewStepFromJob(newJob(t, ":one: fake step", exit)),
		}

		// post a new notification
		info := determineBuildStatusNotification(logtest.NoOp(t), b)
		err := client.Send(info)
		if err != nil {
			t.Fatalf("failed to send slack notification: %v", err)
		}
		newNotification := client.GetNotification(b.GetNumber())
		if newNotification == nil {
			t.Errorf("expected not nil notification after new message")
		}
		// now update the notification with additional jobs that failed
		b.AddJob(newJob(t, ":alarm_clock: delayed job", exit))
		info = determineBuildStatusNotification(logtest.NoOp(t), b)
		err = client.Send(info)
		if err != nil {
			t.Fatalf("failed to send slack notification: %v", err)
		}
		updatedNotification := client.GetNotification(b.GetNumber())
		if updatedNotification == nil {
			t.Errorf("expected not nil notification after updated message")
		}
		if newNotification.Equals(updatedNotification) {
			t.Errorf("expected new and updated notifications to differ - new '%v' updated '%v'", newNotification, updatedNotification)
		}
	})
	t.Run("send 3 notifications with more and more failures", func(t *testing.T) {
		// setup the build
		msg := "3 notifications with more and more failures"
		b.Message = &msg
		buildNumber++
		b.Number = &buildNumber
		b.Steps = map[string]*build.Step{
			":one: fake step": build.NewStepFromJob(newJob(t, ":one: fake step", exit)),
		}

		// post a new notification
		info := determineBuildStatusNotification(logtest.NoOp(t), b)
		err := client.Send(info)
		if err != nil {
			t.Fatalf("failed to send slack notification: %v", err)
		}
		newNotification := client.GetNotification(b.GetNumber())
		if newNotification == nil {
			t.Errorf("expected not nil notification after new message")
		}

		b.AddJob(newJob(t, ":alarm: outlier", 1))
		info = determineBuildStatusNotification(logtest.NoOp(t), b)
		err = client.Send(info)
		if err != nil {
			t.Fatalf("failed to send slack notification: %v", err)
		}

		// now add a bunch
		for i := range 5 {
			b.AddJob(newJob(t, fmt.Sprintf(":alarm_clock: delayed job %d", i), exit))
		}
		info = determineBuildStatusNotification(logtest.NoOp(t), b)
		err = client.Send(info)
		if err != nil {
			t.Fatalf("failed to send slack notification: %v", err)
		}
	})
	t.Run("send a failed build that gets fixed later", func(t *testing.T) {
		// setup the build
		msg := "failed then fixed later"
		b.Message = &msg
		buildNumber++
		b.Number = &buildNumber
		b.Steps = map[string]*build.Step{
			":one: fake step":   build.NewStepFromJob(newJob(t, ":one: fake step", exit)),
			":two: fake step":   build.NewStepFromJob(newJob(t, ":two: fake step", exit)),
			":three: fake step": build.NewStepFromJob(newJob(t, ":three: fake step", exit)),
		}

		// post a new notification
		info := determineBuildStatusNotification(logtest.NoOp(t), b)
		err := client.Send(info)
		if err != nil {
			t.Fatalf("failed to send slack notification: %v", err)
		}
		newNotification := client.GetNotification(b.GetNumber())
		if newNotification == nil {
			t.Errorf("expected not nil notification after new message")
		}

		// now fix all the Steps by adding a passed job
		for _, s := range b.Steps {
			b.AddJob(newJob(t, s.Name, 0))
		}
		info = determineBuildStatusNotification(logtest.NoOp(t), b)
		if info.BuildStatus != string(build.BuildFixed) {
			t.Errorf("all jobs are fixed, build status should be fixed")
		}
		err = client.Send(info)
		if err != nil {
			t.Fatalf("failed to send slack notification: %v", err)
		}
	})
	t.Run("send a failed build that gets fixed later", func(t *testing.T) {
		// setup the build
		msg := "mixed of failed and fixed jobs"
		b.Message = &msg
		buildNumber++
		b.Number = &buildNumber
		b.Steps = map[string]*build.Step{
			":one: fake step":   build.NewStepFromJob(newJob(t, ":one: fake step", exit)),
			":two: fake step":   build.NewStepFromJob(newJob(t, ":two: fake step", exit)),
			":three: fake step": build.NewStepFromJob(newJob(t, ":three: fake step", exit)),
			":four: fake step":  build.NewStepFromJob(newJob(t, ":four: fake step", exit)),
			":five: fake step":  build.NewStepFromJob(newJob(t, ":five: fake step", exit)),
			":six: fake step":   build.NewStepFromJob(newJob(t, ":six: fake step", exit)),
		}

		// post a new notification
		info := determineBuildStatusNotification(logtest.NoOp(t), b)
		err := client.Send(info)
		if err != nil {
			t.Fatalf("failed to send slack notification: %v", err)
		}
		newNotification := client.GetNotification(b.GetNumber())
		if newNotification == nil {
			t.Errorf("expected not nil notification after new message")
		}

		// now fix half the Steps by adding a passed job
		count := 0
		for _, s := range b.Steps {
			if count < 3 {
				b.AddJob(newJob(t, s.Name, 0))
			}
			count++
		}
		info = determineBuildStatusNotification(logtest.NoOp(t), b)
		if info.BuildStatus != string(build.BuildFailed) {
			t.Errorf("some jobs are still failed so overall build status should be Failed")
		}
		err = client.Send(info)
		if err != nil {
			t.Fatalf("failed to send slack notification: %v", err)
		}
	})
}

func TestServerNotify(t *testing.T) {
	flag.Parse()
	if !*RunSlackIntegrationTest {
		t.Skip("Slack Integration test not enabled")
	}
	logger := logtest.NoOp(t)

	conf, err := config.NewFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	server := NewServer(logger, *conf)

	num := 160000
	url := "http://www.google.com"
	commit := "78926a5b3b836a8a104a5d5adf891e5626b1e405"
	pipelineID := "sourcegraph"
	exit := 999
	msg := "this is a test"
	build := &build.Build{
		Build: buildkite.Build{
			Message: &msg,
			WebURL:  &url,
			Creator: &buildkite.Creator{
				AvatarURL: "https://www.gravatar.com/avatar/7d4f6781b10e48a94d1052c443d13149",
			},
			Pipeline: &buildkite.Pipeline{
				ID:   &pipelineID,
				Name: &pipelineID,
			},
			Author: &buildkite.Author{
				Name:  "William Bezuidenhout",
				Email: "william.bezuidenhout@sourcegraph.com",
			},
			Number: &num,
			URL:    &url,
			Commit: &commit,
		},
		Pipeline: &build.Pipeline{buildkite.Pipeline{
			Name: &pipelineID,
		}},
		Steps: map[string]*build.Step{
			":one: fake step": build.NewStepFromJob(newJob(t, ":one: fake step", exit)),
		},
	}

	// post a new notification
	err = server.notifyIfFailed(build)
	if err != nil {
		t.Fatalf("failed to send slack notification: %v", err)
	}
}
