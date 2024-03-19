# Install single-container Sourcegraph with Docker on AWS

This tutorial shows you how to deploy [single-container Sourcegraph with Docker](./index.md) to a single EC2 instance on AWS.

> NOTE: We *do not* recommend using this method for a production instance. If deploying a production instance, see [our recommendations](../index.md) for how to choose a deployment type that suits your needs. We recommend [Docker Compose](../docker-compose/aws.md) for most initial production deployments.

---

## Deploy to EC2

- Click **Launch Instance** from your [EC2 dashboard](https://console.aws.amazon.com/ec2/v2/home).
- Select the Amazon Linux 2 AMI (HVM), SSD Volume Type.
- Select an appropriate instance size (we recommend `t2.medium` or `t2.large`, depending on team size and number of repositories/languages enabled), then **Next: Configure Instance Details**
- Ensure the **Auto-assign Public IP** option is "Enable". This ensures your instance is accessible to the Internet.
- Add the following user data (as text) in the **Advanced Details** section:

   ```
   #cloud-config
   repo_update: true
   repo_upgrade: all

   runcmd:
   # Create the directory structure for Sourcegraph data
   - mkdir -p /home/ec2-user/.sourcegraph/config
   - mkdir -p /home/ec2-user/.sourcegraph/data

   # Install, configure, and enable Docker
   - yum update -y
   - amazon-linux-extras install docker
   - systemctl enable --now --no-block docker
   - sed -i -e 's/1024/10240/g' /etc/sysconfig/docker
   - sed -i -e 's/4096/40960/g' /etc/sysconfig/docker
   - usermod -a -G docker ec2-user

   # Install and run Sourcegraph. Restart the container upon subsequent reboots
   - [ sh, -c, 'docker run -d --publish 80:7080 --publish 443:7080 --publish 127.0.0.1:3370:3370 --restart unless-stopped --volume /home/ec2-user/.sourcegraph/config:/etc/sourcegraph --volume /home/ec2-user/.sourcegraph/data:/var/opt/sourcegraph sourcegraph/server:5.2.5' ]
   ```

- Select **Next: ...** until you get to the **Configure Security Group** page. Then add the following rules:
  - Default **HTTP** rule: port range `80`, source `0.0.0.0/0, ::/0`
  - Default **HTTPS** rule: port range `443`, source `0.0.0.0/0, ::/0`<br>(NOTE: additional work will be required later on to [configure NGINX to support SSL](../../../admin/http_https_configuration.md#nginx-ssl-https-configuration))
- Launch your instance, then navigate to its public IP in your browser. (This can be found by navigating to the instance page on EC2 and looking in the "Description" panel for the "IPv4 Public IP" value.) You may have to wait a minute or two for the instance to finish initializing before Sourcegraph becomes accessible. You can monitor the status by SSHing into the EC2 instance and viewing the logs:

     ```
     docker logs $(docker ps | grep sourcegraph/server | awk '{ print $1 }')
     ```
- If you have configured a domain name to point to the IP, configure `externalURL` to reflect that.


  <!-- - mention you might have to wait a bit for the Docker container to fully spin up (include instructions for checking logs for health) -->

---

## Update your Sourcegraph version

To update to the most recent version of Sourcegraph (X.Y.Z), SSH into your instance and run the following:

```bash
docker ps # get the $CONTAINER_ID of the running sourcegraph/server container
docker rm -f $CONTAINER_ID
docker run docker run -d --publish 80:7080 --publish 443:7080 --restart unless-stopped --volume /home/ec2-user/.sourcegraph/config:/etc/sourcegraph --volume /home/ec2-user/.sourcegraph/data:/var/opt/sourcegraph sourcegraph/server:X.Y.Z
```

---

## Using an external database for persistence

The Docker container has its own internal PostgreSQL and Redis databases. To preserve this data when you kill and recreate the container, you can [use external services](../../external_services/index.md) for persistence, such as [AWS RDS for PostgreSQL](https://aws.amazon.com/rds/), [Amazon ElastiCache](https://aws.amazon.com/elasticache/redis/), and [S3](https://aws.amazon.com/s3/) for storing user uploads.

> NOTE: Use of external databases requires [Sourcegraph Enterprise](https://sourcegraph.com/pricing).
