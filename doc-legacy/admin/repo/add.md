# Add repositories (from code hosts) to Sourcegraph

- [Add repositories from a code host](../external_service/index.md) (GitHub (Cloud or Self-hosted), GitLab (Cloud or Self-hosted), Bitbucket Server, Bitbucket Data Center, or Perforce)
  - [GitHub](../external_service/github.md)
  - [GitLab](../external_service/gitlab.md)
  - [Bitbucket Cloud](../external_service/bitbucket_cloud.md)
  - [Bitbucket Server / Bitbucket Data Center](../external_service/bitbucket_server.md) or Bitbucket Data Center
<!--    - [Phabricator](../external_service/phabricator.md) -->
<!--   - [Gitolite](../external_service/gitolite.md) -->
<!--    - [AWS CodeCommit](../external_service/aws_codecommit.md) -->
- [Add repositories by Git clone URLs](../external_service/other.md)
- [Add repositories from non-Git code hosts](../external_service/non-git.md)
  - [Add Perforce repositories](perforce.md)
  - [Add JVM dependencies](../external_service/jvm.md)
  - [Add Go dependencies](../external_service/go.md)
  - [Add npm dependencies](../external_service/npm.md)
  - [Add Python dependencies](../external_service/python.md)
- [Pre-load repositories from the local disk](pre_load_from_local_disk.md)

## Troubleshooting

If your repositories are not showing up, check the site admin **Repositories** page on Sourcegraph (and ensure you're logged in as an admin).
If nothing informative is visible there, check for error messages related to communication with your code host's API in the logs from:

- [Docker Compose](../deploy/docker-compose/index.md) and [Kubernetes](../deploy/kubernetes/index.md): the logs from the `repo-updater` container/pod
- [Single-container](../deploy/docker-single-container/index.md): the `sourcegraph/server` Docker container

If your repositories are showing up but are not cloning or updating from the original Git repository:

- Follow the instructions for [troubleshooting a repository that is not being updated](../../admin/how-to/repo-not-updated.md)
