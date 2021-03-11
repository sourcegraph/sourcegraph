#!/usr/bin/env bash

set -e

# Use .bin outside of schema since schema dir is watched by watchman.
export GOBIN="$PWD/../.bin"
export GO111MODULE=on

go install github.com/sourcegraph/go-jsonschema/cmd/go-jsonschema-compiler
go build -o "$GOBIN"/stringdata stringdata.go

# shellcheck disable=SC2010
schemas="$(ls -- *.schema.json | grep -v json-schema-draft)"

# shellcheck disable=SC2086
"$GOBIN"/go-jsonschema-compiler -o schema.go -pkg schema $schemas

stringdata() {
  # shellcheck disable=SC2039
  target="${1/.schema.json/_stringdata.go}"
  "$GOBIN"/stringdata -i "$1" -name "$2" -pkg schema -o "$target"
}

stringdata aws_codecommit.schema.json AWSCodeCommitSchemaJSON
stringdata batch_spec.schema.json BatchSpecSchemaJSON
stringdata bitbucket_cloud.schema.json BitbucketCloudSchemaJSON
stringdata bitbucket_server.schema.json BitbucketServerSchemaJSON
stringdata changeset_spec.schema.json ChangesetSpecSchemaJSON
stringdata github.schema.json GitHubSchemaJSON
stringdata gitlab.schema.json GitLabSchemaJSON
stringdata gitolite.schema.json GitoliteSchemaJSON
stringdata other_external_service.schema.json OtherExternalServiceSchemaJSON
stringdata phabricator.schema.json PhabricatorSchemaJSON
stringdata perforce.schema.json PerforceSchemaJSON
stringdata settings.schema.json SettingsSchemaJSON
stringdata site.schema.json SiteSchemaJSON

gofmt -s -w ./*.go
