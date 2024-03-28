# init-sg

A go tool to initialize a sourcegraph instance and create a sudo token, and add repos

## Usage

### initSG

Creates a new user, with a sudo access token attached. The access token is appended to ~/.profile

### addRepos

Adds a github external service, via a json config file.

## Testing

Ensure you have an instance of Souregraph running.

### initSG

The arguments for `initSG` can be set via environment variables or the cli.

with env vars

```shell
SOURCEGRAPH_BASE_URL=http://<sourcegraph>
TEST_USER_EMAIL=test@test.com
SOURCEGRAPH_SUDO_USER=admin
TEST_USER_PASSWORD=password

$ go build && init-sg initSG
Running initializer
Site admin has been created: admin
Instance initialized, SOURCEGRAPH_SUDO_TOKEN set in /root/.profile
```

with cli flags

```shell
$ go build && init-sg initSG -baseurl=http://<sourcegraph> email=<useremail> -username=<username> -password=<password>
```

### addRepos

The Github token required for the external service can be set via an environment variable or the cli

with env vars

```shell
GITHUB_TOKEN=<token>
$ go build && init-sg addRepos -config extsvc.json
Site admin authenticated: admin
{"url":"https://github.com","token":"<redacted>","repos":["sourcegraph-testing/etcd","sourcegraph-testing/tidb","sourcegraph-testing/titan","sourcegraph-testing/zap"]}
github.com/sourcegraph-testing/etcd
github.com/sourcegraph-testing/tidb
github.com/sourcegraph-testing/titan
github.com/sourcegraph-testing/zap
```

with cli flags

```shell
go build && init-sg addRepos -githubtoken <token> -config extsvc.json
```

The format for the external service json file is as follows, multiple github external services can be configured:

```json
[
  {
    "Kind": "GITHUB",
    "DisplayName": "Github repos",
    "Config": {
      "url": "https://github.com",
      "repos": ["sourcegraph/sourcegraph", "sourcegraph/deploy-sourcegraph"]
    }
  }
]
```
Hello World
