## Local development environment with [tilt.dev](https://tilt.dev) and [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)

> This is still a work in progress. `tilt` is useful when you have to develop k8s code and need a local cluster for your edit/compile/test cycle.
> `tilt` can also be used with [bare processes](https://blog.tilt.dev/2020/02/12/local-dev.html) but we haven't converted and optimized our build
> pipeline to it. The `tilt` team have given us a [starting point](https://github.com/windmilleng/sourcegraph/blob/master/Tiltfile) (thanks!).
> For now, (until we convert, test and optimize this approach) please still use `sg start`.

(Instructions assume you are in this directory)

- Install [tilt](https://docs.tilt.dev/install.html)
- Install [minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)
- Generate manifests from the [minikube overlay](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/overlays/minikube) and copy the `generated-cluster` directory into this directory.
- `mkdir tilt-watch-targets`
- Prepare the cluster:

  ```shell
  minikube start
  kubectl create namespace ns-sourcegraph
  kubectl -n ns-sourcegraph apply --prune -l deploy=sourcegraph -f generated-cluster --recursive
  kubectl -n ns-sourcegraph expose deployment sourcegraph-frontend --type=NodePort --name sourcegraph --port=3080 --target-port=3080
  minikube service list
  ```

- From the `minikube service list` output take the exposed port and modify the Caddyfile.
- `caddy run` (this makes Sourcegraph from the minikube cluster available at https://sourcegraph.test:3443)
- `tilt up` (starts tilt)

Tilt will start an initial build of the frontend and deploy it. Whenever you are done editing and want to trigger another build, `touch tilt-watch-targets/frontend`.

Similar to the `frontend` you can add other `custom_build` statements to your `Tiltfile` to build and watch the other servers.

There are many `Tiltfile` modifications that can help accelerate the edit/build/deploy cycle: https://docs.tilt.dev/example_go.html
Hello World
