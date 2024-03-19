# Adding SSL (HTTPS) to Sourcegraph with a self-signed certificate

This is for external Sourcegraph instances that need a self-signed certificate because they don't yet have a certificate from a [globally trusted Certificate Authority (CA)](https://en.wikipedia.org/wiki/Certificate_authority#Providers). It includes how to get the self-signed certificate trusted by your browser.

> NOTE: Using an IP address also works, including the browser being able to trust (validate) the SSL certificate.

Configuring NGINX with a self-signed certificate to support SSL requires:

- [Adding SSL (HTTPS) to Sourcegraph with a self-signed certificate](#adding-ssl-https-to-sourcegraph-with-a-self-signed-certificate)
  - [1. Installing mkcert](#1-installing-mkcert)
  - [2. Creating the self-signed certificate](#2-creating-the-self-signed-certificate)
  - [3. Adding SSL support to NGINX](#3-adding-ssl-support-to-nginx)
  - [4. Changing the Sourcegraph container to listen on port 443](#4-changing-the-sourcegraph-container-to-listen-on-port-443)
  - [5. Getting the self-signed certificate to be trusted (valid) on external instances](#5-getting-the-self-signed-certificate-to-be-trusted-valid-on-external-instances)
  - [Next steps](#next-steps)

## 1. Installing mkcert

While the [OpenSSL](https://wiki.openssl.org/index.php/Command_Line_Utilities) CLI can can generate self-signed certificates, its API is challenging unless you're well versed in SSL.

A better alternative is [mkcert](https://github.com/FiloSottile/mkcert#mkcert), an abstraction over OpenSSL written by [Filippo Valsorda](https://github.com/FiloSottile), a cryptographer working at Google on the Go team.

> NOTE: The following commands are to be run on the Docker host, **not** inside the Sourcegraph container.

To set up mkcert on the Sourcegraph instance:

1. [Install mkcert](https://github.com/FiloSottile/mkcert#installation)
1. Create the root [CA](https://en.wikipedia.org/wiki/Certificate_authority) by running:

```bash
sudo CAROOT=~/.sourcegraph/config mkcert -install
```

## 2. Creating the self-signed certificate

Now that the root CA has been created, mkcert can issue a self-signed certificate (`sourcegraph.crt`) and key (`sourcegraph.key`).

```bash
sudo CAROOT=~/.sourcegraph/config mkcert \
  -cert-file ~/.sourcegraph/config/sourcegraph.crt \
  -key-file ~/.sourcegraph/config/sourcegraph.key \
  $HOSTNAME_OR_IP
```

Run `sudo ls -la ~/.sourcegraph/config` and you should see the CA and SSL certificates and keys.

## 3. Adding SSL support to NGINX

Edit the [default
`~/.sourcegraph/config/nginx.conf`](https://github.com/sourcegraph/sourcegraph/blob/main/cmd/server/shared/assets/nginx.conf),
so that port `7080` redirects to `7443` and `7443` is served with SSL. It should look like this:

```nginx
...
http {
    ...
    server {
        listen 7080;
        return 301 https://$host:7443$request_uri;
    }

    server {
        # Do not remove. The contents of sourcegraph_server.conf can change
        # between versions and may include improvements to the configuration.
        include nginx/sourcegraph_server.conf;

        listen 7443 ssl;
        server_name sourcegraph.example.com;  # change to your URL
        ssl_certificate         sourcegraph.crt;
        ssl_certificate_key     sourcegraph.key;

        location / {
            ...
        }
    }
}
```

## 4. Changing the Sourcegraph container to listen on port 443

> NOTE: If the Sourcegraph container is still running, stop it before reading on.

Now that NGINX is listening on port 7443, we need to configure the Sourcegraph container to forward
443 to 7443 by adding `--publish 443:7443` to the `docker run` command:

```bash
docker container run \
  --rm  \
  --publish 7080:7080 \
  --publish 443:7443 \
  \
  --volume ~/.sourcegraph/config:/etc/sourcegraph  \
  --volume ~/.sourcegraph/data:/var/opt/sourcegraph  \
  sourcegraph/server:5.2.5
```

> NOTE: We recommend removing `--publish 7080:7080` as it's not needed and traffic sent to that port is un-encrypted.

Run the new Docker command, then validate by opening your browser at `https://$HOSTNAME_OR_IP`.

If running Sourcegraph locally, the certificate will be valid because `mkcert` added the root CA to the list trusted by your OS.

## 5. Getting the self-signed certificate to be trusted (valid) on external instances

To have the browser trust the certificate, the root CA on the Sourcegraph instance must be installed locally by:

**1.** [Installing mkcert locally](https://github.com/FiloSottile/mkcert#installation)

**2.** Downloading `rootCA-key.pem` and `rootCA.pem` from `~/.sourcegraph/config/mkcert` on the Sourcegraph instance to the location of `mkcert -CAROOT` on your local machine:

```bash
# Run locally: Ensure directory the root CA files will be downloaded to exists
mkdir -p "$(mkcert -CAROOT)"
```

```bash
# Run on Sourcegraph host: Ensure `scp` user can read (and therefore download) the root CA files
sudo chown $USER ~/.sourcegraph/config/root*
```

```bash
# Run locally: Download the files (change username and hostname)
scp user@example.com:~/.sourcegraph/config/root* "$(mkcert -CAROOT)"
```

**3.** Install the root CA by running:

```bash
mkcert -install
```

Open your browser again at `https://$HOSTNAME_OR_IP` and this time, your certificate should be valid.

### Getting the self-signed cert trusted on other developer machines

This is largely the same as step 5, except easier. For other developer machines to trust the self-signed cert:

- [Install mkcert](https://github.com/FiloSottile/mkcert#installation).
- Download the `rootCA-key.pem` and `rootCA.pem` from Slack or other internal system.
- Move the `rootCA-key.pem` and `rootCA.pem` files into the `mkcert -CAROOT` directory on their machine.
- Run `mkcert -install` on their machine.

## Next steps

- [Configure Sourcegraph's `externalURL`](config/site_config.md)
- [Redirect to external HTTPS URL](http_https_configuration.md#redirect-to-external-https-url)
- [NGINX HTTP Strict Transport Security](http_https_configuration.md#redirect-to-external-https-url)
- [NGINX SSL Termination guide](https://docs.nginx.com/nginx/admin-guide/security-controls/terminating-ssl-http/)
- [NGINX HTTPS Servers guide](https://nginx.org/en/docs/http/configuring_https_servers.html).
