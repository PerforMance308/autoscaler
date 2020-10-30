/*
Copyright 2020 The Kubernetes Authors.

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

package huaweicloud

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	huaweicloudsdkas "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/services/as/v1"
	huaweicloudsdkasmodel "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/services/as/v1/model"
	huaweicloudsdkecs "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	huaweicloudsdkecsmodel "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"
	"k8s.io/autoscaler/cluster-autoscaler/utils/gpu"
	"k8s.io/klog/v2"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"
)

// ElasticCloudServerService represents the elastic cloud server interfaces.
// It should contains all request against elastic cloud server service.
type ElasticCloudServerService interface {
	// DeleteServers deletes a group of server by ID.
	DeleteServers(serverIDs []string) error
	// List all instance type by availablity zone
	ListFlavors(az string) ([]*InstanceType, error)
}

// AutoScalingService represents the auto scaling service interfaces.
// It should contains all request against auto scaling service.
type AutoScalingService interface {
	// ListScalingGroups list all scaling groups.
	ListScalingGroups() ([]AutoScalingGroup, error)

	// GetDesireInstanceNumber gets the desire instance number of specific auto scaling group.
	GetDesireInstanceNumber(groupID string) (int, error)

	// GetInstances gets the instances in an auto scaling group.
	GetInstances(groupID string) ([]cloudprovider.Instance, error)

	// IncreaseSizeInstance increases the instance number of specific auto scaling group.
	// The delta should be non-negative.
	// IncreaseSizeInstance wait until instance number is updated.
	IncreaseSizeInstance(asg *AutoScalingGroup, delta int) error

	// ShowAsgConfig returns auto scaling group config
	ShowAsgConfig(cfgID string) (*AutoScalingGroupConfig, error)
}

type internalService interface {
	// Get default auto scaling group template
	getAsgTemplate(groupID string) (*asgTemplate, error)

	// buildNodeFromTemplate returns template from instance flavor
	buildNodeFromTemplate(asgName string, template *asgTemplate) (*apiv1.Node, error)
}

// CloudServiceManager represents the cloud service interfaces.
// It should contains all requests against cloud services.
type CloudServiceManager interface {
	// ElasticCloudServerService represents the elastic cloud server interfaces.
	ElasticCloudServerService

	// AutoScalingService represents the auto scaling service interfaces.
	AutoScalingService

	// internalService is used for internal use only
	internalService
}

type cloudServiceManager struct {
	cloudConfig      *CloudConfig
	getECSClientFunc func() *huaweicloudsdkecs.EcsClient
	getASClientFunc  func() *huaweicloudsdkas.AsClient
}

type asgTemplate struct {
	InstanceType *InstanceType
	Region       string
	Zone         string
	Tags         map[string]string
}

func newCloudServiceManager(cloudConfig *CloudConfig) *cloudServiceManager {
	return &cloudServiceManager{
		cloudConfig:      cloudConfig,
		getECSClientFunc: cloudConfig.getECSClient,
		getASClientFunc:  cloudConfig.getASClient,
	}
}

// DeleteServers deletes a group of server by ID.
func (csm *cloudServiceManager) DeleteServers(serverIDs []string) error {
	ecsClient := csm.getECSClientFunc()
	if ecsClient == nil {
		return fmt.Errorf("failed to delete servers due to can not get ecs client")
	}

	servers := make([]huaweicloudsdkecsmodel.ServerId, 0, len(serverIDs))
	for i := range serverIDs {
		s := huaweicloudsdkecsmodel.ServerId{
			Id: serverIDs[i],
		}
		servers = append(servers, s)
	}

	deletePublicIP := false
	deleteVolume := false
	opts := &huaweicloudsdkecsmodel.DeleteServersRequest{
		Body: &huaweicloudsdkecsmodel.DeleteServersRequestBody{
			DeletePublicip: &deletePublicIP,
			DeleteVolume:   &deleteVolume,
			Servers:        servers,
		},
	}
	deleteResponse, err := ecsClient.DeleteServers(opts)
	if err != nil {
		return fmt.Errorf("failed to delete servers. error: %v", err)
	}
	jobID := deleteResponse.JobId

	err = wait.Poll(5*time.Second, 300*time.Second, func() (bool, error) {
		showJobOpts := &huaweicloudsdkecsmodel.ShowJobRequest{
			JobId: *jobID,
		}
		showJobResponse, err := ecsClient.ShowJob(showJobOpts)
		if err != nil {
			return false, err
		}

		jobStatusEnum := huaweicloudsdkecsmodel.GetShowJobResponseStatusEnum()
		if *showJobResponse.Status == jobStatusEnum.FAIL {
			errCode := *showJobResponse.ErrorCode
			failReason := *showJobResponse.FailReason
			return false, fmt.Errorf("job failed. error code: %s, error msg: %s", errCode, failReason)
		} else if *showJobResponse.Status == jobStatusEnum.SUCCESS {
			return true, nil
		}

		return true, nil
	})

	if err != nil {
		klog.Warningf("failed to delete servers, error: %v", err)
		return err
	}

	return nil
}

func (csm *cloudServiceManager) GetDesireInstanceNumber(groupID string) (int, error) {
	asClient := csm.getASClientFunc()
	if asClient == nil {
		return 0, fmt.Errorf("failed to get desire instance number due to can not get as client")
	}

	opts := &huaweicloudsdkasmodel.ShowScalingGroupRequest{
		ScalingGroupId: groupID,
	}
	response, err := asClient.ShowScalingGroup(opts)
	if err != nil {
		klog.Errorf("failed to show scaling group info. group: %s, error: %v", groupID, err)
		return 0, err
	}

	if response == nil || response.ScalingGroup == nil {
		klog.Infof("no scaling group found: %s", groupID)
		return 0, nil
	}

	return int(*response.ScalingGroup.DesireInstanceNumber), nil
}

func (csm *cloudServiceManager) GetInstances(groupID string) ([]cloudprovider.Instance, error) {
	asClient := csm.getASClientFunc()
	if asClient == nil {
		return nil, fmt.Errorf("failed to list scaling groups due to can not get as client")
	}

	// SDK 'ListScalingInstances' only return no more than 20 instances.
	// If there is a need in the future, need to retrieve by pages.
	opts := &huaweicloudsdkasmodel.ListScalingInstancesRequest{
		ScalingGroupId: groupID,
	}
	response, err := asClient.ListScalingInstances(opts)
	if err != nil {
		klog.Errorf("failed to list scaling group instances. group: %s, error: %v", groupID, err)
		return nil, err
	}
	if response == nil || response.ScalingGroupInstances == nil {
		klog.Infof("no instance in scaling group: %s", groupID)
		return nil, nil
	}
	instances := make([]cloudprovider.Instance, 0, len(*response.ScalingGroupInstances))
	for _, sgi := range *response.ScalingGroupInstances {
		if sgi.InstanceId == nil {
			continue
		}

		instance := cloudprovider.Instance{
			Id:     *sgi.InstanceId,
			Status: csm.transformInstanceState(*sgi.LifeCycleState, *sgi.HealthStatus),
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

func (csm *cloudServiceManager) IncreaseSizeInstance(asg *AutoScalingGroup, delta int) error {
	asClient := csm.getASClientFunc()
	if asClient == nil {
		return fmt.Errorf("failed to increase scaling groups size due to can not get as client")
	}

	desireNum, err := csm.GetDesireInstanceNumber(asg.groupID)
	if err != nil {
		return err
	}

	newNum := int32(desireNum + delta)
	if int(newNum) > asg.maxInstanceNumber || int(newNum) < 0 {
		return fmt.Errorf("failed to increase scaling groups size reach limit")
	}

	opts := &huaweicloudsdkasmodel.UpdateScalingGroupRequest{
		ScalingGroupId: asg.groupID,
		Body: &huaweicloudsdkasmodel.UpdateScalingGroupRequestBody{
			DesireInstanceNumber: &newNum,
		},
	}

	_, err = asClient.UpdateScalingGroup(opts)
	if err != nil {
		klog.Errorf("failed to update scaling group. group: %s, error: %v", asg.groupID, err)
		return err
	}

	return nil
}

func (csm *cloudServiceManager) ListScalingGroups() ([]AutoScalingGroup, error) {
	asClient := csm.getASClientFunc()
	if asClient == nil {
		return nil, fmt.Errorf("failed to list scaling groups due to can not get as client")
	}

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

	autoScalingGroups := make([]AutoScalingGroup, 0, len(*response.ScalingGroups))
	for _, sg := range *response.ScalingGroups {
		autoScalingGroup := newAutoScalingGroup(csm, sg)
		autoScalingGroups = append(autoScalingGroups, autoScalingGroup)
		klog.Infof("found autoscaling group: %s", autoScalingGroup.groupName)
	}

	return autoScalingGroups, nil
}

func (csm *cloudServiceManager) ListFlavors(az string) ([]*InstanceType, error) {
	ecsClient := csm.getECSClientFunc()
	if ecsClient == nil {
		return nil, fmt.Errorf("failed to list instance flavors due to can not get ecs client")
	}

	opts := &huaweicloudsdkecsmodel.ListFlavorsRequest{
		AvailabilityZone: &az,
	}
	response, err := ecsClient.ListFlavors(opts)
	if err != nil {
		klog.Errorf("failed to list flavors. availability zone: %s", az)
		return nil, err
	}

	instanceTypes := make([]*InstanceType, 0, len(*response.Flavors))
	for _, flavor := range *response.Flavors {
		instanceType := newInstanceType(&flavor)
		instanceTypes = append(instanceTypes, instanceType)
	}
	return instanceTypes, nil
}

func (csm *cloudServiceManager) ShowAsgConfig(cfgID string) (*AutoScalingGroupConfig, error) {
	asClient := csm.getASClientFunc()
	if asClient == nil {
		return nil, fmt.Errorf("failed to get auto scaling config due to can not get as client")
	}

	opts := &huaweicloudsdkasmodel.ShowScalingConfigRequest{
		ScalingConfigurationId: cfgID,
	}
	response, err := asClient.ShowScalingConfig(opts)
	if err != nil {
		klog.Errorf("failed to show scaling group config. config id: %s, error: %v", cfgID, err)
		return nil, err
	}
	return newAutoScalingGroupConfig(response.ScalingConfiguration), nil
}

func (csm *cloudServiceManager) getAsgTemplate(groupID string) (*asgTemplate, error) {
	asClient := csm.getASClientFunc()
	if asClient == nil {
		return nil, fmt.Errorf("failed to get desire instance number due to can not get as client")
	}

	opts := &huaweicloudsdkasmodel.ShowScalingGroupRequest{
		ScalingGroupId: groupID,
	}
	response, err := asClient.ShowScalingGroup(opts)
	if err != nil {
		klog.Errorf("failed to show scaling group info. group: %s, error: %v", groupID, err)
		return nil, err
	}

	if response == nil || response.ScalingGroup == nil {
		klog.Infof("no scaling group found: %s", groupID)
		return nil, nil
	}

	autoScalingConfig, err := csm.ShowAsgConfig(*response.ScalingGroup.ScalingConfigurationId)
	if err != nil {
		klog.Errorf("failed to show scaling group config. group: %s, error: %v", groupID, err)
		return nil, err
	}

	for _, az := range *response.ScalingGroup.AvailableZones {
		flavors, err := csm.ListFlavors(az)
		if err != nil {
			klog.Errorf("failed to list flavors, available zone is: %s, error: %v", az, err)
			return nil, err
		}

		for _, flavor := range flavors {
			if !strings.EqualFold(flavor.Name, autoScalingConfig.flavorRef) {
				continue
			}
			return &asgTemplate{
				InstanceType: flavor,
				Zone:         az,
			}, nil
		}
	}

	return nil, nil
}

func (csm *cloudServiceManager) buildNodeFromTemplate(asgName string, template *asgTemplate) (*apiv1.Node, error) {
	node := apiv1.Node{}
	nodeName := fmt.Sprintf("%s-asg-%d", asgName, rand.Int63())

	node.ObjectMeta = metav1.ObjectMeta{
		Name:     nodeName,
		SelfLink: fmt.Sprintf("/api/v1/nodes/%s", nodeName),
		Labels:   map[string]string{},
	}

	node.Status = apiv1.NodeStatus{
		Capacity: apiv1.ResourceList{},
	}

	// TODO: get a real value.
	node.Status.Capacity[apiv1.ResourcePods] = *resource.NewQuantity(110, resource.DecimalSI)
	node.Status.Capacity[apiv1.ResourceCPU] = *resource.NewQuantity(template.InstanceType.VCPU, resource.DecimalSI)
	node.Status.Capacity[gpu.ResourceNvidiaGPU] = *resource.NewQuantity(template.InstanceType.GPU, resource.DecimalSI)
	node.Status.Capacity[apiv1.ResourceMemory] = *resource.NewQuantity(template.InstanceType.RAM*1024*1024, resource.DecimalSI)

	node.Status.Allocatable = node.Status.Capacity

	node.Labels = cloudprovider.JoinStringMaps(node.Labels, buildGenericLabels(template, nodeName))

	node.Status.Conditions = cloudprovider.BuildReadyConditions()
	return &node, nil
}

func (csm *cloudServiceManager) transformInstanceState(lifeCycleState huaweicloudsdkasmodel.ScalingGroupInstanceLifeCycleState,
	healthStatus huaweicloudsdkasmodel.ScalingGroupInstanceHealthStatus) *cloudprovider.InstanceStatus {
	instanceStatus := &cloudprovider.InstanceStatus{}

	lifeCycleStateEnum := huaweicloudsdkasmodel.GetScalingGroupInstanceLifeCycleStateEnum()
	switch lifeCycleState {
	case lifeCycleStateEnum.INSERVICE:
		instanceStatus.State = cloudprovider.InstanceRunning
	case lifeCycleStateEnum.PENDING:
		instanceStatus.State = cloudprovider.InstanceCreating
	case lifeCycleStateEnum.PENDING_WAIT:
		instanceStatus.State = cloudprovider.InstanceCreating
	case lifeCycleStateEnum.REMOVING:
		instanceStatus.State = cloudprovider.InstanceDeleting
	case lifeCycleStateEnum.REMOVING_WAIT:
		instanceStatus.State = cloudprovider.InstanceDeleting
	default:
		instanceStatus.ErrorInfo = &cloudprovider.InstanceErrorInfo{
			ErrorClass:   cloudprovider.OtherErrorClass,
			ErrorMessage: fmt.Sprintf("invalid instance lifecycle state: %v", lifeCycleState),
		}
		return instanceStatus
	}

	healthStatusEnum := huaweicloudsdkasmodel.GetScalingGroupInstanceHealthStatusEnum()
	switch healthStatus {
	case healthStatusEnum.NORMAL:
	case healthStatusEnum.INITAILIZING:
	case healthStatusEnum.ERROR:
		instanceStatus.ErrorInfo = &cloudprovider.InstanceErrorInfo{
			ErrorClass:   cloudprovider.OtherErrorClass,
			ErrorMessage: fmt.Sprintf("%v", healthStatus),
		}
		return instanceStatus
	default:
		instanceStatus.ErrorInfo = &cloudprovider.InstanceErrorInfo{
			ErrorClass:   cloudprovider.OtherErrorClass,
			ErrorMessage: fmt.Sprintf("invalid instance health state: %v", healthStatus),
		}
		return instanceStatus
	}

	return instanceStatus
}

func buildGenericLabels(template *asgTemplate, nodeName string) map[string]string {
	result := make(map[string]string)
	result[kubeletapis.LabelArch] = cloudprovider.DefaultArch
	result[kubeletapis.LabelOS] = cloudprovider.DefaultOS

	result[apiv1.LabelInstanceType] = template.InstanceType.Name

	result[apiv1.LabelZoneRegion] = template.Region
	result[apiv1.LabelZoneFailureDomain] = template.Zone
	result[apiv1.LabelHostname] = nodeName

	// append custom node labels
	for key, value := range template.Tags {
		result[key] = value
	}

	return result
}
