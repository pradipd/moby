package service

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/request"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestCreateWithLBSandbox(t *testing.T) {
	defer setupTest(t)()
	d := newSwarm(t)
	defer d.Stop(t)
	client, err := request.NewClientForHost(d.Sock())
	require.NoError(t, err)

	overlayName := "overlay1"
	networkCreate := types.NetworkCreate{
		CheckDuplicate: true,
		Driver:         "overlay",
	}

	ctx := context.Background()
	netResp, err := client.NetworkCreate(ctx, overlayName, networkCreate)
	require.NoError(t, err)
	overlayID := netResp.ID

	var instances uint64 = 1
	serviceSpec := swarmServiceSpec("TestService", instances)

	serviceSpec.TaskTemplate.Networks = append(serviceSpec.TaskTemplate.Networks, swarm.NetworkAttachmentConfig{Target: overlayName})

	serviceResp, err := client.ServiceCreate(ctx, serviceSpec, types.ServiceCreateOptions{
		QueryRegistry: false,
	})
	require.NoError(t, err)

	serviceID := serviceResp.ID
	poll.WaitOn(t, serviceTaskCount(client, serviceID, instances), pollSettings)

	network, err := client.NetworkInspect(ctx, overlayID, types.NetworkInspectOptions{})
	require.NoError(t, err)
	assert.Contains(t, network.Containers, overlayName+"-sbox")

	err = client.ServiceRemove(ctx, serviceID)
	require.NoError(t, err)

	poll.WaitOn(t, serviceIsRemoved(client, serviceID), pollSettings)
	err = client.NetworkRemove(ctx, overlayID)
	require.NoError(t, err)

	poll.WaitOn(t, networkIsRemoved(client, overlayID), pollSettings, poll.WithTimeout(time.Minute))
}

func swarmServiceSpec(name string, replicas uint64) swarm.ServiceSpec {
	return swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: name,
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: &swarm.ContainerSpec{
				Image:   "busybox:latest",
				Command: []string{"/bin/top"},
			},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &replicas,
			},
		},
	}
}

func serviceIsRemoved(client client.ServiceAPIClient, serviceID string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		filter := filters.NewArgs()
		filter.Add("service", serviceID)
		_, err := client.TaskList(context.Background(), types.TaskListOptions{
			Filters: filter,
		})
		if err == nil {
			return poll.Continue("waiting for service %s to be deleted", serviceID)
		}
		return poll.Success()
	}
}

func networkIsRemoved(client client.NetworkAPIClient, networkID string) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		_, err := client.NetworkInspect(context.Background(), networkID, types.NetworkInspectOptions{})
		if err == nil {
			return poll.Continue("waiting for network %s to be removed", networkID)
		}
		return poll.Success()
	}
}
