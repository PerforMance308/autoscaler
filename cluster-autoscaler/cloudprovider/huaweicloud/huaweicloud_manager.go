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

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	huaweicloudsdkasmodel "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/services/as/v1/model"
	"k8s.io/autoscaler/cluster-autoscaler/utils/gpu"
	"k8s.io/klog/v2"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"
)

type HuaweiCloudManager struct {
	cloudConfig *CloudConfig
	ecsWrapper  *ecsWrapper
	asWrapper   *autoScalingWrapper
	asgs        *autoScalingGroups
}

type asgTemplate struct {
	InstanceType *instanceType
	Region       string
	Zone         string
	Tags         map[string]string
}

func newHuaweiCloudManager(cloudConfig *CloudConfig) *HuaweiCloudManager {
	asw := newAutoScalingWrapper(cloudConfig)
	ecsw := newEcsWrapper(cloudConfig)
	return &HuaweiCloudManager{
		cloudConfig: cloudConfig,
		ecsWrapper:  ecsw,
		asWrapper:   asw,
		asgs:        newAutoScalingGroups(asw),
	}
}

func (m *HuaweiCloudManager) RegisterAsg(asg *AutoScalingGroup) {
	m.asgs.Register(asg)
}

func (m *HuaweiCloudManager) ListAsg() (*[]huaweicloudsdkasmodel.ScalingGroups, error) {
	return m.asWrapper.listScalingGroup()
}

func (m *HuaweiCloudManager) GetAsgForInstance(instanceName string) (*AutoScalingGroup, error) {
	return m.asgs.FindForInstance(instanceName)
}

func (m *HuaweiCloudManager) GetAsgSize(groupID string) (int, error) {
	sg, err := m.asWrapper.getScalingGroupByID(groupID)
	if err != nil {
		return -1, fmt.Errorf("failed to describe ASG %s,Because of %s", groupID, err.Error())
	}
	return int(*sg.DesireInstanceNumber), nil
}

// SetAsgSize sets ASG size.
func (m *HuaweiCloudManager) ScaleUpCluster(groupID string, size int) error {
	return m.asWrapper.scaleUpCluster(groupID, size)
}

func (m *HuaweiCloudManager) GetInstances(groupID string) ([]huaweicloudsdkasmodel.ScalingGroupInstance, error) {
	return m.asWrapper.getInstances(groupID)
}

func (m *HuaweiCloudManager) ScaleDownCluster(groupID string, instanceIds []string) error {
	return m.asWrapper.scaleDownCluster(groupID, instanceIds)
}

func (m *HuaweiCloudManager) getAsgTemplate(groupID string) (*asgTemplate, error) {
	sg, err := m.asWrapper.getScalingGroupByID(groupID)
	if err != nil {
		klog.Errorf("failed to get ASG by id:%s,because of %s", groupID, err.Error())
		return nil, err
	}

	configuration, err := m.asWrapper.getScalingGroupConfigByID(groupID, *sg.ScalingConfigurationId)

	for _, az := range *sg.AvailableZones {
		flavors, err := m.ecsWrapper.listFlavors(az)
		if err != nil {
			klog.Errorf("failed to list flavors, available zone is: %s, error: %v", az, err)
			return nil, err
		}

		for _, flavor := range flavors {
			if !strings.EqualFold(flavor.name, *configuration.InstanceConfig.FlavorRef) {
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

func (csm *HuaweiCloudManager) buildNodeFromTemplate(asgName string, template *asgTemplate) (*apiv1.Node, error) {
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
	node.Status.Capacity[apiv1.ResourceCPU] = *resource.NewQuantity(template.InstanceType.vcpu, resource.DecimalSI)
	node.Status.Capacity[gpu.ResourceNvidiaGPU] = *resource.NewQuantity(template.InstanceType.gpu, resource.DecimalSI)
	node.Status.Capacity[apiv1.ResourceMemory] = *resource.NewQuantity(template.InstanceType.ram*1024*1024, resource.DecimalSI)

	node.Status.Allocatable = node.Status.Capacity

	node.Labels = cloudprovider.JoinStringMaps(node.Labels, buildGenericLabels(template, nodeName))

	node.Status.Conditions = cloudprovider.BuildReadyConditions()
	return &node, nil
}

func buildGenericLabels(template *asgTemplate, nodeName string) map[string]string {
	result := make(map[string]string)
	result[kubeletapis.LabelArch] = cloudprovider.DefaultArch
	result[kubeletapis.LabelOS] = cloudprovider.DefaultOS

	result[apiv1.LabelInstanceType] = template.InstanceType.name

	result[apiv1.LabelZoneRegion] = template.Region
	result[apiv1.LabelZoneFailureDomain] = template.Zone
	result[apiv1.LabelHostname] = nodeName

	// append custom node labels
	for key, value := range template.Tags {
		result[key] = value
	}

	return result
}
