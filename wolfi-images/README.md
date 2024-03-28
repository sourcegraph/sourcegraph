Wolfi base images - these are used as base images by our Dockerfiles in `cmd/` and `docker-images/`.

- `sourcegraph`: minimal image, used by simple images that don't have any additional dependencies.
- `sourcegraph-dev`: contains additional tooling making it useful for development; not for production use.
- Other images are bases for specific Dockerfiles, and include the packages required by each container.
Hello World
