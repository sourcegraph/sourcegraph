# `src debug`



## Usage

```
'src debug' gathers and bundles debug data from a Sourcegraph deployment for troubleshooting.

Usage:

	src debug command [command options]

The commands are:

	kube                 dumps context from k8s deployments
	compose              dumps context from docker-compose deployments
	server               dumps context from single-container deployments
	

Use "src debug command -h" for more information about a subcommands.
src debug has access to flags on src -- Ex: src -v kube -o foo.zip



```
	