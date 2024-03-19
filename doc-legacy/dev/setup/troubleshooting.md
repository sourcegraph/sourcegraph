# Troubleshooting

## Problems with node_modules or Javascript packages

Noticing problems with <code>node_modules/</code> or package versions? Try
running this command to clear the local package cache.

```bash
rm -rf node_modules ./client/*/node_modules
pnpm install --force
```

## Node version out of date

```bash
Validating package.json...
error @: The engine "node" is incompatible with this module. Expected version "^v14.7.0". Got "14.5.0"
```
If you see an error like this you need to upgrade the version of node installed. You can do this with `nvm use` and then following the prompts to install or update to the correct Node.js version.

## dial tcp 127.0.0.1:3090: connect: connection refused

This means the `frontend` server failed to start, for some reason. Look through the previous logs for possible explanations, such as failure to contact the `redis` server, or database migrations failing.

## Internal Server Error

If you see this error when opening the app:

```
500 Internal Server Error template: app.html:21:70: executing "app.html" at <version "styles/styl...>: error calling version: open client/web/dist/main-ABCD.css: no such file or directory
```

that means the web builder hasn't finished compiling the styles yet (it takes about 3 minutes).

## Increase maximum available file descriptors.

`sg start` may ask you to run ulimit to increase the maximum number
of available file descriptors for a process. You can make this setting
permanent for every shell session by adding the following line to your
`.*rc` file (usually `.bashrc` or `.zshrc`):

```bash
# increase max number of file descriptors for running a sourcegraph instance.
ulimit -n 10000
```

On Linux, it may also be necessary to increase `sysctl -n fs.inotify.max_user_watches`, which can be done by running one of the following:

```bash
echo 524288 | sudo tee -a /proc/sys/fs/inotify/max_user_watches

# If the above doesn't work, you can also try this:
sudo sysctl fs.inotify.max_user_watches=524288
```

If you ever need to wipe your local database and Redis, run the following command.

```bash
./dev/drop-entire-local-database-and-redis.sh
```

## Caddy 2 certificate problems

We use Caddy 2 to setup HTTPS for local development. It creates self-signed certificates and uses that to serve the local Sourcegraph instance. If your browser complains about the certificate, check the following:

1. The first time that Caddy 2 reverse-proxies your Sourcegraph instance, it needs to add its certificate authority to your local certificate store. This may require elevated permissions on your machine. If you haven't done so already, try running `./dev/caddy.sh reverse-proxy --to localhost:3080` and enter your password if prompted. You may also need to run that command as the `root` user.

1. If you have completed the previous step and your browser still complains about the certificate, try restarting your browser or your local machine.

### Adding Caddy certificates to Windows

When running Caddy on WSL, you need to manually add the Caddy root certificate to the Windows certificate store using [certutil.exe](https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/certutil).

```bash
# Run inside WSL
certutil.exe -addstore -user Root "$(find /usr/local/share/ca-certificates/ -name '*Caddy*')"
```

This command will add the certificate to the `Trusted Root Certification Authorities` for your Windows user.

### Enabling Caddy certificates in Firefox

Firefox on Windows and macOS will not look for enterprise roots by default. If you are getting certificate errors in Firefox only, toggling `security.enterprise_roots.enabled` on in `about:config` should fix the problem.

### Certificate expiry

If you see a certificate expiry warning you may need to delete your certificate and restart your server.

On macOS, the certificate can be removed from here: `~/Library/Application\ Support/Caddy/certificates/local/sourcegraph.test`

## Running out of disk space

If you see errors similar to this:

```
gitserver | ERROR cleanup: error freeing up space, error: only freed 1124101958 bytes, wanted to free 29905298227
```

You are probably low on disk space. By default it tries to cleanup when there is less than 10% of available disk space.
You can override that by setting this env variable:

```bash
# means 5%. You may want to put that into .bashrc for convinience
SRC_REPOS_DESIRED_PERCENT_FREE=5
```

## CPU/RAM/bandwidth/battery usage

On first install, the program will use quite a bit of bandwidth to concurrently download all the Go and Node packages. After packages have been installed, the Javascript assets will be compiled into a single Javascript file, which can take up to 5 minutes, and can be heavy on the CPU at times.

After the initial install/compile is complete, the Docker for Mac binary uses about 1.5GB of RAM. The numerous different Go binaries don't use that much RAM or CPU each, about 5MB of RAM each.

If you're running macOS 10.15.x (Catalina) reinstalling the Xcode Command Line Tools may be necessary as follows:

1. Uninstall the Command Line Tools with `rm -rf /Library/Developer/CommandLineTools`
2. Reinstall it with `xcode-select --install`
3. Go to `sourcegraph/sourcegraph`’s root directory and run `rm -rf node_modules`
3. Re-start the dev environment (`sg start`)

## Permission errors for Grafana and Prometheus containers

The Grafana and Prometheus containers need group read access for specific files. Otherwise, you will see errors such as:

```
grafana | standard_init_linux.go:228: exec user process caused: permission denied
prometheus | standard_init_linux.go:228: exec user process caused: permission denied
```

or

```
prometheus | t=2021-05-26T10:05:26+0000 lvl=eror msg="command [/prometheus.sh --web.listen-address=0.0.0.0:9092] exited: fork/exec /prometheus.sh: permission denied" cmd=prom-wrapper
prometheus | t=2021-05-26T10:05:26+0000 lvl=eror msg="command [/alertmanager.sh --config.file=/sg_config_prometheus/alertmanag
```

If files do not normally have group permissions in your environment (e.g. if you set `umask 077`), then you need to:

1. Set group read permissions for the Grafana and Prometheus docker images, with

   ```sh
   chmod -R g=rX docker-images/{grafana,prometheus}
   ```

2. Run `sg start monitoring` so that new files are group readable. If you have `umask`
   set to a different value you can change that value just for this command by
   starting it in a subshell:

   ```sh
   (umask 027; sg start monitoring)
   ```

## Installing `sg` with Windows Subsystem for Linux (WSL2)

When trying to install `sg` with the pre-built binaries on WSL2 you may run into this error message: `failed to set max open files: invalid argument`. The default configuration of WSL2 does not allow the user to modify the number of open files by default [which `sg` requires](https://github.com/sourcegraph/sourcegraph/blob/379369e3d92c9b28d5891d3251922c7737ed810b/dev/sg/main.go#L75:L90) to start. To work around this you can modify the file limits for your given session with `sudo prlimit --nofile=20000 --pid $$; ulimit -n 20000` then re-run the installation script.

Note: this change will be reverted when your session ends. You will need to reset these limits every time you open a new session and want to use `sg`.

