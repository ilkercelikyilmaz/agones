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

package e2e

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"agones.dev/agones/pkg/apis"
	agonesv1 "agones.dev/agones/pkg/apis/agones/v1"
	allocationv1 "agones.dev/agones/pkg/apis/allocation/v1"
	multiclusterv1alpha1 "agones.dev/agones/pkg/apis/multicluster/v1alpha1"
	e2e "agones.dev/agones/test/e2e/framework"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestCreateFleetAndGameServerAllocate(t *testing.T) {
	t.Parallel()

	fixtures := []apis.SchedulingStrategy{apis.Packed, apis.Distributed}

	for _, strategy := range fixtures {
		strategy := strategy
		t.Run(string(strategy), func(t *testing.T) {
			t.Parallel()

			fleets := framework.AgonesClient.AgonesV1().Fleets(defaultNs)
			fleet := defaultFleet(defaultNs)
			fleet.Spec.Scheduling = strategy
			flt, err := fleets.Create(fleet)
			if assert.Nil(t, err) {
				defer fleets.Delete(flt.ObjectMeta.Name, nil) // nolint:errcheck
			}

			framework.AssertFleetCondition(t, flt, e2e.FleetReadyCount(flt.Spec.Replicas))

			gsa := &allocationv1.GameServerAllocation{
				Spec: allocationv1.GameServerAllocationSpec{
					Scheduling: strategy,
					Required:   metav1.LabelSelector{MatchLabels: map[string]string{agonesv1.FleetNameLabel: flt.ObjectMeta.Name}},
				}}

			gsa, err = framework.AgonesClient.AllocationV1().GameServerAllocations(fleet.ObjectMeta.Namespace).Create(gsa)
			if assert.Nil(t, err) {
				assert.Equal(t, string(allocationv1.GameServerAllocationAllocated), string(gsa.Status.State))
			}
		})
	}
}

func TestMultiClusterAllocationOnLocalCluster(t *testing.T) {
	t.Parallel()

	fixtures := []apis.SchedulingStrategy{apis.Packed, apis.Distributed}
	for _, strategy := range fixtures {
		strategy := strategy
		t.Run(string(strategy), func(t *testing.T) {
			t.Parallel()

			namespace := fmt.Sprintf("gsa-multicluster-local-%s", uuid.NewUUID())
			framework.CreateNamespace(t, namespace)
			defer framework.DeleteNamespace(t, namespace)

			fleets := framework.AgonesClient.AgonesV1().Fleets(namespace)
			fleet := defaultFleet(namespace)
			fleet.Spec.Scheduling = strategy
			flt, err := fleets.Create(fleet)
			if assert.Nil(t, err) {
				defer fleets.Delete(flt.ObjectMeta.Name, nil) // nolint:errcheck
			}

			framework.AssertFleetCondition(t, flt, e2e.FleetReadyCount(flt.Spec.Replicas))

			// Allocation Policy #1: local cluster with desired label.
			// This policy allocates locally on the cluster due to matching namespace with gsa and not setting AllocationEndpoints.
			mca := &multiclusterv1alpha1.GameServerAllocationPolicy{
				Spec: multiclusterv1alpha1.GameServerAllocationPolicySpec{
					Priority: 1,
					Weight:   100,
					ConnectionInfo: multiclusterv1alpha1.ClusterConnectionInfo{
						ClusterName: "multicluster1",
						SecretName:  "sec1",
						Namespace:   namespace,
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels:       map[string]string{"cluster": "onprem"},
					GenerateName: "allocationpolicy-",
				},
			}
			resp, err := framework.AgonesClient.MulticlusterV1alpha1().GameServerAllocationPolicies(fleet.ObjectMeta.Namespace).Create(mca)
			if assert.Nil(t, err) {
				assert.Equal(t, mca.Spec, resp.Spec)
			}

			// Allocation Policy #2: another cluster with desired label, but lower priority.
			// If the policy is selected due to a bug the request fails as it cannot find the secret.
			mca = &multiclusterv1alpha1.GameServerAllocationPolicy{
				Spec: multiclusterv1alpha1.GameServerAllocationPolicySpec{
					Priority: 2,
					Weight:   100,
					ConnectionInfo: multiclusterv1alpha1.ClusterConnectionInfo{
						AllocationEndpoints: []string{"another-endpoint"},
						ClusterName:         "multicluster2",
						SecretName:          "sec2",
						Namespace:           namespace,
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels:       map[string]string{"cluster": "onprem"},
					GenerateName: "allocationpolicy-",
				},
			}
			resp, err = framework.AgonesClient.MulticlusterV1alpha1().GameServerAllocationPolicies(fleet.ObjectMeta.Namespace).Create(mca)
			assert.Nil(t, err)
			if assert.Nil(t, err) {
				assert.Equal(t, mca.Spec, resp.Spec)
			}

			// Allocation Policy #3: another cluster with highest priority, but missing desired label (will not be selected)
			mca = &multiclusterv1alpha1.GameServerAllocationPolicy{
				Spec: multiclusterv1alpha1.GameServerAllocationPolicySpec{
					Priority: 1,
					Weight:   10,
					ConnectionInfo: multiclusterv1alpha1.ClusterConnectionInfo{
						AllocationEndpoints: []string{"another-endpoint"},
						ClusterName:         "multicluster3",
						SecretName:          "sec3",
						Namespace:           namespace,
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "allocationpolicy-",
				},
			}
			resp, err = framework.AgonesClient.MulticlusterV1alpha1().GameServerAllocationPolicies(fleet.ObjectMeta.Namespace).Create(mca)
			assert.Nil(t, err)
			if assert.Nil(t, err) {
				assert.Equal(t, mca.Spec, resp.Spec)
			}

			gsa := &allocationv1.GameServerAllocation{
				Spec: allocationv1.GameServerAllocationSpec{
					Scheduling: strategy,
					Required:   metav1.LabelSelector{MatchLabels: map[string]string{agonesv1.FleetNameLabel: flt.ObjectMeta.Name}},
					MultiClusterSetting: allocationv1.MultiClusterSetting{
						Enabled: true,
						PolicySelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"cluster": "onprem",
							},
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					ClusterName:  "multicluster1",
					GenerateName: "allocation-",
					Namespace:    namespace,
				},
			}

			gsa, err = framework.AgonesClient.AllocationV1().GameServerAllocations(fleet.ObjectMeta.Namespace).Create(gsa)
			if assert.Nil(t, err) {
				assert.Equal(t, string(allocationv1.GameServerAllocationAllocated), string(gsa.Status.State))
			}
		})
	}
}

// Can't allocate more GameServers if a fleet is fully used.
func TestCreateFullFleetAndCantGameServerAllocate(t *testing.T) {
	t.Parallel()

	fixtures := []apis.SchedulingStrategy{apis.Packed, apis.Distributed}

	for _, strategy := range fixtures {
		strategy := strategy

		t.Run(string(strategy), func(t *testing.T) {
			t.Parallel()

			fleets := framework.AgonesClient.AgonesV1().Fleets(defaultNs)
			fleet := defaultFleet(defaultNs)
			fleet.Spec.Scheduling = strategy
			flt, err := fleets.Create(fleet)
			if assert.Nil(t, err) {
				defer fleets.Delete(flt.ObjectMeta.Name, nil) // nolint:errcheck
			}

			framework.AssertFleetCondition(t, flt, e2e.FleetReadyCount(flt.Spec.Replicas))

			gsa := &allocationv1.GameServerAllocation{
				Spec: allocationv1.GameServerAllocationSpec{
					Scheduling: strategy,
					Required:   metav1.LabelSelector{MatchLabels: map[string]string{agonesv1.FleetNameLabel: flt.ObjectMeta.Name}},
				}}

			for i := 0; i < replicasCount; i++ {
				var gsa2 *allocationv1.GameServerAllocation
				gsa2, err = framework.AgonesClient.AllocationV1().GameServerAllocations(defaultNs).Create(gsa.DeepCopy())
				if assert.Nil(t, err) {
					assert.Equal(t, allocationv1.GameServerAllocationAllocated, gsa2.Status.State)
				}
			}

			framework.AssertFleetCondition(t, flt, func(fleet *agonesv1.Fleet) bool {
				return fleet.Status.AllocatedReplicas == replicasCount
			})

			gsa, err = framework.AgonesClient.AllocationV1().GameServerAllocations(defaultNs).Create(gsa.DeepCopy())
			if assert.Nil(t, err) {
				assert.Equal(t, string(allocationv1.GameServerAllocationUnAllocated), string(gsa.Status.State))
			}
		})
	}
}

func TestGameServerAllocationMetaDataPatch(t *testing.T) {
	t.Parallel()

	gs := defaultGameServer(defaultNs)
	gs.ObjectMeta.Labels = map[string]string{"test": t.Name()}

	gs, err := framework.CreateGameServerAndWaitUntilReady(defaultNs, gs)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "could not create GameServer")
	}
	defer framework.AgonesClient.AgonesV1().GameServers(defaultNs).Delete(gs.ObjectMeta.Name, nil) // nolint: errcheck

	gsa := &allocationv1.GameServerAllocation{ObjectMeta: metav1.ObjectMeta{GenerateName: "allocation-"},
		Spec: allocationv1.GameServerAllocationSpec{
			Required: metav1.LabelSelector{MatchLabels: map[string]string{"test": t.Name()}},
			MetaPatch: allocationv1.MetaPatch{
				Labels:      map[string]string{"red": "blue"},
				Annotations: map[string]string{"dog": "good"},
			},
		}}

	err = wait.PollImmediate(time.Second, 30*time.Second, func() (bool, error) {
		gsa, err = framework.AgonesClient.AllocationV1().GameServerAllocations(defaultNs).Create(gsa.DeepCopy())

		if err != nil {
			return true, err
		}

		return allocationv1.GameServerAllocationAllocated == gsa.Status.State, nil
	})
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	gs, err = framework.AgonesClient.AgonesV1().GameServers(defaultNs).Get(gsa.Status.GameServerName, metav1.GetOptions{})
	if assert.Nil(t, err) {
		assert.Equal(t, "blue", gs.ObjectMeta.Labels["red"])
		assert.Equal(t, "good", gs.ObjectMeta.Annotations["dog"])
	}
}

func TestGameServerAllocationPreferredSelection(t *testing.T) {
	t.Parallel()

	fleets := framework.AgonesClient.AgonesV1().Fleets(defaultNs)
	gameServers := framework.AgonesClient.AgonesV1().GameServers(defaultNs)
	label := map[string]string{"role": t.Name()}

	preferred := defaultFleet(defaultNs)
	preferred.ObjectMeta.GenerateName = "preferred-"
	preferred.Spec.Replicas = 1
	preferred.Spec.Template.ObjectMeta.Labels = label
	preferred, err := fleets.Create(preferred)
	if assert.Nil(t, err) {
		defer fleets.Delete(preferred.ObjectMeta.Name, nil) // nolint:errcheck
	} else {
		assert.FailNow(t, "could not create first fleet")
	}

	required := defaultFleet(defaultNs)
	required.ObjectMeta.GenerateName = "required-"
	required.Spec.Replicas = 2
	required.Spec.Template.ObjectMeta.Labels = label
	required, err = fleets.Create(required)
	if assert.Nil(t, err) {
		defer fleets.Delete(required.ObjectMeta.Name, nil) // nolint:errcheck
	} else {
		assert.FailNow(t, "could not create second fleet")
	}

	framework.AssertFleetCondition(t, preferred, e2e.FleetReadyCount(preferred.Spec.Replicas))
	framework.AssertFleetCondition(t, required, e2e.FleetReadyCount(required.Spec.Replicas))

	gsa := &allocationv1.GameServerAllocation{ObjectMeta: metav1.ObjectMeta{GenerateName: "allocation-"},
		Spec: allocationv1.GameServerAllocationSpec{
			Required: metav1.LabelSelector{MatchLabels: label},
			Preferred: []metav1.LabelSelector{
				{MatchLabels: map[string]string{agonesv1.FleetNameLabel: preferred.ObjectMeta.Name}},
			},
		}}

	gsa1, err := framework.AgonesClient.AllocationV1().GameServerAllocations(defaultNs).Create(gsa.DeepCopy())
	if assert.Nil(t, err) {
		assert.Equal(t, allocationv1.GameServerAllocationAllocated, gsa1.Status.State)
		gs, err := gameServers.Get(gsa1.Status.GameServerName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, preferred.ObjectMeta.Name, gs.ObjectMeta.Labels[agonesv1.FleetNameLabel])
	} else {
		assert.FailNow(t, "could not completed gsa1 allocation")
	}

	gs2, err := framework.AgonesClient.AllocationV1().GameServerAllocations(defaultNs).Create(gsa.DeepCopy())
	if assert.Nil(t, err) {
		assert.Equal(t, allocationv1.GameServerAllocationAllocated, gs2.Status.State)
		gs, err := gameServers.Get(gs2.Status.GameServerName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, required.ObjectMeta.Name, gs.ObjectMeta.Labels[agonesv1.FleetNameLabel])
	} else {
		assert.FailNow(t, "could not completed gs2 allocation")
	}

	// delete the preferred gameserver, and then let's try allocating again, make sure it goes back to the
	// preferred one
	err = gameServers.Delete(gsa1.Status.GameServerName, nil)
	if !assert.Nil(t, err) {
		assert.FailNow(t, "could not delete gameserver")
	}

	// wait until the game server is deleted
	err = wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		_, err = gameServers.Get(gsa1.Status.GameServerName, metav1.GetOptions{})

		if err != nil && errors.IsNotFound(err) {
			return true, nil
		}

		return false, err
	})
	assert.Nil(t, err)

	// now wait for another one to come along
	framework.AssertFleetCondition(t, preferred, e2e.FleetReadyCount(preferred.Spec.Replicas))

	gsa3, err := framework.AgonesClient.AllocationV1().GameServerAllocations(defaultNs).Create(gsa.DeepCopy())
	if assert.Nil(t, err) {
		assert.Equal(t, allocationv1.GameServerAllocationAllocated, gsa3.Status.State)
		gs, err := gameServers.Get(gsa3.Status.GameServerName, metav1.GetOptions{})
		assert.Nil(t, err)
		assert.Equal(t, preferred.ObjectMeta.Name, gs.ObjectMeta.Labels[agonesv1.FleetNameLabel])
	}
}

func TestGameServerAllocationDeletionOnUnAllocate(t *testing.T) {
	t.Parallel()

	allocations := framework.AgonesClient.AllocationV1().GameServerAllocations(defaultNs)

	gsa := &allocationv1.GameServerAllocation{ObjectMeta: metav1.ObjectMeta{GenerateName: "allocation-"},
		Spec: allocationv1.GameServerAllocationSpec{
			Required: metav1.LabelSelector{MatchLabels: map[string]string{"never": "goingtohappen"}},
		}}

	gsa, err := allocations.Create(gsa.DeepCopy())
	if assert.Nil(t, err) {
		assert.Equal(t, allocationv1.GameServerAllocationUnAllocated, gsa.Status.State)
	}
}

func TestGameServerAllocationDuringMultipleAllocationClients(t *testing.T) {
	t.Parallel()

	fleets := framework.AgonesClient.AgonesV1().Fleets(defaultNs)
	label := map[string]string{"role": t.Name()}

	preferred := defaultFleet(defaultNs)
	preferred.ObjectMeta.GenerateName = "preferred-"
	preferred.Spec.Replicas = 150
	preferred.Spec.Template.ObjectMeta.Labels = label
	preferred, err := fleets.Create(preferred)
	if assert.Nil(t, err) {
		defer fleets.Delete(preferred.ObjectMeta.Name, nil) // nolint:errcheck
	} else {
		assert.FailNow(t, "could not create first fleet")
	}

	framework.AssertFleetCondition(t, preferred, e2e.FleetReadyCount(preferred.Spec.Replicas))

	// scale down before starting allocation
	preferred = scaleFleetPatch(t, preferred, preferred.Spec.Replicas-20)

	gsa := &allocationv1.GameServerAllocation{ObjectMeta: metav1.ObjectMeta{GenerateName: "allocation-"},
		Spec: allocationv1.GameServerAllocationSpec{
			Required: metav1.LabelSelector{MatchLabels: label},
			Preferred: []metav1.LabelSelector{
				{MatchLabels: map[string]string{agonesv1.FleetNameLabel: "preferred"}},
			},
		}}

	allocatedGS := sync.Map{}

	logrus.Infof("Starting Allocation.")
	var wg sync.WaitGroup

	// Allocate GS by 10 clients in parallel while the fleet is scaling down
	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				gsa1, err := framework.AgonesClient.AllocationV1().GameServerAllocations(defaultNs).Create(gsa.DeepCopy())
				if err == nil {
					allocatedGS.LoadOrStore(gsa1.Status.GameServerName, true)
				} else {
					t.Errorf("could not completed gsa1 allocation : %v", err)
				}
			}
		}()
	}

	time.Sleep(3 * time.Second)
	// scale down further while allocating
	scaleFleetPatch(t, preferred, preferred.Spec.Replicas-10)

	wg.Wait()
	logrus.Infof("Finished Allocation.")

	// count the number of unique game servers allocated
	// there should not be any duplicate
	uniqueAllocatedGSs := 0
	allocatedGS.Range(func(k, v interface{}) bool {
		uniqueAllocatedGSs++
		return true
	})
	assert.Equal(t, 100, uniqueAllocatedGSs)
}
