package main

import (
	"flag"
	"os"
	"os/user"
	"path/filepath"
	"sync"

	//	"time"

	stableV1alpha1 "agones.dev/agones/pkg/apis/stable/v1alpha1"
	//allocationv1alpha1 "agones.dev/agones/pkg/client/clientset/versioned/typed/allocation/v1alpha1"
	"agones.dev/agones/pkg/apis/allocation/v1alpha1"

	e2eframework "agones.dev/agones/test/e2e/framework"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultNs = "default"

var framework *e2eframework.Framework

//var agonesClient versioned.Interface

func main() {
	usr, _ := user.Current()
	kubeconfig := flag.String("kubeconfig", filepath.Join(usr.HomeDir, "/.kube/config"),
		"kube config path, e.g. $HOME/.kube/config")

	flag.Parse()

	logrus.SetFormatter(&logrus.TextFormatter{
		EnvironmentOverrideColors: true,
		FullTimestamp:             true,
		TimestampFormat:           "2006-01-02 15:04:05.000",
	})

	var (
		err error
	)

	cnt := 1
	//for {
	if framework, err = e2eframework.New(*kubeconfig); err != nil {
		logrus.Printf("failed to setup framework: %v\n", err)
		os.Exit(1)
	}

	logrus.Infof("Starting Allocation %v", cnt)
	allocate(120)
	logrus.Infof("Finished Allocation.")
	logrus.Infof("=======================================================================")
	logrus.Infof("=======================================================================")
	logrus.Infof("=======================================================================")
	cnt = cnt + 1
	//time.Sleep(30 * time.Minute)
	//}
	//framework.CleanUp("default")

}

// allocate does allocation
func allocate(numOfClients int) {
	//label := map[string]string{"role": "zz-simple-udp"}

	gsa := &v1alpha1.GameServerAllocation{ObjectMeta: metav1.ObjectMeta{GenerateName: "allocation-"},
		Spec: v1alpha1.GameServerAllocationSpec{
			Required: metav1.LabelSelector{MatchLabels: map[string]string{stableV1alpha1.FleetNameLabel: "simple-udp-packed"}},
			Preferred: []metav1.LabelSelector{
				{MatchLabels: map[string]string{stableV1alpha1.FleetNameLabel: "simple-udp-packed"}},
			},
		}}

	//logrus.Infof("Starting Allocation.")
	var wg sync.WaitGroup

	// Allocate GS by numOfClients in parallel while the fleet is scaling down
	for i := 0; i < numOfClients; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			for j := 0; j < 160; j++ {
				gsa1, err := framework.AgonesClient.Allocation().GameServerAllocations(defaultNs).Create(gsa.DeepCopy())
				if err == nil {
					//gsa1.Status.GameServerName == "" ||
					if gsa1.Status.State == "Contention" {
						logrus.Errorf("could not allocate : %v", gsa1.Status.State)
					}
					//allocatedGS.LoadOrStore(gsa1.Status.GameServerName, true)
					//logrus.Infof("Allocated gsa1 allocation : %v", gsa1.Status.GameServerName)
				} else {
					logrus.Errorf("could not completed gsa1 allocation : %v", err)
				}
			}
		}()
	}

	//time.Sleep(5 * time.Second)
	// scale down further while allocating
	//scaleFleetPatch(t, preferred, preferred.Spec.Replicas-10)

	wg.Wait()
	//logrus.Infof("Finished Allocation.")
}

// func createFleet() {
// 	fleets := framework.AgonesClient.StableV1alpha1().Fleets(defaultNs)
// 	//label := map[string]string{"role": t.Name()}

// 	preferred := defaultFleet()
// 	preferred.ObjectMeta.Name = "preferred"
// 	preferred.Spec.Replicas = 1100
// 	preferred.Spec.Template.ObjectMeta.Labels = label
// 	preferred, err := fleets.Create(preferred)
// 	if err != nil {
// 		os.Exit(1)
// 	}

// 	framework.WaitForFleetCondition(t, preferred, e2eframework.FleetReadyCount(preferred.Spec.Replicas))
// }
