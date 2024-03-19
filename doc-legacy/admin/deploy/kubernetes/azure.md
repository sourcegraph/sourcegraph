# Sourcegraph with Kubernetes on Azure

> WARNING: This guide applies exclusively to a Kubernetes deployment **without** Helm.
> If you have not deployed Sourcegraph yet, it is higly recommended to use Helm as it simplifies the configuration and greatly simplifies the later upgrade process. See our guidance on [using Helm to deploy to Azure AKS](helm.md#configure-sourcegraph-on-azure-managed-kubernetes-service-aks).

Install the [Azure CLI tool](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest) and log in:

```
az login
```

Sourcegraph on Kubernetes requires at least **16 cores** in the **DSv3** family in the Azure location of your choice (e.g. `eastus`), so make sure you have enough available (if not, [request a quota increase](https://docs.microsoft.com/en-us/azure/azure-supportability/resource-manager-core-quotas-request)):

```
$ az vm list-usage -l eastus -o table
Name                                CurrentValue    Limit
--------------------------------  --------------  -------
...
Standard DSv3 Family vCPUs                     0       32
...
```

Ensure that these Azure service providers are enabled:

```
az provider register -n Microsoft.Network
az provider register -n Microsoft.Storage
az provider register -n Microsoft.Compute
az provider register -n Microsoft.ContainerService
```

Create a resource group:

```
az group create --name sourcegraphResourceGroup --location eastus
```

Create a cluster:

```
az aks create --resource-group sourcegraphResourceGroup --name sourcegraphCluster --node-count 1 --generate-ssh-keys --node-vm-size Standard_D16s_v3
```

Connect to the cluster for future `kubectl` commands:

```
az aks get-credentials --resource-group sourcegraphResourceGroup --name sourcegraphCluster
```

Follow the [Sourcegraph cluster installation instructions](configure.md#configure-a-storage-class) with `storageClass` set to `managed-premium` in `config.json`:

```diff
-    "storageClass": "default"
+    "storageClass": "managed-premium"
```

You can see if the pods are ready and check for installation problems through the Kubernetes dashboard:

```
az aks browse --resource-group sourcegraphResourceGroup --name sourcegraphCluster
```

Set up a load balancer to make the main web server accessible over the network to external users:

```
kubectl expose deployment sourcegraph-frontend --type=LoadBalancer --name=sourcegraphloadbalancer --port=80 --target-port=3080
```
