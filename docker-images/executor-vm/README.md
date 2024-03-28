# sourcegraph/executor-vm

We produce a version of Ubuntu 20.04 (Focal Fossa), losely inspired by [weaveworks/ignite-ubuntu:20.04-amd64](https://github.com/weaveworks/ignite/blob/46bdd5d48425c4245fbe895e7da3621f491c3660/images/ubuntu/Dockerfile) that contains additional dependencies and config tweaks for the Sourcegraph executor fircracker VMs.

This image serves as the base image for [Firecracker](https://github.com/firecracker-microvm/firecracker) virtual machines in which we run user configured containers and code.
Hello World
