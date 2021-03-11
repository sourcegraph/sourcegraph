# Install Sourcegraph with Docker

> NOTE: We *do not* recommend using this method for a production instance. If deploying a production instance, see [our recommendations](../index.md) for how to chose a deployment type that suits your needs. We recommend [Docker Compose](../docker-compose/index.md) for most initial production deployments.

---

It takes less than a minute to run and install Sourcegraph using Docker:

<!--
  DO NOT CHANGE THIS TO A CODEBLOCK.
  We want line breaks for readability, but backslashes to escape them do not work cross-platform.
  This uses line breaks that are rendered but not copy-pasted to the clipboard.
-->

<pre class="pre-wrap start-sourcegraph-command"><code>docker run<span class="virtual-br"></span> --publish 7080:7080 --publish 127.0.0.1:3370:3370 --rm<span class="virtual-br"></span> --volume ~/.sourcegraph/config:/etc/sourcegraph<span class="virtual-br"></span> --volume ~/.sourcegraph/data:/var/opt/sourcegraph<span class="virtual-br"></span> sourcegraph/server:3.25.2</code></pre>

Once the server is ready (logo is displayed in the terminal), navigate to the hostname or IP address on port `7080`.  Create the admin account, then you'll be guided through setting up Sourcegraph for code searching and navigation.

<!--
TODO(ryan): Replace with updated screencast
<p class="container">
  <div style="padding:56.25% 0 0 0;position:relative;">
    <iframe src="https://player.vimeo.com/video/314926561?color=0CB6F4&title=0&byline=0&portrait=0" style="position:absolute;top:0;left:0;width:100%;height:100%;" frameborder="0" webkitallowfullscreen mozallowfullscreen allowfullscreen></iframe>
  </div>
</p>
-->

For next steps and further configuration options, visit the [site administration documentation](../../index.md).

> NOTE: If you get stuck or need help, [file an issue](https://github.com/sourcegraph/sourcegraph/issues/new?&title=Improve+Sourcegraph+quickstart+guide), [tweet (@srcgraph)](https://twitter.com/srcgraph) or [email](mailto:support@sourcegraph.com?subject=Sourcegraph%20quickstart%20guide).

## Cloud installation guides

Cloud specific Sourcegraph installation guides for AWS, Google Cloud and Digital Ocean.

- [Install Sourcegraph with Docker on AWS](../../install/docker/aws.md)
- [Install Sourcegraph with Docker on Google Cloud](../../install/docker/google_cloud.md)
- [Install Sourcegraph with Docker on DigitalOcean](../../install/docker/digitalocean.md)

## File system performance on Docker for Mac

There is a [known issue](https://github.com/docker/for-mac/issues/77) in Docker for Mac that causes slower than expected file system performance on volume mounts, which impacts the performance of search and cloning.

To achieve better performance, you can do any of the following:

- For better clone performance, clone the repository on your host machine and then [add it to Sourcegraph Server](../../repo/add.md#add-repositories-already-cloned-to-disk).
- Try adding the `:delegated` suffix the data volume mount. [Learn more](https://github.com/docker/for-mac/issues/1592).
  ```
  --volume ~/.sourcegraph/data:/var/opt/sourcegraph:delegated
  ```

### Testing Sourcegraph on Windows

Sourcegraph can be **tested** on Windows 10 using roughly the same steps provided above, but data will not be retained after server restarts ([this is due to a limitation of Docker on Windows](https://github.com/docker/for-win/issues/39#issuecomment-371942845)).

1. [Install Docker for Windows](https://docs.docker.com/docker-for-windows/install/)
2. Using a command prompt, follow the same [installation steps provided above](#install-sourcegraph-with-docker) but remove the `--volume` arguments. For example by pasting this:

<pre class="pre-wrap"><code>docker run<span class="virtual-br"></span> --publish 7080:7080 --publish 127.0.0.1:3370:3370 --rm<span class="virtual-br"></span> sourcegraph/server:3.25.2</code></pre>

## Low resource environments

To test sourcegraph in a low resource environment you may want to disable some of the observability tools (Prometheus, Grafana and Jaeger).

Add `-e DISABLE_OBSERVABILITY=true` to your docker run command

## Starting in Postgres restore mode

In order to restore a Postgres backup, you need to start on an empty database and prevent all other Sourcegraph services from starting.
You can do this by adding `-e PGRESTORE=true` to your `docker run` command. This will start only the Postgres system and allow you to perform a restore, once it is done, simply remove that parameter from your docker command.

The database is only accessible from within the container, to perform a restore you will need to copy the required files to the container and then execute the restore commands from within the container using `docker exec`.

You can find examples of this procedure for `docker-compose` in our [docker-compose migration docs](../docker-compose/migrate.md).

## Insiders build

To test new development builds of Sourcegraph (triggered by commits to `main`), change the tag to `insiders` in the `docker run` command.

> WARNING: `insiders` builds may be unstable, so back up Sourcegraph's data and config (usually `~/.sourcegraph`) beforehand.

```bash
docker run --publish 7080:7080 --rm --volume ~/.sourcegraph/config:/etc/sourcegraph --volume ~/.sourcegraph/data:/var/opt/sourcegraph sourcegraph/server:insiders
```

To keep this up to date, run `docker pull sourcegraph/server:insiders` to pull in the latest image, and restart the container to access new changes.

## Next steps

- [Configuring Sourcegraph](../../config/index.md)
- [Upgrading Sourcegraph](../../updates.md)
- [Site administration documentation](../../index.md)
