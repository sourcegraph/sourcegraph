# How to setup HTTPS connection with Ingress controller on your Kubernetes instance

This document will take you through how to setup HTTPS connection using the preinstalled [Ingress controller](../deploy/kubernetes/configure.md#ingress-controller), which allows external users to access your main web server over the network. It installs rules for the default ingress, see comments to restrict it to a specific host. This is our recommended method to configure network access for production environments.

## Prerequisites

- This document assumes that your Sourcegraph instance is deployed into a Kubernetes cluster and that ingress has already been installed for [sourcegraph-frontend](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/base/frontend/sourcegraph-frontend.Ingress.yaml) (by default).

## Steps for GCE-GKE user

> WARNING: Please visit our [Kubernetes Configuration Docs](../deploy/kubernetes/configure.md#ingress-controller) for more detail on Network-related topics
> 

### 1. Install the NGINX ingress controller (ingress-nginx) 
Install the NGINX ingress controller by following the instructions at [https://kubernetes.github.io/ingress-nginx/deploy/](https://kubernetes.github.io/ingress-nginx/deploy/)

For example, GCE-GKE user would simply run [this command](https://kubernetes.github.io/ingress-nginx/deploy/#gce-gke) `kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v0.47.0/deploy/static/provider/cloud/deploy.yaml` to install the NGINX ingress controller

### 2. Update the create-new-cluster.sh file
Add the [configure/ingress-nginx/install.sh](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/configure/ingress-nginx/install.sh) command to the [create-new-cluster.sh](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/create-new-cluster.sh) file at root, and commit the change. 
Your file should look similar to this:
```sh
echo ./configure/ingress-nginx/install.sh >> create-new-cluster.sh
./kubectl-apply-all.sh $@
```

### 3. Once the ingress has acquired an external address
You should be able to access Sourcegraph using the external address returns from the following `kubectl -n ingress-nginx get svc`.

```bash
$kubectl -n ingress-nginx get svc
NAME                                 TYPE           CLUSTER-IP    EXTERNAL-IP     PORT(S)                      AGE
ingress-nginx-controller             LoadBalancer   10.XX.8.XXX   XX.XXX.XXX.XX   80:32695/TCP,443:31722/TCP   5d13h
ingress-nginx-controller-admission   ClusterIP      10.XX.8.X     <none>          443/TCP                      5d13h
```

## Configure TLS/SSL 

After your Sourcegraph instance is exposed via an ingress controller, you should consider using TLS so that all traffic will be served over HTTPS.

### 1. Create TLS certificate and private key

Place the newly created certificate and private key in a secured place. We will be using `.envrc/private.key` and `.envrc/public.pem` in this example.

### 2. Create a TLS secret for your Cluster

Create a TLS secret that contains your TLS certificate and private key by running the following command:

```bash
kubectl create secret tls sourcegraph-tls --key .envrc/private.key --cert .envrc/public.pem
```

> NOTE: You can delete it by running `kubectl delete secret sourcegraph-tls`

### 3. Update the create-new-cluster.sh file 

Add the previous command to the [create-new-cluster.sh](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/create-new-cluster.sh) file at root, and commit the change. Your file should look similar to this:

```bash
echo ./configure/ingress-nginx/install.sh >> create-new-cluster.sh
echo kubectl create secret tls sourcegraph-tls --key .envrc/private.key --cert .envrc/public.pem  >> create-new-cluster.sh
./kubectl-apply-all.sh $@
```

### 4. Update the ingress sourcegraph-frontend.Ingress.yaml file

Add the tls configuration to [base/frontend/sourcegraph-frontend.Ingress.yaml](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/base/frontend/sourcegraph-frontend.Ingress.yaml) file by commenting out the `tls` section, and replace `sourcegraph.example.com` with your domain.

> NOTE: It must be a DNS name, not an IP address

```yaml
   # base/frontend/sourcegraph-frontend.Ingress.yaml
   tls:
     - hosts:
         #  Replace 'sourcegraph.example.com' with the real domain that you want to use for your Sourcegraph instance.
         - sourcegraph.example.com
       secretName: sourcegraph-tls
   rules:
     - http:
         paths:
         - path: /
           backend:
             serviceName: sourcegraph-frontend
             servicePort: 30080
       # Replace 'sourcegraph.example.com' with the real domain that you want to use for your Sourcegraph instance.
       host: sourcegraph.example.com

```

### 5. Update Site Configuration

Update your externalURL in the [site configuration](https://docs.sourcegraph.com/admin/config/site_config) to e.g. https://sourcegraph.example.com:

```json
{
"externalURL": "https://sourcegraph.example.com"
}
```

### 6. Update the ingress controller

Update the ingress controller with the previous changes with the following command:

```bash
kubectl apply -f base/frontend/sourcegraph-frontend.Ingress.yaml
```
