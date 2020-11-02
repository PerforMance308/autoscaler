package huaweicloud

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

type autoScalingGroups struct {
	registeredAsgs           []*asgInformation
	instanceToAsg            map[string]*AutoScalingGroup
	cacheMutex               sync.Mutex
	instancesNotInManagedAsg map[string]struct{}
	service                  *autoScalingWrapper
}

type asgInformation struct {
	config   *AutoScalingGroup
	basename string
}

func newAutoScalingGroups(asClient *autoScalingWrapper) *autoScalingGroups {
	registry := &autoScalingGroups{
		service:                  asClient,
		registeredAsgs:           make([]*asgInformation, 0),
		instanceToAsg:            make(map[string]*AutoScalingGroup),
		instancesNotInManagedAsg: make(map[string]struct{}),
	}

	go wait.Forever(func() {
		registry.cacheMutex.Lock()
		defer registry.cacheMutex.Unlock()
		if err := registry.regenerateCache(); err != nil {
			klog.Errorf("failed to do regenerating ASG cache,because of %s", err.Error())
		}
	}, time.Hour)
	return registry
}

// Register registers Asg in Manager.
func (m *autoScalingGroups) Register(asg *AutoScalingGroup) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	m.registeredAsgs = append(m.registeredAsgs, &asgInformation{
		config: asg,
	})
}

func (m *autoScalingGroups) FindForInstance(instanceName string) (*AutoScalingGroup, error) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	if config, found := m.instanceToAsg[instanceName]; found {
		return config, nil
	}

	if _, found := m.instancesNotInManagedAsg[instanceName]; found {
		return nil, nil
	}
	if err := m.regenerateCache(); err != nil {
		return nil, err
	}
	if config, found := m.instanceToAsg[instanceName]; found {
		return config, nil
	}
	// instance does not belong to any configured ASG
	m.instancesNotInManagedAsg[instanceName] = struct{}{}
	return nil, nil
}

func (m *autoScalingGroups) regenerateCache() error {
	newCache := make(map[string]*AutoScalingGroup)

	for _, asg := range m.registeredAsgs {
		instances, err := m.service.getScalingInstancesByGroup(asg.config.groupID)
		if err != nil {
			return err
		}
		for _, instance := range *instances {
			newCache[*instance.InstanceName] = asg.config
		}
	}

	m.instanceToAsg = newCache
	return nil
}
