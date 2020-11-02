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
	"sync"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	huaweicloudsdkasmodel "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huaweicloud-sdk-go-v3/services/as/v1/model"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	"k8s.io/autoscaler/cluster-autoscaler/config/dynamic"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	klog "k8s.io/klog/v2"
)

const (
	// GPULabel is the label added to nodes with GPU resource.
	GPULabel = "cloud.google.com/gke-accelerator"
)

var (
	availableGPUTypes = map[string]struct{}{
		"nvidia-tesla-k80":  {},
		"nvidia-tesla-p100": {},
		"nvidia-tesla-v100": {},
	}
)

// huaweicloudCloudProvider implements CloudProvider interface defined in autoscaler/cluster-autoscaler/cloudprovider/cloud_provider.go
type huaweicloudCloudProvider struct {
	manager          *HuaweiCloudManager
	resourceLimiter  *cloudprovider.ResourceLimiter
	autoScalingGroup []*AutoScalingGroup
	lock             sync.RWMutex
}

func newCloudProvider(opts config.AutoscalingOptions, do cloudprovider.NodeGroupDiscoveryOptions, rl *cloudprovider.ResourceLimiter) (*huaweicloudCloudProvider, error) {
	cloudConfig, err := readConf(opts.CloudConfig)
	if err != nil {
		klog.Errorf("failed to read cloud configuration. error: %v", err)
		return nil, err
	}
	if err = cloudConfig.validate(); err != nil {
		klog.Errorf("cloud configuration is invalid. error: %v", err)
		return nil, err
	}

	csm := newHuaweiCloudManager(cloudConfig)

	hcp := &huaweicloudCloudProvider{
		manager:          csm,
		resourceLimiter:  rl,
		autoScalingGroup: make([]*AutoScalingGroup, 0),
	}

	asgs, err := hcp.manager.ListAsg()
	if err != nil {
		klog.Warning("failed to list scaling group to huaweicloud provider")
		return nil, err
	}
	for _, spec := range do.NodeGroupSpecs {
		if err := hcp.addNodeGroup(spec, asgs); err != nil {
			klog.Warningf("failed to add node group to huaweicloud provider with spec: %s", spec)
			return nil, err
		}
	}

	return hcp, nil
}

func (hcp *huaweicloudCloudProvider) addNodeGroup(value string, asgs *[]huaweicloudsdkasmodel.ScalingGroups) error {
	nodeSpec, err := dynamic.SpecFromString(value, true)

	if err != nil {
		return fmt.Errorf("failed to parse node group spec: %v", err)
	}

	for _, asg := range *asgs {
		if *asg.ScalingGroupName == nodeSpec.Name {
			asg, err := buildAsgFromSpec(*asg.ScalingGroupId, hcp.manager, asgs)
			if err != nil {
				klog.Errorf("failed to build ASG from spec,because of %s", err.Error())
				return err
			}
			hcp.addAsg(asg)
		}
	}

	return nil
}

func (hcp *huaweicloudCloudProvider) addAsg(asg *AutoScalingGroup) {
	hcp.autoScalingGroup = append(hcp.autoScalingGroup, asg)
	hcp.manager.RegisterAsg(asg)
}

// Name returns the name of the cloud provider.
func (hcp *huaweicloudCloudProvider) Name() string {
	return cloudprovider.HuaweicloudProviderName
}

// NodeGroups returns all node groups managed by this cloud provider.
func (hcp *huaweicloudCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	groups := make([]cloudprovider.NodeGroup, 0, len(hcp.autoScalingGroup))
	for _, asg := range hcp.autoScalingGroup {
		groups = append(groups, asg)
	}
	return groups
}

// NodeGroupForNode returns the node group for the given node, nil if the node
// should not be processed by cluster autoscaler, or non-nil error if such
// occurred. Must be implemented.
func (hcp *huaweicloudCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	if _, found := node.ObjectMeta.Labels["node-role.kubernetes.io/master"]; found {
		return nil, nil
	}

	return hcp.manager.GetAsgForInstance(node.Name)
}

// Pricing returns pricing model for this cloud provider or error if not available. Not implemented.
func (hcp *huaweicloudCloudProvider) Pricing() (cloudprovider.PricingModel, errors.AutoscalerError) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider. Not implemented.
func (hcp *huaweicloudCloudProvider) GetAvailableMachineTypes() ([]string, error) {
	return []string{}, nil
}

// NewNodeGroup builds a theoretical node group based on the node definition provided. The node group is not automatically
// created on the cloud provider side. The node group is not returned by NodeGroups() until it is created. Not implemented.
func (hcp *huaweicloudCloudProvider) NewNodeGroup(machineType string, labels map[string]string, systemLabels map[string]string,
	taints []apiv1.Taint, extraResources map[string]resource.Quantity) (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetResourceLimiter returns struct containing limits (max, min) for resources (cores, memory etc.).
func (hcp *huaweicloudCloudProvider) GetResourceLimiter() (*cloudprovider.ResourceLimiter, error) {
	return hcp.resourceLimiter, nil
}

// GPULabel returns the label added to nodes with GPU resource.
func (hcp *huaweicloudCloudProvider) GPULabel() string {
	return GPULabel
}

// GetAvailableGPUTypes returns all available GPU types cloud provider supports.
func (hcp *huaweicloudCloudProvider) GetAvailableGPUTypes() map[string]struct{} {
	return availableGPUTypes
}

// Cleanup currently does nothing.
func (hcp *huaweicloudCloudProvider) Cleanup() error {
	return nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
// In particular the list of node groups returned by NodeGroups can change as a result of CloudProvider.Refresh().
// Currently does nothing.
func (hcp *huaweicloudCloudProvider) Refresh() error {
	return nil
}

// BuildHuaweiCloud is called by the autoscaler/cluster-autoscaler/builder to build a huaweicloud cloud provider.
func BuildHuaweiCloud(opts config.AutoscalingOptions, do cloudprovider.NodeGroupDiscoveryOptions, rl *cloudprovider.ResourceLimiter) cloudprovider.CloudProvider {
	if len(opts.CloudConfig) == 0 {
		klog.Fatalf("cloud config is missing.")
	}

	hcp, err := newCloudProvider(opts, do, rl)

	if err != nil {
		klog.Errorf("failed to build huawei cloud provider, error: %v", err)
		return nil
	}
	return hcp
}

func buildAsgFromSpec(groupID string, manager *HuaweiCloudManager, asgs *[]huaweicloudsdkasmodel.ScalingGroups) (*AutoScalingGroup, error) {
	// check auto scaling group is exists or not
	asg, err := manager.asWrapper.getScalingGroupByID(groupID)
	if err != nil {
		klog.Errorf("your scaling group: %s does not exist", groupID)
		return nil, err
	}

	return &AutoScalingGroup{
		manager:           manager,
		groupName:         *asg.ScalingGroupName,
		groupID:           *asg.ScalingGroupId,
		maxInstanceNumber: int(*asg.MaxInstanceNumber),
		minInstanceNumber: int(*asg.MinInstanceNumber),
	}, nil
}
