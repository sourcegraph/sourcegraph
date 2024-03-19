## Installation & versioning

Binary downloads are available on the [releases tab](https://github.com/sourcegraph/src-cli/releases), and through Sourcegraph.com. _If the latest version does not work for you,_ consider using the version compatible with your Sourcegraph instance instead.

### Installation: Mac OS

#### Latest version

```bash
curl -L https://sourcegraph.com/.api/src-cli/src_darwin_amd64 -o /usr/local/bin/src
chmod +x /usr/local/bin/src
```

or with Homebrew:

```bash
brew install sourcegraph/src-cli/src-cli
```

or with npm:

```bash
npm install -g @sourcegraph/src
```

#### Version compatible with your Sourcegraph instance

Replace `sourcegraph.example.com` with your Sourcegraph instance URL:

```bash
curl -L https://sourcegraph.example.com/.api/src-cli/src_darwin_amd64 -o /usr/local/bin/src
chmod +x /usr/local/bin/src
```

or, if you know the specific version to target, for example 3.43.2:

```bash
brew install sourcegraph/src-cli/src-cli@3.43.2
```

or with npm/npx:

```bash
npx @sourcegraph/src@3.43.2 version
```

> Note: Versioned formulas are available on Homebrew for Sourcegraph versions 3.43.2 and later.

### Installation: Linux

#### Latest version

```bash
curl -L https://sourcegraph.com/.api/src-cli/src_linux_amd64 -o /usr/local/bin/src
chmod +x /usr/local/bin/src
```

or with npm:

```bash
npm install -g @sourcegraph/src
```

#### Version compatible with your Sourcegraph instance

Replace `sourcegraph.example.com` with your Sourcegraph instance URL:

```bash
curl -L https://sourcegraph.example.com/.api/src-cli/src_linux_amd64 -o /usr/local/bin/src
chmod +x /usr/local/bin/src
```

or, with npm/npx, if you know the specific version to target, for example 3.43.2:

```bash
npx @sourcegraph/src@3.43.2 version
```

### Installation: Windows

See [Sourcegraph CLI for Windows](windows.md).

### Installation: Docker

`sourcegraph/src-cli` is published to Docker Hub. You can use the `latest` tag or a specific version such as `3.43`. To see all versions view [sourcegraph/src-cli tags](https://hub.docker.com/r/sourcegraph/src-cli/tags).

```bash
docker run --rm=true sourcegraph/src-cli:latest search 'hello world'
```
