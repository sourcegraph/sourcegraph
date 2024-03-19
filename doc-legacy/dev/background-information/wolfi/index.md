# Wolfi

Sourcegraph is in the process of migrating from Alpine-based Docker images to Wolfi-based images. This page covers the migration, and highlight changes to our images build process.

For information on how to build and update packages and base images, see [More Information](#more-information).

## What is Wolfi?

[Wolfi](https://github.com/wolfi-dev) is a stripped-down open Linux distro designed for cloud-native containers. It has several key features that make it a good fit for distributing Sourcegraph:

- A distroless build system that lets us build containers with only the precise dependencies we need, reducing attack surface area and the number of components that need to be kept up-to-date.
- A package repository that receives fast security patches and splits larger dependencies into smaller packages, keeping our images minimal and secure.
- High quality build-time SBOM (software bill of materials) support, allowing us to be transparent about our image composition.

## Why have we adopted Wolfi?

Adopting Wolfi helps significantly reduce the number of unpatched vulnerabilities in our container images, and move faster to patch them when new issues our found.

For full details, see [RFC 768: Harden container images by switching from Alpine to Wolfi](https://docs.google.com/document/d/1yQsXU7ekqPGjdkKItXKxROVNcJAYiGg3ZA70zJcLzIQ/edit#).

## How is the build process different from Alpine to Wolfi?

In our Alpine images, most of the work is done in the Dockerfile: packages are installed; third-party dependencies are fetched, built, and installed; users and directories are created; and binaries are copied.

When building Wolfi images, most of this work is done **outside** of the Dockerfile:

- All third-party dependencies are [packaged](../../how-to/wolfi/add_update_packages.md) as APKs (Alpine Packages, a format that Wolfi uses).
- Dependencies, user accounts, directory structure, and permissions are combined into a pre-built [base image](../../how-to/wolfi/add_update_images.md).
- The Dockerfile then only copies over Sourcegraph Go binaries, any configuration, and sets the entrypoint.

In short, all dependencies are pre-installed in a Wolfi base image. The Dockerfile is the final step that just adds on the Sourcegraph code.

## More Information

- [How to add and update Wolfi packages](../../how-to/wolfi/add_update_packages.md)
- [How to add and update Wolfi base images](../../how-to/wolfi/add_update_images.md)
