package huaweicloud

import (
	"strconv"

	huaweicloudsdkecsmodel "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
)

type InstanceType struct {
	Name string
	VCPU int64
	RAM  int64
	GPU  int64
}

func newInstanceType(flavor *huaweicloudsdkecsmodel.Flavor) *InstanceType {
	vcpus, _ := strconv.ParseInt(flavor.Vcpus, 10, 64)
	return &InstanceType{
		Name: flavor.Name,
		VCPU: vcpus,
		RAM:  int64(flavor.Ram),
		GPU:  0,
	}
}
