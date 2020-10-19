package ecs

import (
	"fmt"
	huaweicloudsdk "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/huaweicloud/huawei-cloud-sdk-go"
)


const (
	flavors = "flavors?availability_zone=%s"
)

// getFlavorsURL returns a URL for getting the information of a ecs flavors.
// REST API:
// 		GET /v1/{project_id}/cloudservers/flavors?availability_zone={availability_zone}
// Example:
// 		https://cce.cn-north-1.myhuaweicloud.com/v1/017a290a8242480e82de8db804c1718d/cloudservers/flavors?availability_zone=AZ1
func getFlavorsURL(sc *huaweicloudsdk.ServiceClient, availabilityZone string) string {
	return sc.ServiceURL(fmt.Sprintf(flavors, availabilityZone))
}