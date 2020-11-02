package huaweicloud

import (
	"strconv"

	huaweicloudsdkecs "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	huaweicloudsdkecsmodel "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
	"k8s.io/klog/v2"
)

type ecsWrapper struct {
	ecsInstance func() *huaweicloudsdkecs.EcsClient
}

type instanceType struct {
	name string
	vcpu int64
	ram  int64
	gpu  int64
}

func newEcsWrapper(cloudConfig *CloudConfig) *ecsWrapper {
	return &ecsWrapper{
		ecsInstance: cloudConfig.getECSClient,
	}
}

func newInstanceType(flavor *huaweicloudsdkecsmodel.Flavor) *instanceType {
	vcpus, _ := strconv.ParseInt(flavor.Vcpus, 10, 64)
	return &instanceType{
		name: flavor.Name,
		vcpu: vcpus,
		ram:  int64(flavor.Ram),
		gpu:  0,
	}
}

func (e *ecsWrapper) listFlavors(az string) ([]*instanceType, error) {
	ecsClient := e.ecsInstance()
	opts := &huaweicloudsdkecsmodel.ListFlavorsRequest{
		AvailabilityZone: &az,
	}
	response, err := ecsClient.ListFlavors(opts)
	if err != nil {
		klog.Errorf("failed to list flavors. availability zone: %s", az)
		return nil, err
	}

	instanceTypes := make([]*instanceType, 0, len(*response.Flavors))
	for _, flavor := range *response.Flavors {
		instanceType := newInstanceType(&flavor)
		instanceTypes = append(instanceTypes, instanceType)
	}
	return instanceTypes, nil
}
