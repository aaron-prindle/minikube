/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/minikube/pkg/minikube/cluster"
	"k8s.io/minikube/pkg/minikube/machine"
	"k8s.io/minikube/pkg/minikube/service"
)

// tunnelCmd represents the mount command
var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "tunnels all services in the minikube vm to localhost",
	Long:  `tunnels all services in the minikube vm to localhost`,
	Run: func(cmd *cobra.Command, args []string) {

		api, err := machine.NewAPIClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting client: %s\n", err)
			os.Exit(1)
		}
		defer api.Close()
		cluster.EnsureMinikubeRunningOrExit(api, 1)
		host, err := cluster.CheckIfApiExistsAndLoad(api)
		if err != nil {
			panic("Error checking if api exist and loading it")
		}
		ip, err := host.Driver.GetIP()
		if err != nil {
			panic(err)
		}

		K8s := &service.K8sClientGetter{}
		client, err := K8s.GetCoreClient()
		if err != nil {
			// return nil, err
			panic(fmt.Sprintf("Error preparing connection to kubernetes cluster: %s", err))
		}

		fmt.Printf(`Starting minikube tunnel process. Press Ctrl+C to exit.\nAll cluster IPs and load balancers are now available from your host machine.\n`)
		//need sanity check to see if there's existing external-lb functionality, or maybe at least an overridable option to exit out if not running on minikube.

		router := *NewRouter(ip)

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigs
			router.deleteroutes()
			os.Exit(0)
		}()
		for {
			svc, err := client.Services("").List(metav1.ListOptions{})
			if err != nil {
				panic(fmt.Sprintf("Error getting services from kubernetes cluster: %s", err))
			}
			for _, svc := range svc.Items {
				if svc.Spec.Type == "LoadBalancer" && len(svc.Status.LoadBalancer.Ingress) == 0 {
					fmt.Printf("Service %q has no ingress for its loadbalancer\n", svc.Name)
					if addIngressToService(client, svc); err != nil {
						panic(fmt.Sprintf("Error patching service %s: %s", svc.Name, err))
					}
					router.addroute(svc.Spec.ClusterIP)
				} else if svc.Spec.Type == "LoadBalancer" {
					fmt.Printf("Service %q has IP %v for its loadbalancer\n", svc.Name, svc.Status.LoadBalancer.Ingress[0].IP) //need to check for hostnames and other types too.
				} else {
					fmt.Printf("Service %q is not type LoadBalancer\n", svc.Name)
				}
			}
			time.Sleep(1 * time.Second)
		}
	},
}

type Router struct {
	routetable []string
	minikubeip string
}

func NewRouter(ip string) *Router {
	return &Router{
		minikubeip: ip,
		routetable: []string{}}
}

func (r *Router) addroute(clusterip string) {
	r.routetable = append(r.routetable, clusterip)

	var routeCmd []string
	if runtime.GOOS == "windows" {
		routeCmd = []string{"NOT SURE"}
	} else if runtime.GOOS == "linux" {
		routeCmd = []string{"bash", "-c",
			fmt.Sprintf("sudo ip route add %s via %s", clusterip, r.minikubeip)}
	} else if runtime.GOOS == "darwin" {
		routeCmd = []string{"bash", "-c",
			fmt.Sprintf("sudo route -n add %s %s", clusterip, r.minikubeip)}
	} else {
		panic("unsupported os detected")
	}
	cmd := exec.Command(routeCmd[0], routeCmd[1:]...)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("%s\n", stdoutStderr)
		panic(err)
	}
	fmt.Printf("route added for %s\n", clusterip)
}

func (r Router) deleteroutes() { // clusterip, minikubeIP
	for _, clusterip := range r.routetable {
		var routeCmd []string
		if runtime.GOOS == "windows" {
			routeCmd = []string{"NOT SURE"}
		} else if runtime.GOOS == "linux" {
			routeCmd = []string{"bash", "-c",
				fmt.Sprintf("sudo ip route delete %s", clusterip)}
		} else if runtime.GOOS == "darwin" {
			routeCmd = []string{"bash", "-c",
				fmt.Sprintf("sudo route -n del %s", clusterip)}
		} else {
			panic("unsupported os detected")
		}

		cmd := exec.Command(routeCmd[0], routeCmd[1:]...)
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("%s\n", stdoutStderr)
			panic(err)
		}
		fmt.Printf("route removed for %s\n", clusterip)
	}
}

func addIngressToService(client corev1.CoreV1Interface, svc v1.Service) error {
	patch := []byte(fmt.Sprintf(`[{"op": "add", "path": "/status/loadBalancer/ingress", "value":  [ { "ip": "%s" } ] }]`, svc.Spec.ClusterIP))
	return client.RESTClient().Patch(types.JSONPatchType).Resource("services").Namespace(svc.Namespace).Name(svc.Name).SubResource("status").Body(patch).Do().Error()
}

func init() {
	RootCmd.AddCommand(tunnelCmd)
}
