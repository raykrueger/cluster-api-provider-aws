# CloudFormation Templates

Templates in this directory are development artifacts for CAPA E2E testing and
manual validation. They are not part of the `clusterawsadm bootstrap` flow.

## EKS Auto Mode Node Role

`eks-automode-node-role.yaml` provisions an IAM role for EKS Auto Mode nodes.
This role will be folded into the main `clusterawsadm bootstrap` CloudFormation
template in a follow-up.

Deploy:

```bash
aws cloudformation deploy \
  --template-file hack/cloudformation/eks-automode-node-role.yaml \
  --stack-name capa-eks-automode-dev \
  --capabilities CAPABILITY_IAM
```

Retrieve the role ARN:

```bash
aws cloudformation describe-stacks \
  --stack-name capa-eks-automode-dev \
  --query 'Stacks[0].Outputs[?OutputKey==`NodeRoleArn`].OutputValue' \
  --output text
```
