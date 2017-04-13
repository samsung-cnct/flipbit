package main

import (
	"github.com/samsung-cnct/flipbit/libflipbit"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"os"
	"k8s.io/client-go/kubernetes"
	"fmt"
	"time"
	"sort"
	"sync"
	"bytes"
	"net/http"
	"encoding/json"
	"math/rand"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

type FlipBit struct {
	K8SApiConfig rest.Config
	K8SAPIClient *kubernetes.Clientset
	Services *v1.ServiceList
	Nodes map[string]libflipbit.Node
	LoadBalancers libflipbit.LoadBalancers
	Entries map[string]libflipbit.Entry
}

func (fb *FlipBit) initialize() {
	fb.K8SApiConfig = fb.getK8SAPIConfig()
	var err error
	fb.K8SAPIClient, err = kubernetes.NewForConfig(&fb.K8SApiConfig)
	if err != nil {
		panic(err.Error())
	}

	fb.getLoadBalancers()
	fb.Entries = make(map[string]libflipbit.Entry)
}

func (fb *FlipBit) doStuff() {
	var services *v1.ServiceList
	var err error

	for {
		// Get services
		services, err = fb.getCandidateServices()
		if err != nil {
			panic(err.Error())
		}

		// Get Node list
		fb.getNodes()

		// Process services
		_ = fb.processServices(services)

		fb.updateLoadBalancers()

		fb.displayServices()
		fb.displayNodes()

		// Sleep and repeat
		time.Sleep(10 * time.Second)
	}
}

func (fb *FlipBit) getLoadBalancers() {
	fb.LoadBalancers = make(libflipbit.LoadBalancers,0)

	addresses := strings.Split(os.Getenv("FLIPBIT_LB_URLS"),",")
	for _, value := range addresses {
		fb.LoadBalancers = append(fb.LoadBalancers, libflipbit.LoadBalancer{URL:value, Timeout:10})
	}
}


func (fb *FlipBit) processServices(services *v1.ServiceList) (map[string]libflipbit.Entry) {
	removedList := make(map[string]libflipbit.Entry)

	for i := 0; i < len(services.Items); i++ {
		ports := make(libflipbit.Ports,0)
		for j := 0; j < len(services.Items[i].Spec.Ports); j++ {
			ports = append(ports, libflipbit.Port{
				NodePort:services.Items[i].Spec.Ports[j].NodePort,
				NativePort:services.Items[i].Spec.Ports[j].Port,
				Protocol:string(services.Items[i].Spec.Ports[j].Protocol) } )
		}

		lbLimit := 3
		hosts := make([]string,0)

		if lbLimit > len(fb.Nodes) {
			for key := range fb.Nodes {
				hosts = append(hosts, key)
			}
		} else {
			randomhosts := make(libflipbit.LBHosts,0)
			for key := range fb.Nodes {
				randomhosts = append(randomhosts, libflipbit.LBHost{ Chance: rand.Int(), Host: key})
				if len(randomhosts) > lbLimit {
					sort.Sort(randomhosts)
					randomhosts = randomhosts[1:]
				}
			}
			for _, value := range randomhosts {
				hosts = append(hosts, value.Host)
			}
		}

		for j := 0; j < len(hosts); j++ {
			tempNode := fb.Nodes[hosts[j]]
			tempNode.Services++
			fb.Nodes[hosts[j]] = tempNode
		}

		fb.Entries[services.Items[i].Name + "." + services.Items[i].Namespace] = libflipbit.Entry{
			Name: services.Items[i].Name,
			Namespace: services.Items[i].Namespace,
			Ports: ports,
			Remained: true,
			Changed: true,
			Hosts: hosts,
		}

	}

	return removedList
}

func (fb *FlipBit) updateLoadBalancers() {
	var wg sync.WaitGroup
	wg.Add(len(fb.LoadBalancers))
	for _, loadBalancer := range fb.LoadBalancers {
		go func(loadBalancer libflipbit.LoadBalancer) {
			defer wg.Done()
			fb.updateLoadBalancer(loadBalancer)
		}(loadBalancer)
	}
	wg.Wait()
}

func (fb *FlipBit) updateLoadBalancer(loadBalancer libflipbit.LoadBalancer) {
	dataBuffer := new(bytes.Buffer)
	json.NewEncoder(dataBuffer).Encode(fb.Entries)
	client := http.Client{ Timeout: time.Duration(time.Duration(loadBalancer.Timeout) * time.Second)}
	client.Post(loadBalancer.URL,"application/json; charset=utf-8", dataBuffer)
}

func (fb FlipBit) displayServices() {
	fmt.Printf("There are %d services in the cluster\n", len(fb.Entries))
	for key, value := range fb.Entries {
		fmt.Printf("Key: %s - Service name is %s, namespace is %s, ports are: ", key, value.Name, value.Namespace)
		for i := 0; i < len(value.Ports); i++ {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%d:%d Protocol: %s", value.Ports[i].NodePort, value.Ports[i].NativePort, value.Ports[i].Protocol)
		}
		fmt.Printf(" and hosts are: ")
		for i := 0; i < len(value.Hosts); i++ {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s", value.Hosts[i])
		}
		fmt.Printf("\n")
	}

	fmt.Printf("\n")

}

func (fb FlipBit) getK8SAPIConfig() (rest.Config) {
	var config rest.Config

	// If we have a bearer token, let's use that
	if os.Getenv("K8S_BEARER_TOKEN") != "" {
		return fb.createTokenK8SAPIConfig()
	}

	if os.Getenv("K8S_USERNAME") != "" {
		return fb.createUserPassK8SAPIConfig()
	}

	// Assuming it is inside of K8S instead
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	config = *inClusterConfig

	return config
}

func (fb FlipBit) createTokenK8SAPIConfig() (rest.Config) {
	token := os.Getenv("K8S_BEARER_TOKEN")
	apiserver := os.Getenv("K8S_API_SERVER")

	if (token == "") || (apiserver == "") {
		panic("K8S_BEARER_TOKEN or K8S_API_SERVER not set")
	}

	tlsConfig := rest.TLSClientConfig{
		CAData: []byte(os.Getenv("K8S_CA_DATA")),
	}

	config := rest.Config{
		Host: apiserver,
		BearerToken: token,
		TLSClientConfig: tlsConfig,
	}

	return config
}

func (fb FlipBit) createUserPassK8SAPIConfig() (rest.Config) {
	username := os.Getenv("K8S_USERNAME")
	password := os.Getenv("K8S_PASSWORD")
	apiserver := os.Getenv("K8S_API_SERVER")

	if (username == "") || (password == "") || (apiserver == "") {
		panic("K8S_USERNAME or K8S_PASSWORD or K8S_API_SERVER not set")
	}

	tlsConfig := rest.TLSClientConfig{
		CAData: []byte(os.Getenv("K8S_CA_DATA")),
	}

	config := rest.Config{
		Host: apiserver,
		Username: username,
		Password: password,
		TLSClientConfig: tlsConfig,
	}

	return config
}

func (fb FlipBit) getCandidateServices() (*v1.ServiceList, error) {
	if os.Getenv("FLIPBIT_SERVICE_LABEL") == "" {
		return fb.K8SAPIClient.CoreV1().Services("").List(metav1.ListOptions{})
	} else {
		return fb.K8SAPIClient.CoreV1().Services("").List(metav1.ListOptions{LabelSelector: os.Getenv("FLIPBIT_SERVICE_LABEL") + "=true"})
	}


}

func (fb *FlipBit) getNodes() {
	fb.Nodes = make(map[string]libflipbit.Node)
	var nodes *v1.NodeList
	var err error

	if os.Getenv("FLIPBIT_NODE_LABEL") == "" {
		nodes, err = fb.K8SAPIClient.CoreV1().Nodes().List(metav1.ListOptions{})
	} else {
		nodes, err = fb.K8SAPIClient.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: os.Getenv("FLIPBIT_NODE_LABEL") + "=true"})
	}

	if err != nil {
		panic(err.Error())
	}

	for i := 0; i < len(nodes.Items); i++ {
		fb.Nodes[nodes.Items[i].Labels["kubernetes.io/hostname"]] = libflipbit.Node{ Address: nodes.Items[i].Labels["kubernetes.io/hostname"], Services: 0}
	}
}

func (fb FlipBit) displayNodes() {
	fmt.Printf("Nodes Available \n")
	for key, value := range fb.Nodes {
		fmt.Printf("Hostname %s - Service Count: %d\n", key, value.Services)
	}
	fmt.Printf("\n")
}