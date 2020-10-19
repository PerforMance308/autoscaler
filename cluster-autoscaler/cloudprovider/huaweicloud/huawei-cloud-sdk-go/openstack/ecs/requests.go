package ecs

import huaweicloudsdk "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huawei-cloud-sdk-go"

// GetECSFlavors calls ECS REST API to get ecs Flavors' information.
func GetECSFlavors(client *huaweicloudsdk.ServiceClient, availabilityZone string) (r GetECSFlavorsrResult) {
	clusterURL := getFlavorsURL(client, availabilityZone)
	_, r.Err = client.Get(clusterURL, &r.Body, nil)
	return
}

