# Install single-container Sourcegraph with Docker on DigitalOcean

This tutorial shows you how to deploy [single-container Sourcegraph with Docker](./index.md) to a single node running on DigitalOcean.

> NOTE: We *do not* recommend using this method for a production instance. If deploying a production instance, see [our recommendations](../index.md) for how to choose a deployment type that suits your needs. We recommend [Docker Compose](../docker-compose/digitalocean.md) for most initial production deployments.


---

## Run Sourcegraph on a Digital Ocean Droplet

1. [Create a new Digital Ocean Droplet](https://cloud.digitalocean.com/droplets/new). Set the
   operating system to be Ubuntu 18.04. For droplet size, we recommend at least 4GB RAM and 2 CPU,
   but you may need more depending on team size and number of repositories. We recommend you set up
   SSH access (Authentication > SSH keys) for convenient access to the droplet.
1. SSH into the droplet, and install Docker: `snap install docker`
1. Run the Sourcegraph Docker image as a daemon:

   ```
   docker run -d --publish 80:7080 --publish 443:7443 --restart unless-stopped --volume /root/.sourcegraph/config:/etc/sourcegraph --volume /root/.sourcegraph/data:/var/opt/sourcegraph sourcegraph/server:5.2.5
   ```
1. Navigate to the droplet's IP address to finish initializing Sourcegraph. If you have configured a
   DNS entry for the IP, configure `externalURL` to reflect that.

### After initialization

After initial setup, we recommend you do the following:

* Restrict the accessibility of ports other than `80` and `443` via [Cloud
  Firewalls](https://www.digitalocean.com/docs/networking/firewalls/quickstart/).
* Set up [TLS/SSL](../../http_https_configuration.md#nginx-ssl-https-configuration) in the NGINX configuration.

---

## Update your Sourcegraph version

To update to the most recent version of Sourcegraph (X.Y.Z), SSH into your instance and run the following:

```
docker ps # get the $CONTAINER_ID of the running sourcegraph/server container
docker rm -f $CONTAINER_ID
docker run -d ... sourcegraph/server:X.Y.Z
```
