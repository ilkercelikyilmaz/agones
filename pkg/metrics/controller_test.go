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
	"strings"
	"testing"

	agonesv1 "agones.dev/agones/pkg/apis/agones/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestControllerGameServerCount(t *testing.T) {

	registry := prometheus.NewRegistry()
	_, err := RegisterPrometheusExporter(registry)
	assert.Nil(t, err)

	c := newFakeController()
	defer c.close()

	gs1 := gameServerWithFleetAndState("test-fleet", agonesv1.GameServerStateCreating)
	c.gsWatch.Add(gs1)
	gs1 = gs1.DeepCopy()
	gs1.Status.State = agonesv1.GameServerStateReady
	c.gsWatch.Modify(gs1)

	c.sync()
	c.collect()
	report()

	gs1 = gs1.DeepCopy()
	gs1.Status.State = agonesv1.GameServerStateShutdown
	c.gsWatch.Modify(gs1)
	c.gsWatch.Add(gameServerWithFleetAndState("", agonesv1.GameServerStatePortAllocation))
	c.gsWatch.Add(gameServerWithFleetAndState("", agonesv1.GameServerStatePortAllocation))

	c.sync()
	c.collect()
	report()

	assert.Nil(t, testutil.GatherAndCompare(registry, strings.NewReader(gsCountExpected), "agones_gameservers_count"))
}

func TestControllerGameServersTotal(t *testing.T) {

	registry := prometheus.NewRegistry()
	_, err := RegisterPrometheusExporter(registry)
	assert.Nil(t, err)

	c := newFakeController()
	defer c.close()
	c.run(t)

	// deleted gs should not be counted
	gs := gameServerWithFleetAndState("deleted", agonesv1.GameServerStateCreating)
	c.gsWatch.Add(gs)
	c.gsWatch.Delete(gs)

	generateGsEvents(16, agonesv1.GameServerStateCreating, "test", c.gsWatch)
	generateGsEvents(15, agonesv1.GameServerStateScheduled, "test", c.gsWatch)
	generateGsEvents(10, agonesv1.GameServerStateStarting, "test", c.gsWatch)
	generateGsEvents(1, agonesv1.GameServerStateUnhealthy, "test", c.gsWatch)
	generateGsEvents(19, agonesv1.GameServerStateCreating, "", c.gsWatch)
	generateGsEvents(18, agonesv1.GameServerStateScheduled, "", c.gsWatch)
	generateGsEvents(16, agonesv1.GameServerStateStarting, "", c.gsWatch)
	generateGsEvents(1, agonesv1.GameServerStateUnhealthy, "", c.gsWatch)

	c.sync()
	report()

	assert.Nil(t, testutil.GatherAndCompare(registry, strings.NewReader(gsTotalExpected), "agones_gameservers_total"))
}

func TestControllerFleetReplicasCount(t *testing.T) {

	registry := prometheus.NewRegistry()
	_, err := RegisterPrometheusExporter(registry)
	assert.Nil(t, err)

	c := newFakeController()
	defer c.close()
	c.run(t)

	f := fleet("fleet-test", 8, 2, 5, 1)
	fd := fleet("fleet-deleted", 100, 100, 100, 100)
	c.fleetWatch.Add(f)
	f = f.DeepCopy()
	f.Status.ReadyReplicas = 1
	f.Spec.Replicas = 5
	c.fleetWatch.Modify(f)
	c.fleetWatch.Add(fd)
	c.fleetWatch.Delete(fd)

	c.sync()
	report()

	assert.Nil(t, testutil.GatherAndCompare(registry, strings.NewReader(fleetReplicasCountExpected), "agones_fleets_replicas_count"))
}

func TestControllerFleetAutoScalerState(t *testing.T) {
	registry := prometheus.NewRegistry()
	_, err := RegisterPrometheusExporter(registry)
	assert.Nil(t, err)

	c := newFakeController()
	defer c.close()
	c.run(t)

	// testing fleet name change
	fasFleetNameChange := fleetAutoScaler("first-fleet", "name-switch")
	c.fasWatch.Add(fasFleetNameChange)
	fasFleetNameChange = fasFleetNameChange.DeepCopy()
	fasFleetNameChange.Spec.Policy.Buffer.BufferSize = intstr.FromInt(10)
	fasFleetNameChange.Spec.Policy.Buffer.MaxReplicas = 50
	fasFleetNameChange.Spec.Policy.Buffer.MinReplicas = 10
	fasFleetNameChange.Status.CurrentReplicas = 20
	fasFleetNameChange.Status.DesiredReplicas = 10
	fasFleetNameChange.Status.ScalingLimited = true
	c.fasWatch.Modify(fasFleetNameChange)
	fasFleetNameChange = fasFleetNameChange.DeepCopy()
	fasFleetNameChange.Spec.FleetName = "second-fleet"
	c.fasWatch.Modify(fasFleetNameChange)
	// testing deletion
	fasDeleted := fleetAutoScaler("deleted-fleet", "deleted")
	fasDeleted.Spec.Policy.Buffer.BufferSize = intstr.FromString("50%")
	fasDeleted.Spec.Policy.Buffer.MaxReplicas = 150
	fasDeleted.Spec.Policy.Buffer.MinReplicas = 15
	c.fasWatch.Add(fasDeleted)
	c.fasWatch.Delete(fasDeleted)

	c.sync()
	report()

	assert.Nil(t, testutil.GatherAndCompare(registry, strings.NewReader(fasStateExpected),
		"agones_fleet_autoscalers_able_to_scale", "agones_fleet_autoscalers_buffer_limits", "agones_fleet_autoscalers_buffer_size",
		"agones_fleet_autoscalers_current_replicas_count", "agones_fleet_autoscalers_desired_replicas_count", "agones_fleet_autoscalers_limited"))

}

func TestControllerGameServersNodeState(t *testing.T) {
	registry := prometheus.NewRegistry()
	_, err := RegisterPrometheusExporter(registry)
	assert.Nil(t, err)

	c := newFakeController()
	defer c.close()

	c.nodeWatch.Add(nodeWithName("node1"))
	c.nodeWatch.Add(nodeWithName("node2"))
	c.nodeWatch.Add(nodeWithName("node3"))
	c.gsWatch.Add(gameServerWithNode("node1"))
	c.gsWatch.Add(gameServerWithNode("node2"))
	c.gsWatch.Add(gameServerWithNode("node2"))

	c.sync()
	c.collect()
	report()

	if err := testutil.GatherAndCompare(registry, strings.NewReader(nodeCountExpected), "agones_nodes_count", "agones_gameservers_node_count"); err != nil {
		t.Fatal(err)
	}
}
