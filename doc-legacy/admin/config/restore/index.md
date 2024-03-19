# Restoring Postgres databases in Kubernetes

_Expected use case:_ Restoring a previous snapshot for disaster recovery

For restoring Postgres databases in our Kubernetes deployments you should have backups of both the primary db and the codeintel-db. The primary db contains all user information.

To restore persistent volumes you need to

1. Understand how your Kubernetes cluster provides Persistent Volume (PV) Storage
   [GKE](https://cloud.google.com/kubernetes-engine/docs/concepts/persistent-volumes)
   , [EKS](https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html)
   , [AKS](https://docs.microsoft.com/en-us/azure/aks/concepts-storage)
1. Scale down all pods that are using the PersistentVolumeClaim (PVC)
1. Backup the current PVCs
1. Create new PVs that reference the restored snapshots

## Steps

1. Backup the PVCs we will be operating on
   ```
   kubectl get pvc pgsql codeintel-db codeinsights-db -oyaml > backupPVC.yaml
   ```
1. Ensure that you have snapshots of the necessary PVs (this depends on your deployment environment)
   ```
   kubectl get pv $(kubectl get pvc pgsql codeintel-db codeinsights-db -o=jsonpath='{.items[*].spec.volumeName}')
   ```
1. Restore the snapshots to new disks that meet the requirements to be consumed by your cluster (same region, etc).
1. Scale down all deployments to zero (note: both approaches involve downtime, the first approach is preferred)
   ```
   kubectl scale deployment  -l deploy=sourcegraph --replicas=0
   ```
   or (if you are unable to scale deployments to their previous values easily)
   ```
    kubectl scale deployment pgsql codeinsights-db codeintel-db  --replicas=0
   ```

1. Next, you need to replace the existing PVC and PV.

  1. First, delete the PVC `kubectl delete pvc pgsql`
  1. Then create the new PV & PVC. The options used to configure the PV will be specific to every provider.
   ```yaml
   apiVersion: v1
   kind: PersistentVolume
   metadata:
     name: example-pgsql-volume
   spec:
     storageClassName: "sourcegraph"
     accessModes:
       - ReadWriteOnce
     capacity:
       storage: 200Gi
     claimRef:
       namespace: ns-sourcegraph
       name: pgsql
     # You will need to customize the fields below most likely
     gcePersistentDisk:
       pdName: disk-restored-from-snapshot
       fsType: ext4
   ---
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata:
     name: pgsql
     namespace: ns-sourcegraph
   spec:
     storageClassName: "sourcegraph"
     accessModes:
       - ReadWriteOnce
     resources:
       requests:
         storage: 200Gi
   ```
  1. Ensure PVCs become Bound `kubectl get pvc -w`

1. (Optional) Commit PVs and PVCs back to deployment repo into the `base/${deployment}` directory.

1. Scale deployments back up
1. If you use our deployment scripts, after committing the change you may run `./kubectl-apply-all.sh` again to restore
   all deployments to their set number of replicas.
1. Otherwise, `kubectl scale deployment pgsql codeinsights-db codeintel-db --replicas=1`

1. After ensuring Sourcegraph is functional, you may delete the previous PVs. If the **Reclaim Policy** is not set to 
   **Delete** you will need to manually delete the disk from your provider as well. 

  
   
