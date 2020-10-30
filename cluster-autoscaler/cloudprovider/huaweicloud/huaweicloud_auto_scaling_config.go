package huaweicloud

import (
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/core/sdktime"
	huaweicloudsdkasmodel "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/services/as/v1/model"
)

type AutoScalingGroupConfig struct {
	configName string
	configID   string
	createTime *sdktime.SdkTime
	flavorRef  string
}

func newAutoScalingGroupConfig(cfg *huaweicloudsdkasmodel.ScalingConfiguration) *AutoScalingGroupConfig {
	return &AutoScalingGroupConfig{
		configName: *cfg.ScalingConfigurationName,
		configID:   *cfg.ScalingConfigurationId,
		createTime: cfg.CreateTime,
		flavorRef:  *cfg.InstanceConfig.FlavorRef,
	}
}
