package install

import (
	"math/rand"
	"testing"

	agonesv1 "agones.dev/agones/pkg/apis/agones/v1"
	allocationv1 "agones.dev/agones/pkg/apis/allocation/v1"
	autoscalingv1 "agones.dev/agones/pkg/apis/autoscaling/v1"
	multiclusterv1alpha1 "agones.dev/agones/pkg/apis/multicluster/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/api/apitesting/roundtrip"
	genericfuzzer "k8s.io/apimachinery/pkg/apis/meta/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func TestRoundTripTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	localSchemeBuilder := runtime.SchemeBuilder{
		agonesv1.AddToScheme,
		allocationv1.AddToScheme,
		autoscalingv1.AddToScheme,
		multiclusterv1alpha1.AddToScheme,
	}
	seed := rand.Int63()
	localFuzzer := fuzzer.FuzzerFor(genericfuzzer.Funcs, rand.NewSource(seed), codecs)

	assert.NoError(t, localSchemeBuilder.AddToScheme(scheme))
	roundtrip.RoundTripExternalTypes(t, scheme, codecs, localFuzzer, nil)
}
