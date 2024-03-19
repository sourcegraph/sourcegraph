# Configuring a test instance of Phabricator and Gitolite

## Gitolite

### Setup

#### Kubernetes

1. Spin up gitolite.sgdev.org in the tooling cluster if it does not yet exist.

Create the gitolite pods by navigating to the *infrastructure* repository and applying the Gitolite config.

```shell
cd sourcegraph/infrastructure/kubernetes/tooling
kubectl apply -f ./gitolite
```

2. Follow [readme docs](https://github.com/sourcegraph/infrastructure/tree/master/docker-images/gitolite)

#### Locally

Alternatively, you can [run gitolite via docker
locally.](https://github.com/miracle2k/dockerfiles/tree/master/gitolite)
I suggest just using gitolite.sgdev.org because it can messy
getting your gitolite docker container to talk to your
Phabricator docker container.

```shell
docker run -p 22:22 -e SSH_KEY="$(cat ~/.ssh/id_rsa.pub)" elsdoerfer/gitolite

# Print info from gitolite
ssh -p22 git@localhost info
```

### Managing repositories

Managing your gitolite instance is done via the `gitolite-admin`
repository. You simply clone it, commit configuration changes
and push back to the remote and gitolite will apply any changes
you made.

```shell
git clone git@<your-gitolite-host>:gitolite-admin
```

If you'd like to create a new private/public key pair for
Phabricator, add the public key to `gitolite-admin/keydir` and
modify `gitolite-admin/conf/gitolite.conf` to look the following

```shell
repo gitolite-admin
  RW+ = @all

repo testing
  RW+ = @all
```

To create a new repository, add a new entry in this file and
gitolite will provision a new repository once you push the
update to the remote.

Once your changes in `gitolite-admin` are done, commit and push
to the remote.

## Phabricator

Phabricator is primarily used as a code review tool. Phabricator's code
reviews are of patches from a changeset rather than a diff between a
target (e.g. master) and a source branch (e.g. my-new-feature) as is the
case with pull requests. This means that it doesn't have to upload
changes to the git remote of a repository. Because of this, Sourcegraph
must get the changes a user is viewing from somewhere else.

Ideally, [staging areas (cmd+f for "staging
area")](https://secure.phabricator.com/book/phabricator/article/harbormaster/)
are enabled for each repository and accessible by Sourcegraph. If this
is the case, Sourcegraph simply takes changes from the staging area,
which is itself a git repository.

If staging areas aren't enabled, Sourcegraph [takes the patchset from
the diff and attempts to apply them to the
repository.](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/frontend/graphqlbackend/repository.go#L225-338)

### Setup

#### Kubernetes

Create the phabricator pods by navigating to the
*infrastructure* repository and applying the Phabricator
config.

```
cd sourcegraph/infrastructure/kubernetes/tooling
kubectl apply -f ./phabricator
```

#### Docker (local)

You can run locally via docker. We have
[<https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/dev/phabricator>](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/dev/phabricator)
for this using
[Bitnami.](https://docs.bitnami.com/installer/apps/phabricator/)

```shell
dev/phabricator/start.sh <tag>
# where <tag> is a release version from https://hub.docker.com/r/bitnami/phabricator/tags/

dev/phabricator/restart.sh
dev/phabricator/stop.sh
```

### Add repositories

1. Click the link to [Diffusion](http://localhost/diffusion/)

2. [Create a repository](http://localhost/diffusion/edit) and
    select git as the vcs

    Give the repository a name and a callsign with only
    alphanumeric characters (just the name in all caps works).

3. Click URIs in the side bar

4. Click on each URI that's already there, click edit, and set it to no I/O and hidden.

5. Go to the URIs list again and click Add New URI on the right side of the page, click edit, then set it to Observe and Visible.

6. Once created, click "Set Credential" on the right

    Click "Add New Credential". Fill out the form. Give it a private key from a public/private key pair that has access to your gitolite. To add it to Gitolite, checkout the `gitolite-admin` repo and add the public key to `keydir/` under the name `$USER.pub`. Then open `conf/gitolite.conf` and give $USER read and write permissions to the repository.


7. Now go back to the "manage repository" page and click activate repository on the right.

    If configured correctly, Phabricator will start mirroring
    the repository.

### Install the Sourcegraph Phabricator extension.

You can use `dev/phabricator/install-sourcegraph.sh`. To install it manually:

SSH into your Phabricator instance and follow [the installation steps in the README.](https://github.com/sourcegraph/phabricator-extension/blob/master/README.md#installation)
If you used the helper scripts, the root Phabricator directory
will be `/opt/bitnami/phabricator`.

### Workflow for creating diffs

1. Install Phabricator's cli, [Arcanist](https://secure.phabricator.com/book/phabricator/article/arcanist/)

2. In your terminal, navigate to a repository that has been added to your Phabricator instance

3. Ensure `.arcconfig` has been added

    ```
      {
        "phabricator.uri" : "https://<your phabricator host>/"
      }
    ```

4. Make some changes and push the diff to Phabricator's

    Use `arc` to create a new branch

    ```
    arc branch my-branch
    ```

    Make some changes, commit them, and upload the diff to Phabricator. DO NOT PUSH them to the git remote. If you push the changes to the git remote, we no longer are testing a critical feature of the Sourcegraph Phabricator integration, which is that it uses staging areas if configured or attempts to apply patchsets.

    ```
    git add . git commit -m "some changes" arc diff
    ```

    `arc` uploaded the patch that `git` generated from your changes and creates an associated "diff". Diffs are code reviews for patchsets. Phabricator's philosophy is to keep diffs as small as possible so they can be reviewed quickly and thoroughly, but don't assume that users follow this. Create large diffs in your test cases.

    At this point, changes live on Phabricator that aren't in the git remote. Sourcegraph either gets these changes from [staging areas (cmd+f for "staging area")](https://secure.phabricator.com/book/phabricator/article/harbormaster/) or [it attempts to apply the patchset on a temporary clone of the repo.](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/frontend/graphqlbackend/repository.go#L225-338)

    You are now at a point where you can test the Sourcegraph extensions in Phabricator code review. Navigate to the diff that `arc` created in your browser and the extension should be working just as the browser extension does on GitHub.
