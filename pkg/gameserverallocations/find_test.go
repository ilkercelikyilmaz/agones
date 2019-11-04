// Copyright 2019 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gameserverallocations

import (
	"testing"

	"agones.dev/agones/pkg/apis"

	agonesv1 "agones.dev/agones/pkg/apis/agones/v1"
	allocationv1 "agones.dev/agones/pkg/apis/allocation/v1"
	agtesting "agones.dev/agones/pkg/testing"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

func TestFindGameServerForAllocationPacked(t *testing.T) {
	t.Parallel()

	labels := map[string]string{"role": "gameserver"}
	prefLabels := map[string]string{"role": "gameserver", "preferred": "true"}

	gsa := &allocationv1.GameServerAllocation{
		ObjectMeta: metav1.ObjectMeta{Namespace: defaultNs},
		Spec: allocationv1.GameServerAllocationSpec{
			Required: metav1.LabelSelector{
				MatchLabels: labels,
			},
			Scheduling: apis.Packed,
		},
	}

	n := metav1.Now()
	prefGsa := gsa.DeepCopy()
	prefGsa.Spec.Preferred = append(prefGsa.Spec.Preferred, metav1.LabelSelector{
		MatchLabels: map[string]string{"preferred": "true"},
	})

	fixtures := map[string]struct {
		list []agonesv1.GameServer
		test func(*testing.T, []*agonesv1.GameServer)
	}{
		"required": {
			list: []agonesv1.GameServer{
				{ObjectMeta: metav1.ObjectMeta{Name: "gs6", Namespace: defaultNs, Labels: labels, DeletionTimestamp: &n}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateReady}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs1", Namespace: defaultNs, Labels: labels}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateReady}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs2", Namespace: defaultNs, Labels: labels}, Status: agonesv1.GameServerStatus{NodeName: "node2", State: agonesv1.GameServerStateReady}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs3", Namespace: defaultNs, Labels: labels}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateAllocated}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs4", Namespace: defaultNs, Labels: labels}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateAllocated}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs5", Namespace: defaultNs, Labels: labels}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateError}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs6", Namespace: "does-not-apply", Labels: labels}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateReady}},
			},
			test: func(t *testing.T, list []*agonesv1.GameServer) {
				assert.Len(t, list, 3)

				gs, index, err := findGameServerForAllocation(gsa, list)
				assert.NoError(t, err)
				if !assert.NotNil(t, gs) {
					assert.FailNow(t, "gameserver should not be nil")
				}
				assert.Equal(t, "node1", gs.Status.NodeName)
				assert.Equal(t, "gs1", gs.ObjectMeta.Name)
				assert.Equal(t, gs, list[index])
				assert.Equal(t, agonesv1.GameServerStateReady, gs.Status.State)

				// mock that the first found game server is allocated
				list = append(list[:index], list[index+1:]...)
				assert.Equal(t, agonesv1.GameServerStateReady, list[0].Status.State)
				assert.Len(t, list, 2)

				gs, index, err = findGameServerForAllocation(gsa, list)
				assert.NoError(t, err)
				if !assert.NotNil(t, gs) {
					assert.FailNow(t, "gameserver should not be nil")
				}
				assert.Equal(t, "node2", gs.Status.NodeName)
				assert.Equal(t, "gs2", gs.ObjectMeta.Name)
				assert.Equal(t, gs, list[index])
				assert.Equal(t, agonesv1.GameServerStateReady, gs.Status.State)

				list = nil
				gs, _, err = findGameServerForAllocation(gsa, list)
				assert.Error(t, err)
				assert.Equal(t, ErrNoGameServerReady, err)
				assert.Nil(t, gs)
			},
		},
		"preferred": {
			list: []agonesv1.GameServer{
				{ObjectMeta: metav1.ObjectMeta{Name: "gs1", Namespace: defaultNs, Labels: prefLabels}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateReady}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs2", Namespace: defaultNs, Labels: labels}, Status: agonesv1.GameServerStatus{NodeName: "node2", State: agonesv1.GameServerStateReady}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs3", Namespace: defaultNs, Labels: labels}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateReady}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs4", Namespace: defaultNs, Labels: prefLabels}, Status: agonesv1.GameServerStatus{NodeName: "node2", State: agonesv1.GameServerStateReady}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs5", Namespace: defaultNs, Labels: labels}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateReady}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs6", Namespace: defaultNs, Labels: labels}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateReady}},
			},
			test: func(t *testing.T, list []*agonesv1.GameServer) {
				assert.Len(t, list, 6)

				gs, index, err := findGameServerForAllocation(prefGsa, list)
				assert.NoError(t, err)
				assert.Equal(t, "node1", gs.Status.NodeName)
				assert.Equal(t, "gs1", gs.ObjectMeta.Name)
				assert.Equal(t, gs, list[index])
				assert.Equal(t, agonesv1.GameServerStateReady, gs.Status.State)

				list = append(list[:index], list[index+1:]...)
				gs, index, err = findGameServerForAllocation(prefGsa, list)
				assert.NoError(t, err)
				assert.Equal(t, "node2", gs.Status.NodeName)
				assert.Equal(t, "gs4", gs.ObjectMeta.Name)
				assert.Equal(t, gs, list[index])
				assert.Equal(t, agonesv1.GameServerStateReady, gs.Status.State)

				list = append(list[:index], list[index+1:]...)
				gs, index, err = findGameServerForAllocation(prefGsa, list)
				assert.NoError(t, err)
				assert.Equal(t, "node1", gs.Status.NodeName)
				assert.Contains(t, []string{"gs3", "gs5", "gs6"}, gs.ObjectMeta.Name)
				assert.Equal(t, gs, list[index])
				assert.Equal(t, agonesv1.GameServerStateReady, gs.Status.State)
			},
		},
		"allocation trap": {
			list: []agonesv1.GameServer{
				{ObjectMeta: metav1.ObjectMeta{Name: "gs1", Labels: labels, Namespace: defaultNs}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateAllocated}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs2", Labels: labels, Namespace: defaultNs}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateAllocated}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs3", Labels: labels, Namespace: defaultNs}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateAllocated}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs4", Labels: labels, Namespace: defaultNs}, Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateAllocated}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs5", Labels: labels, Namespace: defaultNs}, Status: agonesv1.GameServerStatus{NodeName: "node2", State: agonesv1.GameServerStateReady}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs6", Labels: labels, Namespace: defaultNs}, Status: agonesv1.GameServerStatus{NodeName: "node2", State: agonesv1.GameServerStateReady}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs7", Labels: labels, Namespace: defaultNs}, Status: agonesv1.GameServerStatus{NodeName: "node2", State: agonesv1.GameServerStateReady}},
				{ObjectMeta: metav1.ObjectMeta{Name: "gs8", Labels: labels, Namespace: defaultNs}, Status: agonesv1.GameServerStatus{NodeName: "node2", State: agonesv1.GameServerStateReady}},
			},
			test: func(t *testing.T, list []*agonesv1.GameServer) {
				assert.Len(t, list, 4)

				gs, index, err := findGameServerForAllocation(gsa, list)
				assert.Nil(t, err)
				assert.Equal(t, "node2", gs.Status.NodeName)
				assert.Equal(t, gs, list[index])
				assert.Equal(t, agonesv1.GameServerStateReady, gs.Status.State)
			},
		},
	}

	for k, v := range fixtures {
		t.Run(k, func(t *testing.T) {
			controller, m := newFakeController()
			c := controller.allocator.readyGameServerCache

			m.AgonesClient.AddReactor("list", "gameservers", func(action k8stesting.Action) (bool, runtime.Object, error) {
				return true, &agonesv1.GameServerList{Items: v.list}, nil
			})

			stop, cancel := agtesting.StartInformers(m, c.gameServerSynced)
			defer cancel()

			// This call initializes the cache
			err := c.syncReadyGSServerCache()
			assert.Nil(t, err)

			err = c.counter.Run(0, stop)
			assert.Nil(t, err)

			list := c.ListSortedReadyGameServers()
			v.test(t, list)
		})
	}
}

func TestFindGameServerForAllocationDistributed(t *testing.T) {
	t.Parallel()

	controller, m := newFakeController()
	c := controller.allocator.readyGameServerCache
	labels := map[string]string{"role": "gameserver"}

	gsa := &allocationv1.GameServerAllocation{
		ObjectMeta: metav1.ObjectMeta{Namespace: defaultNs},
		Spec: allocationv1.GameServerAllocationSpec{
			Required: metav1.LabelSelector{
				MatchLabels: labels,
			},
			Scheduling: apis.Distributed,
		},
	}

	gsList := []agonesv1.GameServer{
		{ObjectMeta: metav1.ObjectMeta{Name: "gs1", Namespace: defaultNs, Labels: labels},
			Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateReady}},
		{ObjectMeta: metav1.ObjectMeta{Name: "gs2", Namespace: defaultNs, Labels: labels},
			Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateReady}},
		{ObjectMeta: metav1.ObjectMeta{Name: "gs3", Namespace: defaultNs, Labels: labels},
			Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateReady}},
		{ObjectMeta: metav1.ObjectMeta{Name: "gs4", Namespace: defaultNs, Labels: labels},
			Status: agonesv1.GameServerStatus{NodeName: "node1", State: agonesv1.GameServerStateError}},
		{ObjectMeta: metav1.ObjectMeta{Name: "gs5", Namespace: defaultNs, Labels: labels},
			Status: agonesv1.GameServerStatus{NodeName: "node2", State: agonesv1.GameServerStateReady}},
		{ObjectMeta: metav1.ObjectMeta{Name: "gs6", Namespace: defaultNs, Labels: labels},
			Status: agonesv1.GameServerStatus{NodeName: "node2", State: agonesv1.GameServerStateReady}},
		{ObjectMeta: metav1.ObjectMeta{Name: "gs7", Namespace: defaultNs, Labels: labels},
			Status: agonesv1.GameServerStatus{NodeName: "node3", State: agonesv1.GameServerStateReady}},
	}

	m.AgonesClient.AddReactor("list", "gameservers", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &agonesv1.GameServerList{Items: gsList}, nil
	})

	stop, cancel := agtesting.StartInformers(m, c.gameServerSynced)
	defer cancel()

	// This call initializes the cache
	err := c.syncReadyGSServerCache()
	assert.Nil(t, err)

	err = c.counter.Run(0, stop)
	assert.Nil(t, err)

	list := c.ListSortedReadyGameServers()
	assert.Len(t, list, 6)

	gs, index, err := findGameServerForAllocation(gsa, list)
	assert.NoError(t, err)
	assert.Equal(t, gs, list[index])
	assert.Equal(t, agonesv1.GameServerStateReady, gs.Status.State)

	past := gs
	// we should get a different result in 10 tries, so we can see we get some randomness.
	for i := 0; i < 10; i++ {
		gs, index, err = findGameServerForAllocation(gsa, list)
		assert.NoError(t, err)
		assert.Equal(t, gs, list[index])
		assert.Equal(t, agonesv1.GameServerStateReady, gs.Status.State)

		if gs.ObjectMeta.Name != past.ObjectMeta.Name {
			return
		}
	}

	assert.FailNow(t, "We should get a different gameserver by now")

}
