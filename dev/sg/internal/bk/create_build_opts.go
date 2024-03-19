package bk

import "github.com/buildkite/go-buildkite/v3/buildkite"

type CreateBuildOpt func(*buildkite.CreateBuild)

func WithEnvVar(key, value string) CreateBuildOpt {
	return func(b *buildkite.CreateBuild) {
		if b.Env == nil {
			b.Env = map[string]string{}
		}

		b.Env[key] = value
	}
}

func WithEnv(env map[string]string) CreateBuildOpt {
	return func(b *buildkite.CreateBuild) {
		b.Env = env
	}
}
