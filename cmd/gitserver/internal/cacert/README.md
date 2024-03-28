# cacert

This package exists to expose the internal code in the stdlib x509 package related to loading system certificates. Not all operating systems expose the system root certificates, and instead rely on a system call. However, in linux you can load the system root certificates.

To update run `update.bash`. It will copy the relevant code from your go installation and then patch it.

Note: This does mean this won't work on darwin. So for example our code which depends on this package will not have system certificates to use in our development environments.
Hello World
