# Sync repositories from gitolite.sgdev.org

>NOTE: SSH key configuration is not yet available on [Sourcegraph Cloud](../../cloud/index.md).

1. Create `~/.ssh/gitolite_ssh_key` and paste in the [private key stored in 1Password](https://start.1password.com/open/i?a=HEDEDSLHPBFGRBTKAKJWE23XX4&v=dnrhbauihkhjs5ag6vszsme45a&i=i5bm6syw45w2c33cvfrrlt4fhu&h=team-sourcegraph.1password.com)
1. Run `chmod 400 ~/.ssh/gitolite_ssh_key` to give it correct permissions
1. Create `~/.ssh/gitolite_ssh_key.pub` and paste in the *public* key stored in same 1Password entry
1. Edit `~/.ssh/known_hosts` and add the "known hosts entry" from same 1Password entry
1. Edit `~/.ssh/config` and add the following to tell SSH to use the key we just created when connecting to `gitolite.sgdev.org`:

    ```
    Host gitolite.sgdev.org
      User git
      IdentityFile ~/.ssh/gitolite_ssh_key
      IdentitiesOnly yes
    ```
1. Create an external service with the following configuration:

    ```
    {
      "host": "git@gitolite.sgdev.org",
      "prefix": "gitolite.sgdev.org/"
    }
    ```
