package find

import (
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"testing"
	"time"
)

func int32Ptr(i int32) *int32 { return &i }

type fakeClientConfig struct{}

func (f *fakeClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return clientcmdapi.Config{}, nil
}

func (f *fakeClientConfig) ClientConfig() (*restclient.Config, error) {
	return nil, nil
}

func (f *fakeClientConfig) Namespace() (string, bool, error) {
	return "current-namespace", false, nil
}

func (f *fakeClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return nil
}

func TestFindInContext(t *testing.T) {
	cases := map[string]struct {
		expected []struct {
			name      string
			namespace string
		}
		searchNamespace string
		fakeObjects     []runtime.Object
	}{
		"no-vclusters": {
			expected: []struct {
				name      string
				namespace string
			}{},
			searchNamespace: "",
			fakeObjects:     []runtime.Object{},
		},
		"vcluster-deployment": {
			expected: []struct {
				name      string
				namespace string
			}{
				{
					name:      "my-vcluster",
					namespace: "my-vcluster-ns",
				},
			},
			searchNamespace: "",
			fakeObjects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-vcluster",
						Namespace: "my-vcluster-ns",
						Labels: map[string]string{
							"app":     "vcluster",
							"release": "my-vcluster",
						},
					},
				},
			},
		},
		"vcluster-statefulset": {
			expected: []struct {
				name      string
				namespace string
			}{
				{
					name:      "my-vcluster",
					namespace: "my-vcluster-ns",
				},
			},
			searchNamespace: "",
			fakeObjects: []runtime.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-vcluster",
						Namespace: "my-vcluster-ns",
						Labels: map[string]string{
							"app":     "vcluster",
							"release": "my-vcluster",
						},
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(1),
					},
				},
			},
		},
		"vcluster-duplicate-devspace": {
			expected: []struct {
				name      string
				namespace string
			}{
				{
					name:      "my-vcluster",
					namespace: "my-vcluster-ns",
				},
			},
			searchNamespace: "",
			fakeObjects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-vcluster",
						Namespace: "my-vcluster-ns",
						Labels: map[string]string{
							"app":     "vcluster",
							"release": "my-vcluster",
						},
					},
				},
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-vcluster",
						Namespace: "my-vcluster-ns",
						Labels: map[string]string{
							"app":     "vcluster",
							"release": "my-vcluster",
						},
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(0),
					},
				},
			},
		},
		"vcluster-duplicate-actual": {
			expected: []struct {
				name      string
				namespace string
			}{
				{
					name:      "my-vcluster",
					namespace: "my-vcluster-ns",
				},
				{
					name:      "my-vcluster",
					namespace: "my-vcluster-ns",
				},
			},
			searchNamespace: "",
			fakeObjects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-vcluster",
						Namespace: "my-vcluster-ns",
						Labels: map[string]string{
							"app":     "vcluster",
							"release": "my-vcluster",
						},
					},
				},
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-vcluster",
						Namespace: "my-vcluster-ns",
						Labels: map[string]string{
							"app":     "vcluster",
							"release": "my-vcluster",
						},
					},
					Spec: appsv1.StatefulSetSpec{
						Replicas: int32Ptr(1),
					},
				},
			},
		},
	}

	for testName, testCase := range cases {
		t.Run(testName, func(t *testing.T) {
			t.Logf("%s: starting", testName)

			client := fake.NewSimpleClientset(testCase.fakeObjects...)

			vclusters, err := findInContext(
				"ignored",
				"my-vcluster",
				testCase.searchNamespace,
				1*time.Second,
				client,
				&fakeClientConfig{},
			)
			if err != nil {
				t.Fatal(err)
			}

			if len(testCase.expected) != len(vclusters) {
				t.Fatalf(
					"expected %d vclusters returned, got %d",
					len(testCase.expected), len(vclusters),
				)
			}

			for _, expected := range testCase.expected {
				for _, actual := range vclusters {
					if expected.name != actual.Name || expected.namespace != actual.Namespace {
						t.Fatalf(
							"expected vcluster '%s/%s', got '%s/%s'",
							expected.name, expected.namespace, actual.Name, actual.Namespace,
						)
					}
				}
			}
		})
	}
}
