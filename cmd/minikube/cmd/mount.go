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
	"net"
	"os"
	"path/filepath"
	"sync"

	"strings"

	"strconv"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	cmdUtil "k8s.io/minikube/cmd/util"
	"k8s.io/minikube/pkg/minikube/cluster"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/machine"
	"k8s.io/minikube/third_party/go9p/ufs"
)

// mountCmd represents the mount command
var mountCmd = &cobra.Command{
	Use:   "mount [flags] MOUNT_DIRECTORY(ex:\"/home\")",
	Short: "Mounts the specified directory into minikube.",
	Long:  `Mounts the specified directory into minikube.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			errText := `Please specify the directory to be mounted: 
\tminikube mount HOST_MOUNT_DIRECTORY:VM_MOUNT_DIRECTORY(ex:"/host-home:/vm-home")
`
			fmt.Fprintln(os.Stderr, errText)
			os.Exit(1)
		}
		if args[0] == "kill" {
			mountProc, err := cmdUtil.ReadProcessFromFile(filepath.Join(constants.GetMinipath(), constants.MountProcessFileName))
			if err != nil {
				glog.Errorf("Error reading mount process from file: ", err)
			}
			mountProc.Kill()
			os.Exit(0)
		}
		mountString := args[0]
		idx := strings.LastIndex(mountString, ":")
		if idx == -1 { // no ":" was present
			errText := `Mount directory must be in the form: 
			\tHOST_MOUNT_DIRECTORY:VM_MOUNT_DIRECTORY`
			fmt.Fprintln(os.Stderr, errText)
			os.Exit(1)
		}
		hostPath := mountString[:idx]
		vmPath := mountString[idx+1:]
		if _, err := os.Stat(hostPath); err != nil {
			if os.IsNotExist(err) {
				errText := fmt.Sprintf("Cannot find directory %s for mount", hostPath)
				fmt.Fprintln(os.Stderr, errText)
			} else {
				errText := fmt.Sprintf("Error accesssing directory %s for mount", hostPath)
				fmt.Fprintln(os.Stderr, errText)
			}
			os.Exit(1)
		}
		if len(vmPath) == 0 || !strings.HasPrefix(vmPath, "/") {
			errText := fmt.Sprintf("The :VM_MOUNT_DIRECTORY must be an absolute path")
			fmt.Fprintln(os.Stderr, errText)
			os.Exit(1)
		}
		var debugVal int
		if glog.V(1) {
			debugVal = 1 // ufs.StartServer takes int debug param
		}
		api, err := machine.NewAPIClient(clientType)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting client: %s\n", err)
			os.Exit(1)
		}
		defer api.Close()
		host, err := api.Load(config.GetMachineName())
		if err != nil {
			glog.Errorln("Error loading api: ", err)
			os.Exit(1)
		}
		ip, err := cluster.GetVMHostIP(host)
		if err != nil {
			glog.Errorln("Error getting the host IP address to use from within the VM: ", err)
			os.Exit(1)
		}

		fmt.Printf("Mounting %s into %s on the minikubeVM\n", hostPath, vmPath)
		fmt.Println("This daemon process needs to stay alive for the mount to still be accessible...")
		port, err := cmdUtil.GetPort()
		if err != nil {
			glog.Errorln("Error finding port for mount: ", err)
			os.Exit(1)
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			ufs.StartServer(net.JoinHostPort(ip.String(), strconv.Itoa(port)), debugVal, hostPath)
			wg.Done()
		}()
		err = cluster.MountHost(api, vmPath, strconv.Itoa(port))
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		wg.Wait()
	},
}

func init() {
	RootCmd.AddCommand(mountCmd)
}
