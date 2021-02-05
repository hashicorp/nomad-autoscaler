package plugin

import (
	"context"
	"google.golang.org/api/compute/v1"
)

type instanceGroup interface {
	getName() string
	status(ctx context.Context, service *compute.Service) (bool, int64, error)
	resize(ctx context.Context, service *compute.Service, num int64) error
	deleteInstance(ctx context.Context, service *compute.Service, instanceIDs []string) error
}

type regionalInstanceGroup struct {
	project string
	region  string
	name    string
}

type zonalInstanceGroup struct {
	project string
	zone    string
	name    string
}

func (z *zonalInstanceGroup) getName() string {
	return z.name
}

func (z *zonalInstanceGroup) status(ctx context.Context, service *compute.Service) (bool, int64, error) {
	mig, err := service.InstanceGroupManagers.Get(z.project, z.zone, z.name).Context(ctx).Do()
	if err != nil {
		return false, -1, err
	}
	return mig.Status.IsStable, mig.TargetSize, nil
}

func (z *zonalInstanceGroup) resize(ctx context.Context, service *compute.Service, num int64) error {
	_, err := service.InstanceGroupManagers.Resize(z.project, z.zone, z.name, num).Context(ctx).Do()
	return err
}

func (z *zonalInstanceGroup) deleteInstance(ctx context.Context, service *compute.Service, instanceIDs []string) error {
	request := &compute.InstanceGroupManagersDeleteInstancesRequest{
		Instances: instanceIDs,
	}

	_, err := service.InstanceGroupManagers.DeleteInstances(z.project, z.zone, z.name, request).Context(ctx).Do()
	return err
}

func (r *regionalInstanceGroup) getName() string {
	return r.name
}

func (r *regionalInstanceGroup) status(ctx context.Context, service *compute.Service) (bool, int64, error) {
	mig, err := service.RegionInstanceGroupManagers.Get(r.project, r.region, r.name).Context(ctx).Do()
	if err != nil {
		return false, -1, err
	}
	return mig.Status.IsStable, mig.TargetSize, nil
}

func (r *regionalInstanceGroup) resize(ctx context.Context, service *compute.Service, num int64) error {
	_, err := service.RegionInstanceGroupManagers.Resize(r.project, r.region, r.name, num).Context(ctx).Do()
	return err
}

func (r *regionalInstanceGroup) deleteInstance(ctx context.Context, service *compute.Service, instanceIDs []string) error {
	request := &compute.RegionInstanceGroupManagersDeleteInstancesRequest{
		Instances: instanceIDs,
	}

	_, err := service.RegionInstanceGroupManagers.DeleteInstances(r.project, r.region, r.name, request).Context(ctx).Do()
	return err
}
