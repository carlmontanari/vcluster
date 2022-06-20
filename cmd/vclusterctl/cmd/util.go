package cmd

import (
	"context"
	"github.com/loft-sh/vcluster/cmd/vclusterctl/cmd/app/podprinter"
	"github.com/loft-sh/vcluster/cmd/vclusterctl/cmd/find"
	"github.com/loft-sh/vcluster/cmd/vclusterctl/log"
	"github.com/loft-sh/vcluster/pkg/util/kubeconfig"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CriticalStatus container status
var CriticalStatus = map[string]bool{
	"Error":                      true,
	"Unknown":                    true,
	"ImagePullBackOff":           true,
	"CrashLoopBackOff":           true,
	"RunContainerError":          true,
	"ErrImagePull":               true,
	"CreateContainerConfigError": true,
	"InvalidImageName":           true,
}

var SortPodsByNewest = func(pods []corev1.Pod, i, j int) bool {
	return pods[i].CreationTimestamp.Unix() > pods[j].CreationTimestamp.Unix()
}

// GetKubeConfig attempts to read the kubeconfig from the default Secret and
// falls back to reading from filesystem if the Secret is not read successfully.
// Reading from filesystem is implemented for the backward compatibility and
// can be eventually removed in the future.
//
// This is retried until the kube config is successfully retrieve, or until 10 minute timeout is reached.
func GetKubeConfig(ctx context.Context, kubeClient *kubernetes.Clientset, vclusterName string, namespace string, log log.Logger) (*api.Config, error) {
	var kubeConfig *api.Config

	printedWaiting := false
	podInfoPrinter := podprinter.PodInfoPrinter{LastWarning: time.Now().Add(time.Second * 6)}
	err := wait.PollImmediate(time.Second, time.Minute*10, func() (done bool, err error) {
		podList, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=vcluster,release=" + vclusterName,
		})
		if err != nil {
			return false, err
		} else if len(podList.Items) > 0 {
			sort.Slice(podList.Items, func(i, j int) bool {
				return SortPodsByNewest(podList.Items, i, j)
			})
			if HasPodProblem(&podList.Items[0]) {
				if !printedWaiting {
					log.Infof("Waiting for vcluster to come up...")
					printedWaiting = true
				}

				podInfoPrinter.PrintPodWarning(ctx, kubeClient, &podList.Items[0], log)
				return false, nil
			} else if !allContainersReady(&podList.Items[0]) {
				if !printedWaiting {
					log.Infof("Waiting for vcluster to come up...")
					printedWaiting = true
				}

				if find.GetPodStatus(&podList.Items[0]) != "Running" {
					podInfoPrinter.PrintPodInfo(&podList.Items[0], log)
				}
				return false, nil
			}
		}

		kubeConfig, err = kubeconfig.ReadKubeConfig(ctx, kubeClient, vclusterName, namespace)
		if err != nil {
			if !printedWaiting {
				log.Infof("Waiting for vcluster to come up...")
				printedWaiting = true
			}

			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "wait for vcluster")
	}

	return kubeConfig, nil
}

func allContainersReady(pod *corev1.Pod) bool {
	for _, cs := range pod.Status.ContainerStatuses {
		if !cs.Ready || cs.State.Running == nil {
			return false
		}
	}
	return true
}

func HasPodProblem(pod *corev1.Pod) bool {
	status := find.GetPodStatus(pod)
	status = strings.TrimPrefix(status, "Init:")
	return CriticalStatus[status]
}

func updateKubeConfig(contextName string, cluster *api.Cluster, authInfo *api.AuthInfo, setActive bool) error {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{}).RawConfig()
	if err != nil {
		return err
	}

	config.Clusters[contextName] = cluster
	config.AuthInfos[contextName] = authInfo

	// Update kube context
	context := api.NewContext()
	context.Cluster = contextName
	context.AuthInfo = contextName

	config.Contexts[contextName] = context
	if setActive {
		config.CurrentContext = contextName
	}

	// Save the config
	return clientcmd.ModifyConfig(clientcmd.NewDefaultClientConfigLoadingRules(), config, false)
}

func randomPort() int {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 10; i++ {
		port := 10000 + rand.Intn(3000)
		s, err := checkPort(port)
		if s && err == nil {
			return port
		}
	}

	// just try another port
	return 10000 + rand.Intn(3000)
}

func checkPort(port int) (status bool, err error) {
	// Concatenate a colon and the port
	host := "localhost:" + strconv.Itoa(port)

	// Try to create a server with the port
	server, err := net.Listen("tcp", host)

	// if it fails then the port is likely taken
	if err != nil {
		return false, err
	}

	// close the server
	_ = server.Close()

	// we successfully used and closed the port
	// so it's now available to be used again
	return true, nil
}

func RESTConfigFromAPIConfig(contextName string, config *api.Config) (*rest.Config, error) {
	var clusterConfig *api.Cluster
	var authConfig *api.AuthInfo

	for _, c := range config.Clusters {
		clusterConfig = c
	}

	for _, a := range config.AuthInfos {
		authConfig = a
	}

	c := api.NewContext()
	c.Cluster = contextName
	c.AuthInfo = contextName

	clientConfig := api.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       map[string]*api.Cluster{contextName: clusterConfig},
		Contexts:       map[string]*api.Context{contextName: c},
		CurrentContext: contextName,
		AuthInfos:      map[string]*api.AuthInfo{contextName: authConfig},
	}

	configB, err := clientcmd.Write(clientConfig)
	if err != nil {
		return nil, err
	}

	return clientcmd.RESTConfigFromKubeConfig(configB)
}
