package main

import (
	"flag"
	"os"
	"os/user"
	"path/filepath"
	"sync"

	"agones.dev/agones/pkg/apis/allocation/v1alpha1"
	stableV1alpha1 "agones.dev/agones/pkg/apis/stable/v1alpha1"

	e2eframework "agones.dev/agones/test/e2e/framework"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultNs = "default"

var framework *e2eframework.Framework

func main() {
	usr, _ := user.Current()
	kubeconfig := flag.String("kubeconfig", filepath.Join(usr.HomeDir, "/.kube/config"),
		"kube config path, e.g. $HOME/.kube/config")

	fleetName := flag.String("fleet_name", "simple-udp-packed", "The fleet name that the tests will run against")
	qps := flag.Int("qps", 1000, "The QPS value that will overwrite the default value")
	burst := flag.Int("burst", 1000, "The Burst value that will overwrite the default value")
	clientCnt := flag.Int("burst", 120, "The number of concurrent clients")
	flag.Parse()

	logrus.SetFormatter(&logrus.TextFormatter{
		EnvironmentOverrideColors: true,
		FullTimestamp:             true,
		TimestampFormat:           "2006-01-02 15:04:05.000",
	})

	var err error

	if framework, err = e2eframework.NewForLoadTesting(*kubeconfig, float32(*qps), *burst); err != nil {
		logrus.Printf("failed to setup framework: %v\n", err)
		os.Exit(1)
	}

	logrus.Info("Starting Allocation")
	allocate(*clientCnt, *fleetName)
	logrus.Infof("Finished Allocation.")
	logrus.Infof("=======================================================================")
	logrus.Infof("=======================================================================")
	logrus.Infof("=======================================================================")
}

// allocate does allocation
func allocate(numOfClients int, fleetName string) {
	gsa := &v1alpha1.GameServerAllocation{ObjectMeta: metav1.ObjectMeta{GenerateName: "allocation-"},
		Spec: v1alpha1.GameServerAllocationSpec{
			Required: metav1.LabelSelector{MatchLabels: map[string]string{stableV1alpha1.FleetNameLabel: "simple-udp-packed"}},
			Preferred: []metav1.LabelSelector{
				{MatchLabels: map[string]string{stableV1alpha1.FleetNameLabel: fleetName}},
			},
		}}
	var wg sync.WaitGroup

	// Allocate GS by numOfClients in parallel
	for i := 0; i < numOfClients; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			for j := 0; j < 160; j++ {
				gsa1, err := framework.AgonesClient.Allocation().GameServerAllocations(defaultNs).Create(gsa.DeepCopy())
				if err == nil {
					if gsa1.Status.State == "Contention" {
						logrus.Errorf("could not allocate : %v", gsa1.Status.State)
					}
				} else {
					logrus.Errorf("could not completed gsa1 allocation : %v", err)
				}
			}
		}()
	}

	wg.Wait()
}
