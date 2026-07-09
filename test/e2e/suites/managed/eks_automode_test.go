//go:build e2e
// +build e2e

/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package managed

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ekscontrolplanev1 "sigs.k8s.io/cluster-api-provider-aws/v2/controlplane/eks/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-aws/v2/pkg/cloud/awserrors"
	"sigs.k8s.io/cluster-api-provider-aws/v2/test/e2e/shared"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
)

// EKS Auto Mode e2e tests.
var _ = ginkgo.Describe("[managed] [automode] EKS Auto Mode tests", func() {
	var (
		namespace   *corev1.Namespace
		ctx         context.Context
		specName    = "automode"
		clusterName string
	)

	ginkgo.It("[managed] [automode] should create an EKS cluster with Auto Mode enabled and update to disabled", func() {
		ginkgo.By("should have a valid test configuration")
		Expect(e2eCtx.Environment.BootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. BootstrapClusterProxy can't be nil")
		Expect(e2eCtx.E2EConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(e2eCtx.E2EConfig.Variables).To(HaveKey(shared.KubernetesVersion))

		nodeRoleARN, ok := e2eCtx.E2EConfig.Variables[shared.EksAutoModeNodeRoleARN]
		Expect(ok).To(BeTrue(), "EKS_AUTO_MODE_NODE_ROLE_ARN must be set in the test configuration")
		Expect(nodeRoleARN).ToNot(BeEmpty(), "EKS_AUTO_MODE_NODE_ROLE_ARN must not be empty")

		ctx = context.TODO()
		namespace = shared.SetupSpecNamespace(ctx, specName, e2eCtx)
		clusterName = fmt.Sprintf("%s-%s", specName, util.RandomString(6))
		eksClusterName := getEKSClusterName(namespace.Name, clusterName)

		ginkgo.By("should create an EKS cluster with Auto Mode enabled")
		ManagedClusterSpec(ctx, func() ManagedClusterSpecInput {
			return ManagedClusterSpecInput{
				E2EConfig:                e2eCtx.E2EConfig,
				ConfigClusterFn:          defaultConfigCluster,
				BootstrapClusterProxy:    e2eCtx.Environment.BootstrapClusterProxy,
				AWSSession:               e2eCtx.BootstrapUserAWSSession,
				Namespace:                namespace,
				ClusterName:              clusterName,
				Flavour:                  EKSAutoModeFlavor,
				ControlPlaneMachineCount: 1,
				WorkerMachineCount:       0,
			}
		})

		ginkgo.By("EKS cluster should be active")
		verifyClusterActiveAndOwned(ctx, eksClusterName, e2eCtx.BootstrapUserAWSSession)

		ginkgo.By("Auto Mode should be enabled on the EKS cluster")
		WaitForEKSClusterAutoMode(ctx, e2eCtx.BootstrapUserAWSSession, eksClusterName, true)

		ginkgo.By("should update Auto Mode from Enabled to Disabled")
		controlPlaneName := fmt.Sprintf("%s-control-plane", clusterName)
		controlPlane := &ekscontrolplanev1.AWSManagedControlPlane{}
		Eventually(func() error {
			return e2eCtx.Environment.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKey{
				Namespace: namespace.Name,
				Name:      controlPlaneName,
			}, controlPlane)
		}, e2eCtx.E2EConfig.GetIntervals("", "wait-client-request")...).Should(Succeed(), "failed to get control plane")

		patchHelper, err := patch.NewHelper(controlPlane, e2eCtx.Environment.BootstrapClusterProxy.GetClient())
		Expect(err).ToNot(HaveOccurred())
		controlPlane.Spec.AutoMode.Mode = ekscontrolplanev1.AutoModeStateDisabled

		Eventually(func() error {
			return patchHelper.Patch(ctx, controlPlane)
		}, e2eCtx.E2EConfig.GetIntervals("", "wait-client-request")...).Should(Succeed(), "failed to patch control plane")

		ginkgo.By("Auto Mode should be disabled on the EKS cluster")
		WaitForEKSClusterAutoMode(ctx, e2eCtx.BootstrapUserAWSSession, eksClusterName, false)

		cluster := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
			Getter:    e2eCtx.Environment.BootstrapClusterProxy.GetClient(),
			Namespace: namespace.Name,
			Name:      clusterName,
		})
		Expect(cluster).NotTo(BeNil(), "couldn't find CAPI cluster")

		framework.DeleteCluster(ctx, framework.DeleteClusterInput{
			Deleter: e2eCtx.Environment.BootstrapClusterProxy.GetClient(),
			Cluster: cluster,
		})
		framework.WaitForClusterDeleted(ctx, framework.WaitForClusterDeletedInput{
			ClusterProxy:         e2eCtx.Environment.BootstrapClusterProxy,
			Cluster:              cluster,
			ClusterctlConfigPath: e2eCtx.Environment.ClusterctlConfigPath,
			ArtifactFolder:       e2eCtx.Settings.ArtifactFolder,
		}, e2eCtx.E2EConfig.GetIntervals("", "wait-delete-cluster")...)
	})
})

// WaitForEKSClusterAutoMode polls the AWS EKS API until the cluster's Auto Mode
// state matches the expected value, failing early if the cluster is not found.
func WaitForEKSClusterAutoMode(ctx context.Context, sess *aws.Config, eksClusterName string, expectedEnabled bool) {
	ginkgo.By(fmt.Sprintf("Checking EKS control plane Auto Mode is %v", expectedEnabled))
	Eventually(func() error {
		cluster, err := getEKSCluster(ctx, eksClusterName, sess)
		if err != nil {
			smithyErr := awserrors.ParseSmithyError(err)
			notFoundErr := &ekstypes.ResourceNotFoundException{}
			if smithyErr.ErrorCode() == notFoundErr.ErrorCode() {
				return StopTrying(fmt.Sprintf("unrecoverable error: cluster %q not found: %s", eksClusterName, smithyErr.ErrorMessage()))
			}
			return err
		}

		if cluster.ComputeConfig == nil {
			return fmt.Errorf("cluster %q has no ComputeConfig", eksClusterName)
		}

		actualEnabled := aws.ToBool(cluster.ComputeConfig.Enabled)
		if actualEnabled != expectedEnabled {
			return fmt.Errorf("auto mode mismatch: expected %v, but found %v", expectedEnabled, actualEnabled)
		}

		return nil
	}, 10*time.Minute, 10*time.Second).Should(Succeed(), fmt.Sprintf("eventually failed checking EKS Cluster %q auto mode is %v", eksClusterName, expectedEnabled))
}
