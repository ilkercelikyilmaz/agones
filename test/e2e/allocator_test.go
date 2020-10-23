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

package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	pb "agones.dev/agones/pkg/allocation/go/v1alpha1"
	agonesv1 "agones.dev/agones/pkg/apis/agones/v1"
	multiclusterv1alpha1 "agones.dev/agones/pkg/apis/multicluster/v1alpha1"
	e2e "agones.dev/agones/test/e2e/framework"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	agonesSystemNamespace = "agones-system"
	allocatorServiceName  = "agones-allocator"
	allocatorTLSName      = "allocator-tls"
	allocatorClientCAName = "allocator-client-ca"
	tlsCrtTag             = "tls.crt"
	tlsKeyTag             = "tls.key"
	serverCATag           = "ca.crt"
	allocatorReqURLFmt    = "%s:%d"
)

func TestAllocator(t *testing.T) {
	ip, port := getAllocatorEndpoint(t)
	requestURL := fmt.Sprintf(allocatorReqURLFmt, ip, port)
	tlsCA := refreshAllocatorTLSCerts(t, ip)

	namespace := fmt.Sprintf("allocator-%s", uuid.NewUUID())
	framework.CreateNamespace(t, namespace)
	defer framework.DeleteNamespace(t, namespace)

	clientSecretName := fmt.Sprintf("allocator-client-%s", uuid.NewUUID())
	genClientSecret(t, tlsCA, namespace, clientSecretName)

	flt, err := createFleet(namespace)
	if !assert.Nil(t, err) {
		return
	}
	framework.AssertFleetCondition(t, flt, e2e.FleetReadyCount(flt.Spec.Replicas))
	request := &pb.AllocationRequest{
		Namespace:                    namespace,
		RequiredGameServerSelector:   &pb.LabelSelector{MatchLabels: map[string]string{agonesv1.FleetNameLabel: flt.ObjectMeta.Name}},
		PreferredGameServerSelectors: []*pb.LabelSelector{{MatchLabels: map[string]string{agonesv1.FleetNameLabel: flt.ObjectMeta.Name}}},
		Scheduling:                   pb.AllocationRequest_Packed,
		MetaPatch:                    &pb.MetaPatch{Labels: map[string]string{"gslabel": "allocatedbytest"}},
	}

	// wait for the allocation system to come online
	err = wait.PollImmediate(2*time.Second, 5*time.Minute, func() (bool, error) {
		// create the grpc client each time, as we may end up looking at an old cert
		dialOpts, err := createRemoteClusterDialOption(namespace, clientSecretName)
		if err != nil {
			return false, err
		}

		conn, err := grpc.Dial(requestURL, dialOpts)
		if err != nil {
			logrus.WithError(err).Info("failing grpc.Dial")
			return false, nil
		}
		defer conn.Close() // nolint: errcheck

		grpcClient := pb.NewAllocationServiceClient(conn)
		response, err := grpcClient.PostAllocate(context.Background(), request)
		if err != nil {
			logrus.WithError(err).Info("failing PostAllocate request")
			return false, nil
		}
		assert.Equal(t, pb.AllocationResponse_Allocated, response.State)
		return true, nil
	})

	if !assert.NoError(t, err) {
		assert.FailNow(t, "Http test failed")
	}
}

// Tests multi-cluster allocation by reusing the same cluster but across namespace.
// Multi-cluster is represented as two namespaces A and B in the same cluster.
// Namespace A received the allocation request, but because namespace B has the highest priority, A will forward the request to B.
func TestAllocatorCrossNamespace(t *testing.T) {
	ip, port := getAllocatorEndpoint(t)
	requestURL := fmt.Sprintf(allocatorReqURLFmt, ip, port)
	tlsCA := refreshAllocatorTLSCerts(t, ip)
	restartAllocator(t)

	// Create namespaces A and B
	namespaceA := fmt.Sprintf("allocator-a-%s", uuid.NewUUID())
	framework.CreateNamespace(t, namespaceA)
	defer framework.DeleteNamespace(t, namespaceA)
	namespaceB := fmt.Sprintf("allocator-b-%s", uuid.NewUUID())
	framework.CreateNamespace(t, namespaceB)
	defer framework.DeleteNamespace(t, namespaceB)

	// Create client secret A, B is receiver of the request and does not need client secret
	clientSecretNameA := fmt.Sprintf("allocator-client-%s", uuid.NewUUID())
	genClientSecret(t, tlsCA, namespaceA, clientSecretNameA)

	policyName := fmt.Sprintf("a-to-b-%s", uuid.NewUUID())
	p := &multiclusterv1alpha1.GameServerAllocationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespaceA,
		},
		Spec: multiclusterv1alpha1.GameServerAllocationPolicySpec{
			Priority: 1,
			Weight:   1,
			ConnectionInfo: multiclusterv1alpha1.ClusterConnectionInfo{
				SecretName:          clientSecretNameA,
				Namespace:           namespaceB,
				AllocationEndpoints: []string{ip},
			},
		},
	}
	createAllocationPolicy(t, p)

	// Create a fleet in namespace B. Allocation should not happen in A according to policy
	flt, err := createFleet(namespaceB)
	if !assert.Nil(t, err) {
		return
	}
	framework.AssertFleetCondition(t, flt, e2e.FleetReadyCount(flt.Spec.Replicas))
	request := &pb.AllocationRequest{
		Namespace: namespaceA,
		// Enable multi-cluster setting
		MultiClusterSetting:        &pb.MultiClusterSetting{Enabled: true},
		RequiredGameServerSelector: &pb.LabelSelector{MatchLabels: map[string]string{agonesv1.FleetNameLabel: flt.ObjectMeta.Name}},
	}

	// wait for the allocation system to come online
	err = wait.PollImmediate(2*time.Second, 5*time.Minute, func() (bool, error) {
		// create the grpc client each time, as we may end up looking at an old cert
		dialOpts, err := createRemoteClusterDialOption(namespaceA, clientSecretNameA)
		if err != nil {
			return false, err
		}

		conn, err := grpc.Dial(requestURL, dialOpts)
		if err != nil {
			logrus.WithError(err).Info("failing http request")
			return false, nil
		}
		defer conn.Close() // nolint: errcheck

		grpcClient := pb.NewAllocationServiceClient(conn)
		response, err := grpcClient.PostAllocate(context.Background(), request)
		if err != nil {
			logrus.WithError(err).Info("failing PostAllocate request")
			return false, nil
		}
		assert.Equal(t, pb.AllocationResponse_Allocated, response.State)
		return true, nil
	})

	if !assert.NoError(t, err) {
		assert.FailNow(t, "Http test failed")
	}
}

func createAllocationPolicy(t *testing.T, p *multiclusterv1alpha1.GameServerAllocationPolicy) {
	t.Helper()

	mc := framework.AgonesClient.MulticlusterV1alpha1()
	policy, err := mc.GameServerAllocationPolicies(p.Namespace).Create(p)
	if err != nil {
		t.Fatalf("creating allocation policy failed: %s", err)
	}
	t.Logf("created allocation policy %v", policy)
}

func getAllocatorEndpoint(t *testing.T) (string, int32) {
	kubeCore := framework.KubeClient.CoreV1()
	svc, err := kubeCore.Services(agonesSystemNamespace).Get(allocatorServiceName, metav1.GetOptions{})
	if !assert.Nil(t, err) {
		t.FailNow()
	}
	if !assert.NotNil(t, svc.Status.LoadBalancer) {
		t.FailNow()
	}
	if !assert.Equal(t, 1, len(svc.Status.LoadBalancer.Ingress)) {
		t.FailNow()
	}
	if !assert.NotNil(t, 0, svc.Status.LoadBalancer.Ingress[0].IP) {
		t.FailNow()
	}

	port := svc.Spec.Ports[0]
	return svc.Status.LoadBalancer.Ingress[0].IP, port.Port
}

// createRemoteClusterDialOption creates a grpc client dial option with proper certs to make a remote call.
func createRemoteClusterDialOption(namespace, clientSecretName string) (grpc.DialOption, error) {
	kubeCore := framework.KubeClient.CoreV1()
	clientSecret, err := kubeCore.Secrets(namespace).Get(clientSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Errorf("getting client secret %s/%s failed: %s", namespace, clientSecretName, err)
	}

	// Create http client using cert
	clientCert := clientSecret.Data[tlsCrtTag]
	clientKey := clientSecret.Data[tlsKeyTag]
	tlsCA := clientSecret.Data[serverCATag]
	if clientCert == nil || clientKey == nil {
		return nil, errors.New("missing certificate")
	}

	// Load client cert
	cert, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, err
	}

	rootCA := x509.NewCertPool()
	if !rootCA.AppendCertsFromPEM(tlsCA) {
		return nil, errors.New("could not append PEM format CA cert")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      rootCA,
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}

func createFleet(namespace string) (*agonesv1.Fleet, error) {
	fleets := framework.AgonesClient.AgonesV1().Fleets(namespace)
	fleet := defaultFleet(namespace)
	return fleets.Create(fleet)
}

func restartAllocator(t *testing.T) {
	t.Helper()

	kubeCore := framework.KubeClient.CoreV1()
	pods, err := kubeCore.Pods(agonesSystemNamespace).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("listing pods failed: %s", err)
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if !strings.HasPrefix(pod.Name, allocatorServiceName) {
			continue
		}
		if err := kubeCore.Pods(agonesSystemNamespace).Delete(pod.Name, &metav1.DeleteOptions{}); err != nil {
			t.Fatalf("deleting pods failed: %s", err)
		}
	}
}

func genClientSecret(t *testing.T, serverCA []byte, namespace, secretName string) {
	t.Helper()

	pub, priv := generateTLSCertPair(t, "")

	kubeCore := framework.KubeClient.CoreV1()
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			tlsCrtTag:   pub,
			tlsKeyTag:   priv,
			serverCATag: serverCA,
		},
	}
	if _, err := kubeCore.Secrets(namespace).Create(s); err != nil {
		t.Fatalf("Creating secret %s/%s failed: %s", namespace, secretName, err)
	}
	t.Logf("Client secret is created: %v", s)

	// Add client CA to authorized client CAs
	s, err := kubeCore.Secrets(agonesSystemNamespace).Get(allocatorClientCAName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting secret %s/%s failed: %s", agonesSystemNamespace, allocatorClientCAName, err)
	}
	s.Data["ca.crt"] = serverCA
	s.Data["client-ca.crt"] = pub
	clientCASecret, err := kubeCore.Secrets(agonesSystemNamespace).Update(s)
	if err != nil {
		t.Fatalf("updating secrets failed: %s", err)
	}
	t.Logf("Secret is updated: %v", clientCASecret)
}

func refreshAllocatorTLSCerts(t *testing.T, host string) []byte {
	t.Helper()

	pub, priv := generateTLSCertPair(t, host)
	// verify key pair
	if _, err := tls.X509KeyPair(pub, priv); err != nil {
		t.Fatalf("generated key pair failed create cert: %s", err)
	}

	kubeCore := framework.KubeClient.CoreV1()
	s, err := kubeCore.Secrets(agonesSystemNamespace).Get(allocatorTLSName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting secret %s/%s failed: %s", agonesSystemNamespace, allocatorTLSName, err)
	}
	s.Data[tlsCrtTag] = pub
	s.Data[tlsKeyTag] = priv
	if _, err := kubeCore.Secrets(agonesSystemNamespace).Update(s); err != nil {
		t.Fatalf("updating secrets failed: %s", err)
	}

	restartAllocator(t)

	t.Logf("Allocator TLS is refreshed with public CA: %s for endpoint %s", string(pub), host)
	return pub
}

func generateTLSCertPair(t *testing.T, host string) ([]byte, []byte) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key failed: %s", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		t.Fatalf("generating serial number failed: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   host,
			Organization: []string{"testing"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		SignatureAlgorithm:    x509.SHA1WithRSA,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	if host != "" {
		template.IPAddresses = []net.IP{net.ParseIP(host)}
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("creating certificate failed: %s", err)
	}
	pemPubBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshalling private key failed: %v", err)
	}
	pemPrivBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	return pemPubBytes, pemPrivBytes
}
