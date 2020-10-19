package ecs

import huaweicloudsdk "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huawei-cloud-sdk-go"

// ECSFlavors ECS flavors
type ECSFlavors struct {
	Flavors    []ECSFlavor   `json:"flavor"`
}

// ECSFlavor ECS flavor
type ECSFlavor struct {
	ID       string   `json:"id"`
	Name 	 string   `json:"name"`
	VCPU     string   `json:"vcpus"`
	Ram      int   	  `json:"ram"`
}

// GetECSFlavorsrResult for ECS
type GetECSFlavorsrResult struct {
	huaweicloudsdk.Result
}

// Extract flavors info and error
func (r GetECSFlavorsrResult) Extract() ([]ECSFlavor, error) {
	var res []ECSFlavor
	err := r.ExtractInto(&res)
	return res, err
}