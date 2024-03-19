# AWS CodeCommit

Site admins can sync Git repositories hosted on [AWS CodeCommit](https://aws.amazon.com/codecommit/) with Sourcegraph so that users can search and navigate the repositories.

To connect AWS CodeCommit to Sourcegraph:

1. Go to **Site admin > Manage code hosts > Add repositories**
1. Select **AWS CodeCommit repositories**.
1. Configure the connection to AWS CodeCommit using the action buttons above the text field, and additional fields can be added using <kbd>Cmd/Ctrl+Space</kbd> for auto-completion. See the [configuration documentation below](#configuration).
1. Press **Add repositories**.

## AWS CodeCommit Git credentials

Since version **3.4** of Sourcegraph, the AWS CodeCommit service **requires** [Git credentials](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_ssh-keys.html#git-credentials-code-commit) in order to clone repositories via HTTPS. Git credentials consist of a username and a password that you can create in AWS IAM. 

For detailed instructions on how to create the credentials in IAM, see: [Setup for HTTPS Users Using Git Credentials](https://docs.aws.amazon.com/codecommit/latest/userguide/setting-up-gc.html)

## Configuration

AWS CodeCommit connections support the following configuration options, which are specified in the JSON editor in the site admin "Manage code hosts" area.

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/external_service/aws_codecommit.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/admin/external_service/aws_codecommit) to see rendered content.</div>

## Setup steps for SSH connections to AWS CodeCommit repositories

To add CodeCommit repositories in Docker Container:

1. Generate a public/private rsa key pair that does not require passphrase as listed in the [Step 3.1 of the AWS SSH setup guide](https://docs.aws.amazon.com/codecommit/latest/userguide/setting-up-ssh-unixes.html#setting-up-ssh-unixes-keys). Sourcegraph does not work withe key pair that requires passphrase.
1. Follow the rest of the steps detailed in the [AWS SSH setup guide](https://docs.aws.amazon.com/codecommit/latest/userguide/setting-up-ssh-unixes.html) to make sure you can connect to the code host locally.
1. Confirm you have the connection by running the following ssh command locally: `ssh git-codecommit.us-west-1.amazonaws.com` (Update link with your server region)
1. Confirm you can clone the repository locally.
1. Copy all the files at your `$HOME/.ssh directory` to `$HOME/.sourcegraph/config/ssh` directory. See [docs](../deploy/docker-single-container/index.md#ssh-authentication-config-keys-knownhosts) for more information about our ssh file system.
    1. Read our [guide here](../deploy/docker-compose/index.md#git-ssh-configuration) for Docker Compose deployments
    1. Read our [guide here](../deploy/kubernetes/configure.md#ssh-for-cloning) for Kubernetes deployments
1. Start (or restart) the container.
1. Connect Sourcegraph to AWS CodeCommit by going to **Sourcegraph > Site Admin > Manage code hosts > Generic Git host** and add the following:

```json
"url": "ssh://git-codecommit.us-west-1.amazonaws.com", //Please replace the 'us-east-1' region with yours
  "repos": [
    "v1/repos/REPO_NAME_1",
    "v1/repos/REPO_NAME_2",
  ]
``` 
