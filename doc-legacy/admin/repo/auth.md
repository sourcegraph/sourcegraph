# Repository authentication

If authentication (HTTP(S) or SSH) is required to `git clone` a repository then you must provide credentials to the container.

First, ensure your **Site admin > Manage code hosts** code host configuration is configured to use SSH. For example, by setting the `gitURLType` field to "ssh". Alternatively, you may use the "Generic Git host" code host type, which allows you to directly specify Git repository URLs for cloning.

Then, follow the directions below depending on your deployment type:

- [Sourcegraph with Docker Compose](../deploy/docker-compose/index.md): See [the Docker Compose git configuration guide](../deploy/docker-compose/index.md#git-configuration).
- [Sourcegraph with Kubernetes](../deploy/kubernetes/index.md): See [Configure repository cloning via SSH](../deploy/kubernetes/configure.md#ssh-for-cloning).
- [Single-container Sourcegraph](../deploy/docker-single-container/index.md): See [the single-container git configuration guide](../deploy/docker-single-container/index.md#git-configuration-and-authentication).

>NOTE: Repository access over SSH is not yet supported on [Sourcegraph Cloud](../../cloud/index.md).

## Troubleshooting

### What should be included in my config file?

We recommend adding the `StrictHostKeyChecking no` and `AddKeysToAgent yes` flags to prevent the need to give permission interactively when cloning from a new host.

```yaml
Host *
  StrictHostKeyChecking no
  AddKeysToAgent yes
```

See [git configuration](./git_config.md) for more details.

### Error: `sign_and_send_pubkey: no mutual signature supported`

In Sourcegraph 5.1.0 and later, the insecure SSH rsa-sha1 signature algorithm is no longer supported when fetching data from code hosts.

If you use an RSA SSH key to authenticate to your code host, you should ensure that your code host runs OpenSSL 7.2 or newer.

If it is not possible to update the code host, you should generate a new ed25519 SSH key to use for authentication. This can be achieved by running `ssh-keygen -t ed25519`, and [configuring Sourcegraph](https://docs.sourcegraph.com/admin/repo/git_config) to use this new key.

### Error: `Host key verification failed`

This indicates an invalid key is being used. You can confirm the error by cloning inside the gitserver directly. For example:

```bash
docker exec -it gitserver-0 sh
cd data/repos/<CODE-HOST>/<REPO-OWNER>
git clone <SSH-URL>
```

#### Error: `Bad owner or permissions on /home/sourcegraph/.ssh/<YOUR-CONFIG-FILE>`

This indicates the container is having trouble reading the configuration files due to permission / owner issues.
The permission and ownership settings inside your `.ssh/` directory should look similar to:
```bash
$ ls -al #command to display list of file with detailed information
total 20
drwxr-xr-x    6 sourcegr sourcegr       192 May 12 19:54 .
drwxr-sr-x    1 sourcegr sourcegr      4096 May 12 19:43 ..
-rw-------    1 sourcegr sourcegr        34 May 12 19:22 config
-rw-------    1 sourcegr sourcegr       411 May 12 18:52 id_ed25519
-rw-------    1 sourcegr sourcegr        98 May 12 18:52 id_ed25519.pub
-rw-------    1 sourcegr sourcegr       799 May 12 19:54 known_hosts
```

##### Solution

- Inside the `.ssh` directory on the Host Machine:
  - Permission on all files must be set to `600`, and `700` for the directory itself.
  - Files must be owned by a user who has access to the docker container. This can be done via `sudo chown -v -R $USER:$GROUP` (the user may need to set these values).
- (**OR**) Inside the `/home/sourcegraph/` directory on Docker Compose:
  - Permission on all files must be set to `600`, and `700` for the directory itself.
  - Files must be owned by the root user, which is `sourcegraph` by default. This can be done via `sudo chown -v -R $USER:$GROUP` (the user may need to set these values).


>NOTE: Once the volume for the configuration files has been mounted, both the `/ssh` directory on the host machine and docker will be synced and changes within one directory will be reflected by the other one. Consquently, you will only need to make the changes within one directory.

#### Error: `Permissions 0644 for '/home/sourcegraph/.ssh/<YOUR-PRIVATE-KEY-FILE>' are too open`

This indicates the permission on your private key file is accessible by users other than the file owner. Setting the file permission to 600 resolves the issue.
