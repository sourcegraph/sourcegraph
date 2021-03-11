# Quickstart step 5: Configure HTTPS reverse proxy

Sourcegraph's development environment ships with a [Caddy 2](https://caddyserver.com/) HTTPS reverse proxy that allows you to access your local sourcegraph instance via `https://sourcegraph.test:3443` (a fake domain with a self-signed certificate that's added to `/etc/hosts`).

If you'd like Sourcegraph to be accessible under `https://sourcegraph.test` (port 443) instead, you can [set up authbind](https://medium.com/@steve.mu.dev/setup-authbind-on-mac-os-6aee72cb828) and set the environment variable `SOURCEGRAPH_HTTPS_PORT=443`.

## Prerequisites

In order to configure the HTTPS reverse-proxy, you'll need to edit `/etc/hosts` and initialize Caddy 2.

## Add `sourcegraph.test` to `/etc/hosts`

`sourcegraph.test` needs to be added to `/etc/hosts` as an alias to `127.0.0.1`. There are two main ways of accomplishing this:

1. Manually append `127.0.0.1 sourcegraph.test` to `/etc/hosts`
1. Use the provided `./dev/add_https_domain_to_hosts.sh` convenience script (sudo may be required).

```bash
> ./dev/add_https_domain_to_hosts.sh

--- adding sourcegraph.test to '/etc/hosts' (you may need to enter your password)
Password:
Adding host(s) "sourcegraph.test" to IP address 127.0.0.1
--- printing '/etc/hosts'
...
127.0.0.1        localhost sourcegraph.test
...
```

## Initialize Caddy 2

[Caddy 2](https://caddyserver.com/) automatically manages self-signed certificates and configures your system so that your web browser can properly recognize them. The first time that Caddy runs, it needs `root/sudo` permissions to add
its keys to your system's certificate store. You can get this out the way after installing Caddy 2 by running the following command and entering your password if prompted:

```bash
./dev/caddy.sh trust
```

Note: If you are using Firefox and have a master password set, the following prompt will come up first:

```
Enter Password or Pin for "NSS Certificate DB":
```

Enter your Firefox master password here and proceed. See [this issue on GitHub](https://github.com/FiloSottile/mkcert/issues/50) for more information.

You might need to restart your web browsers in order for them to recognize the certificates.

[< Previous](quickstart_4_clone_repository.md) | [Next >](quickstart_6_start_server.md)
