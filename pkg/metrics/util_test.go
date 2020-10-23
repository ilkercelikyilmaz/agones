// Copyright 2018 Google LLC All Rights Reserved.
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

package metrics

import (
	"context"
	"testing"

	agonesv1 "agones.dev/agones/pkg/apis/agones/v1"
	autoscalingv1 "agones.dev/agones/pkg/apis/autoscaling/v1"
	agtesting "agones.dev/agones/pkg/testing"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/watch"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

// newFakeController returns a controller, backed by the fake Clientset
func newFakeController() *fakeController {
	m := agtesting.NewMocks()
	c := NewController(m.KubeClient, m.AgonesClient, m.KubeInformerFactory, m.AgonesInformerFactory)
	gsWatch := watch.NewFake()
	fasWatch := watch.NewFake()
	fleetWatch := watch.NewFake()
	nodeWatch := watch.NewFake()

	m.AgonesClient.AddWatchReactor("gameservers", k8stesting.DefaultWatchReactor(gsWatch, nil))
	m.AgonesClient.AddWatchReactor("fleetautoscalers", k8stesting.DefaultWatchReactor(fasWatch, nil))
	m.AgonesClient.AddWatchReactor("fleets", k8stesting.DefaultWatchReactor(fleetWatch, nil))
	m.KubeClient.AddWatchReactor("nodes", k8stesting.DefaultWatchReactor(nodeWatch, nil))

	stop, cancel := agtesting.StartInformers(m, c.gameServerSynced, c.fleetSynced, c.fasSynced, c.nodeSynced)

	return &fakeController{
		Controller: c,
		Mocks:      m,
		gsWatch:    gsWatch,
		fasWatch:   fasWatch,
		fleetWatch: fleetWatch,
		nodeWatch:  nodeWatch,
		cancel:     cancel,
		stop:       stop,
	}
}

func (c *fakeController) close() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *fakeController) run(t *testing.T) {
	go func() {
		err := c.Controller.Run(1, c.stop)
		assert.Nil(t, err)
	}()
	c.sync()
}

func (c *fakeController) sync() {
	cache.WaitForCacheSync(c.stop, c.gameServerSynced, c.fleetSynced, c.fasSynced, c.nodeSynced)
}

type fakeController struct {
	*Controller
	agtesting.Mocks
	gsWatch    *watch.FakeWatcher
	fasWatch   *watch.FakeWatcher
	fleetWatch *watch.FakeWatcher
	nodeWatch  *watch.FakeWatcher
	stop       <-chan struct{}
	cancel     context.CancelFunc
}

func nodeWithName(name string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  uuid.NewUUID(),
		},
	}
}

func gameServerWithNode(nodeName string) *agonesv1.GameServer {
	gs := gameServerWithFleetAndState("fleet", agonesv1.GameServerStateReady)
	gs.Status.NodeName = nodeName
	return gs
}

func gameServerWithFleetAndState(fleetName string, state agonesv1.GameServerState) *agonesv1.GameServer {
	lbs := map[string]string{}
	if fleetName != "" {
		lbs[agonesv1.FleetNameLabel] = fleetName
	}
	gs := &agonesv1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.String(10),
			Namespace: "default",
			UID:       uuid.NewUUID(),
			Labels:    lbs,
		},
		Status: agonesv1.GameServerStatus{
			State: state,
		},
	}
	return gs
}

func generateGsEvents(count int, state agonesv1.GameServerState, fleetName string, fakew *watch.FakeWatcher) {
	for i := 0; i < count; i++ {
		gs := gameServerWithFleetAndState(fleetName, agonesv1.GameServerState(""))
		fakew.Add(gs)
		gsUpdated := gs.DeepCopy()
		gsUpdated.Status.State = state
		fakew.Modify(gsUpdated)
		// make sure we count only one event
		fakew.Modify(gsUpdated)
	}
}

func fleet(fleetName string, total, allocated, ready, desired int32) *agonesv1.Fleet {
	return &agonesv1.Fleet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fleetName,
			Namespace: "default",
			UID:       uuid.NewUUID(),
		},
		Spec: agonesv1.FleetSpec{
			Replicas: desired,
		},
		Status: agonesv1.FleetStatus{
			AllocatedReplicas: allocated,
			ReadyReplicas:     ready,
			Replicas:          total,
		},
	}
}

func fleetAutoScaler(fleetName string, fasName string) *autoscalingv1.FleetAutoscaler {
	return &autoscalingv1.FleetAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fasName,
			Namespace: "default",
			UID:       uuid.NewUUID(),
		},
		Spec: autoscalingv1.FleetAutoscalerSpec{
			FleetName: fleetName,
			Policy: autoscalingv1.FleetAutoscalerPolicy{
				Type: autoscalingv1.BufferPolicyType,
				Buffer: &autoscalingv1.BufferPolicy{
					MaxReplicas: 30,
					MinReplicas: 10,
					BufferSize:  intstr.FromInt(11),
				},
			},
		},
		Status: autoscalingv1.FleetAutoscalerStatus{
			AbleToScale:     true,
			ScalingLimited:  false,
			CurrentReplicas: 10,
			DesiredReplicas: 20,
		},
	}
}
