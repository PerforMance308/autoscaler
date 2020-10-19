package huaweicloud

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huawei-cloud-sdk-go/openstack/cce/v3/clusters"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huawei-cloud-sdk-go/openstack/ecs"
	"k8s.io/klog/v2"
	"strconv"
	"strings"
)

type instanceType struct {
	InstanceTypeID string
	CPU           int64
	MemoryMB      int64
	GPU           int64
}


func buildInstanceType(mgr *huaweicloudCloudManager, nodePool clusters.NodePool) *instanceType {
	flavors, err := ecs.GetECSFlavors(mgr.ecsClient, nodePool.Spec.NodeTemplate.Az).Extract()
	if err != nil {
		klog.Errorf("failed to get ecs flavors, error: %v", err)
	}

	for _, flavor := range flavors {
		if strings.EqualFold(nodePool.Spec.NodeTemplate.Flavor, flavor.Name) {
			cpus, _ := strconv.ParseInt(flavor.VCPU, 10, 64)
			return &instanceType{
				InstanceTypeID: flavor.Name,
				CPU: cpus,
				MemoryMB: int64(flavor.Ram),
				GPU: 0,
			}
		}
	}

	return &instanceType{}
}