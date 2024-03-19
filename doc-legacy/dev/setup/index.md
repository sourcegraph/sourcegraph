# Local Environment Setup

<style>
.markdown-body h2 {
  margin-top: 2em;
}
.markdown-body ul {
  list-style:none;
  padding-left: 1em;
}
.markdown-body ul li {
  margin: 0.5em 0;
}
.markdown-body ul li:before {
  content: '';
  display: inline-block;
  height: 1.2em;
  width: 1em;
  background-size: contain;
  background-repeat: no-repeat;
  background-image: url(../batch_changes/file-icon.svg);
  margin-right: 0.5em;
  margin-bottom: -0.29em;
}
body.theme-dark .markdown-body ul li:before {
  filter: invert(50%);
}
</style>

<p class="subtitle">Working on Sourcegraph code on your local machine</p>

<div class="cta-group">
<a class="btn btn-primary" href="quickstart">★ Quickstart with <code>sg</code></a>
<a class="btn" href="../how-to">How-tos</a>
<a class="btn" href="#troubleshooting">Troubleshooting</a>
</div>


## Quickstart

In order to run the Sourcegraph locally, the following pages will guide you from zero to having a local environment up and running, ready for contributions for the the most common use cases.

- [**Quickstart with `sg`**](quickstart.md)

The quick start guide above provides a standard approach, focused on simplicity and accessiblility. But it's not the only way, the dependencies page below documents more exhaustively various approaches to set them up.

- [Dependencies](dependencies.md)

## Troubleshooting

The following pages list common problems with the local environment and their solutions:

- [Problems with node_modules or Javascript packages](troubleshooting.md#problems-with-nodemodules-or-javascript-packages)
- [dial tcp 127.0.0.1:3090: connect: connection refused](troubleshooting.md#dial-tcp-1270013090-connect-connection-refused)
- [Database migration failures](troubleshooting.md#database-migration-failures)
- [Internal Server Error](troubleshooting.md#internal-server-error)
- [Increase maximum available file descriptors.](troubleshooting.md#increase-maximum-available-file-descriptors)
- [Caddy 2 certificate problems](troubleshooting.md#caddy-2-certificate-problems)
- [Running out of disk space](troubleshooting.md#running-out-of-disk-space)
- [Certificate expiry](troubleshooting.md#certificate-expiry)
- [CPU/RAM/bandwidth/battery usage](troubleshooting.md#cpurambandwidthbattery-usage)
