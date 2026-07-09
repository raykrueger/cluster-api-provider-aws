# EKS Auto Mode

EKS Auto Mode is an AWS-managed compute capability that simplifies cluster operations by automatically managing compute resources. When enabled, EKS Auto Mode handles node group provisioning, scaling, and lifecycle management.

## Enabling Auto Mode

To enable EKS Auto Mode, set the `autoMode` field in your `AWSManagedControlPlane` spec:

```yaml
apiVersion: controlplane.cluster.x-k8s.io/v1beta2
kind: AWSManagedControlPlane
metadata:
  name: my-cluster-control-plane
spec:
  region: us-west-2
  version: "1.31"
  identityRef:
    kind: AWSClusterStaticIdentity
    name: e2e-account
  autoMode:
    mode: Enabled
    compute:
      nodePools:
        - general-purpose
      nodeRoleARN: "arn:aws:iam::123456789012:role/my-eks-node-role"
```

### Required fields

- `mode`: Must be `Enabled` to enable Auto Mode.
- `compute.nodePools`: List of node pool types. Supported values:
  - `general-purpose`: For general-purpose workloads.
  - `system`: For system-critical workloads.
- `compute.nodeRoleARN`: The IAM role ARN that Auto Mode nodes will assume. This role must have the `AmazonEKSWorkerNodePolicy`, `AmazonEKS_CNI_Policy`, and `AmazonEC2ContainerRegistryReadOnly` policies attached.

### Automatic capabilities

When Auto Mode is enabled, the following capabilities are automatically enabled on the EKS cluster:

- **Block Storage**: EBS volume management for persistent storage.
- **Elastic Load Balancing**: Application and Network Load Balancer support.

These capabilities cannot be configured independently and are always enabled alongside Auto Mode.

## Disabling Auto Mode

To disable Auto Mode, set `mode` to `Disabled`:

```yaml
apiVersion: controlplane.cluster.x-k8s.io/v1beta2
kind: AWSManagedControlPlane
metadata:
  name: my-cluster-control-plane
spec:
  autoMode:
    mode: Disabled
```

When Auto Mode is disabled, the following also become disabled:
- Block Storage capability
- Elastic Load Balancing capability

## Node Role

The `nodeRoleARN` specifies the IAM role that Auto Mode nodes assume. This role requires the following managed policies:

- `AmazonEKSWorkerNodePolicy`
- `AmazonEKS_CNI_Policy`
- `AmazonEC2ContainerRegistryReadOnly`

A CloudFormation template is provided in `hack/cloudformation/eks-automode-node-role.yaml` for creating a suitable role for development and testing.

## Limitations

- Auto Mode cannot be enabled on an existing cluster that has self-managed node groups.
- Auto Mode node pools are managed by AWS and cannot be modified directly through CAPA.
- When Auto Mode is enabled, `ControlPlaneMachineCount` in cluster templates should be set to `1` (clusterctl requires a non-zero value), but no actual self-managed control plane nodes will be created.
