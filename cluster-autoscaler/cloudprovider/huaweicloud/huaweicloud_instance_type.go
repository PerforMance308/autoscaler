package huaweicloud

import "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huawei-cloud-sdk-go/openstack/cce/v3/clusters"

type instanceType struct {
	InstanceTypeID string
	CPU           int64
	MemoryInGB  int64
	GPU           int64
}


func buildInstanceType(nodePool clusters.NodePool) *instanceType {
	return &instanceType{
		InstanceTypeID: nodePool.Spec.NodeTemplate.Flavor,
		CPU: 4,
		MemoryInGB: 4,
		GPU: 0,
	}
}