package huaweicloud

import (
	"fmt"

	huaweicloudsdkas "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/services/as/v1"
	huaweicloudsdkasmodel "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/services/as/v1/model"
	"k8s.io/klog/v2"
)

type autoScalingWrapper struct {
	autoScaling func() *huaweicloudsdkas.AsClient
}

func newAutoScalingWrapper(cloudConfig *CloudConfig) *autoScalingWrapper {
	return &autoScalingWrapper{
		autoScaling: cloudConfig.getASClient,
	}
}

func (s *autoScalingWrapper) getScalingInstancesByGroup(groupID string) (*[]huaweicloudsdkasmodel.ScalingGroupInstance, error) {
	asClient := s.autoScaling()
	opts := &huaweicloudsdkasmodel.ListScalingInstancesRequest{
		ScalingGroupId: groupID,
	}
	response, err := asClient.ListScalingInstances(opts)
	if err != nil {
		klog.Errorf("failed to show scaling group info. group: %s, error: %v", groupID, err)
		return nil, err
	}
	if response == nil || response.ScalingGroupInstances == nil {
		return nil, nil
	}
	return response.ScalingGroupInstances, nil
}

func (s *autoScalingWrapper) listScalingGroup() (*[]huaweicloudsdkasmodel.ScalingGroups, error) {
	asClient := s.autoScaling()
	requiredState := huaweicloudsdkasmodel.GetListScalingGroupsRequestScalingGroupStatusEnum().INSERVICE
	opts := &huaweicloudsdkasmodel.ListScalingGroupsRequest{
		ScalingGroupStatus: &requiredState,
	}

	response, err := asClient.ListScalingGroups(opts)
	if err != nil {
		klog.Errorf("failed to list scaling groups. error: %v", err)
		return nil, err
	}

	if response == nil || response.ScalingGroups == nil {
		klog.Info("no scaling group yet.")
		return nil, nil
	}

	return response.ScalingGroups, nil
}

func (s *autoScalingWrapper) getScalingGroupByID(groupID string) (*huaweicloudsdkasmodel.ScalingGroups, error) {
	asClient := s.autoScaling()
	opts := &huaweicloudsdkasmodel.ShowScalingGroupRequest{
		ScalingGroupId: groupID,
	}
	response, err := asClient.ShowScalingGroup(opts)
	if err != nil {
		klog.Errorf("failed to show scaling group info. group: %s, error: %v", groupID, err)
		return nil, err
	}
	if response == nil || response.ScalingGroup == nil {
		return nil, fmt.Errorf("no scaling group found: %s", groupID)
	}

	return response.ScalingGroup, nil
}

func (s *autoScalingWrapper) scaleUpCluster(groupID string, size int) error {
	asClient := s.autoScaling()
	desireInstanceNumber := int32(size)
	opts := &huaweicloudsdkasmodel.UpdateScalingGroupRequest{
		ScalingGroupId: groupID,
		Body: &huaweicloudsdkasmodel.UpdateScalingGroupRequestBody{
			DesireInstanceNumber: &desireInstanceNumber,
		},
	}

	_, err := asClient.UpdateScalingGroup(opts)
	if err != nil {
		klog.Errorf("failed to scaling up cluster. group: %s, error: %v", groupID, err)
		return err
	}
	return nil
}

func (s *autoScalingWrapper) scaleDownCluster(groupID string, instanceIds []string) error {
	asClient := s.autoScaling()
	instanceDelete := "yes"

	opts := &huaweicloudsdkasmodel.UpdateScalingGroupInstanceRequest{
		ScalingGroupId: groupID,
		Body: &huaweicloudsdkasmodel.UpdateScalingGroupInstanceRequestBody{
			InstancesId:    instanceIds,
			InstanceDelete: &instanceDelete,
			Action:         huaweicloudsdkasmodel.GetUpdateScalingGroupInstanceRequestBodyActionEnum().REMOVE,
		},
	}

	_, err := asClient.UpdateScalingGroupInstance(opts)

	if err != nil {
		klog.Errorf("failed to scaling down cluster. group: %s, error: %v", groupID, err)
		return err
	}

	return nil
}

func (s *autoScalingWrapper) getInstances(groupID string) ([]huaweicloudsdkasmodel.ScalingGroupInstance, error) {
	asClient := s.autoScaling()

	opts := &huaweicloudsdkasmodel.ListScalingInstancesRequest{
		ScalingGroupId: groupID,
	}
	response, err := asClient.ListScalingInstances(opts)
	if err != nil {
		klog.Errorf("failed to list scaling group instances. group: %s, error: %v", groupID, err)
		return nil, err
	}
	if response == nil || response.ScalingGroupInstances == nil {
		return nil, fmt.Errorf("no instance in scaling group: %s", groupID)
	}

	return *response.ScalingGroupInstances, nil
}

func (s *autoScalingWrapper) getScalingGroupConfigByID(groupID, configID string) (*huaweicloudsdkasmodel.ScalingConfiguration, error) {
	asClient := s.autoScaling()
	opts := &huaweicloudsdkasmodel.ShowScalingConfigRequest{
		ScalingConfigurationId: configID,
	}
	response, err := asClient.ShowScalingConfig(opts)
	if err != nil {
		klog.Errorf("failed to show scaling group config. config id: %s, error: %v", configID, err)
		return nil, err
	}
	if response == nil || response.ScalingConfiguration == nil {
		return nil, fmt.Errorf("no instance in scaling group: %s", groupID)
	}
	return response.ScalingConfiguration, nil
}
