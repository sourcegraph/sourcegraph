# Custom git config

Sourcegraph supports customising [git-config](https://git-scm.com/docs/git-config) and [ssh_config](https://linux.die.net/man/5/ssh_config) for adjusting the behaviour of git. Sourcegraph will read these from the standard locations.

This guide documents how to configure git-config. To set up SSH and authentication for repositories, see [Repository authentication](auth.md).

- [Sourcegraph with Docker Compose](../deploy/docker-compose/index.md): See [the Docker Compose git configuration guide](../deploy/docker-compose/index.md#git-configuration).
- [Sourcegraph with Kubernetes](../deploy/kubernetes/index.md): See [Configure repository cloning via SSH](../deploy/kubernetes/configure.md#ssh-for-cloning).
- [Single-container Sourcegraph](../deploy/docker-single-container/index.md): See [the single-container git configuration guide](../deploy/docker-single-container/index.md#git-configuration-and-authentication).

## Example: alternate clone URL for repos

Some sites put an HTTPS or SSH proxy in front of their code host to reduce load. Some sites also use a service like AWS CodeCommit to do the same. In these cases, the repos still should be treated as being repos on the original code host, not the proxy site.

For example, I have a GitHub repo `github.com/foo/bar`. I want Sourcegraph to clone it using the URL https://cloneproxy.example.com/foo/bar.git. But I still want the "Go to GitHub repository" button, etc., to take me to https://github.com/foo/bar. You can use the git configuration [`insteadOf`](https://git-scm.com/docs/git-config#Documentation/git-config.txt-urlltbasegtinsteadOf):

``` ini
# ~/.gitconfig or /etc/gitconfig
[url "https://cloneproxy.example.com"]
  insteadOf = https://github.com
```

If you are [cloning via SSH](./auth.md), you can also achieve this with an SSH configuration:

```
# ~/.ssh/config
Host github.com
  Hostname cloneproxy.example.com
```
