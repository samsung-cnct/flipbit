package main

import (
	"fmt"
	"time"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"k8s.io/client-go/pkg/api/v1"
	"math/rand"
	"sort"
	"github.com/samsung-cnct/flipbit/libflipbit"
	"encoding/json"
	"net/http"
	"bytes"
)





func main() {
	k8sApiConfig := getK8SAPIConfig()
	var services *v1.ServiceList
	var nodes map[string]libflipbit.Node
	var err error

	loadbalancers := make(libflipbit.LoadBalancers,0)
	loadbalancers = append(loadbalancers, libflipbit.LoadBalancer{URL:"http://localhost:8080/update", Timeout:10})

	entries := make(map[string]libflipbit.Entry)


	// creates the clientset
	k8sAPIClient, err := kubernetes.NewForConfig(&k8sApiConfig)
	if err != nil {
		panic(err.Error())
	}
	for {
		// Get services
		services, err = getCandidateServices(k8sAPIClient)
		if err != nil {
			panic(err.Error())
		}

		// Get Node list
		nodes = getNodes(k8sAPIClient)

		// Process services
		_ = processServices(services, entries, nodes)

		updateLoadBalancers(entries, loadbalancers)

		displayServices(entries)
		displayNodes(nodes)

		// Sleep and repeat
		time.Sleep(10 * time.Second)
	}
}

func getKnownFlipBitServices() {
	// TODO: Fetch state from etcd

	return
}

func processServices(services *v1.ServiceList, entries map[string]libflipbit.Entry, nodes map[string]libflipbit.Node) (map[string]libflipbit.Entry) {
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

		if lbLimit > len(nodes) {
			for key := range nodes {
				hosts = append(hosts, key)
			}
		} else {
			randomhosts := make(libflipbit.LBHosts,0)
			for key := range nodes {
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
			tempNode := nodes[hosts[j]]
			tempNode.Services++
			nodes[hosts[j]] = tempNode
		}

		entries[services.Items[i].Name + "." + services.Items[i].Namespace] = libflipbit.Entry{
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

func updateLoadBalancers(entries map[string]libflipbit.Entry, loadBalancers libflipbit.LoadBalancers) {
	var wg sync.WaitGroup
	wg.Add(len(loadBalancers))
	for _, loadBalancer := range loadBalancers {
		go func() {
			defer wg.Done()
			updateLoadBalancer(entries, loadBalancer)
		}()
	}
	wg.Wait()
}

func updateLoadBalancer(entries map[string]libflipbit.Entry, loadBalancer libflipbit.LoadBalancer) {
	dataBuffer := new(bytes.Buffer)
	json.NewEncoder(dataBuffer).Encode(entries)
	client := http.Client{ Timeout: time.Duration(time.Duration(loadBalancer.Timeout) * time.Second)}
	client.Post(loadBalancer.URL,"application/json; charset=utf-8", dataBuffer)
}

func displayServices(entries map[string]libflipbit.Entry) {
	fmt.Printf("There are %d services in the cluster\n", len(entries))
	for key, value := range entries {
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

func getK8SAPIConfig() (rest.Config) {
	var config rest.Config

	// If we have a bearer token, let's use that
	if os.Getenv("K8S_BEARER_TOKEN") != "" {
		return createTokenK8SAPIConfig()
	}

	if os.Getenv("K8S_USERNAME") != "" {
		return createUserPassK8SAPIConfig()
	}

	// Assuming it is inside of K8S instead
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	config = *inClusterConfig

	return config
}

func createTokenK8SAPIConfig() (rest.Config) {
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

func createUserPassK8SAPIConfig() (rest.Config) {
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

func getCandidateServices(clientset *kubernetes.Clientset) (*v1.ServiceList, error) {
	if os.Getenv("FLIPBIT_SERVICE_LABEL") == "" {
		return clientset.CoreV1().Services("").List(metav1.ListOptions{})
	} else {
		return clientset.CoreV1().Services("").List(metav1.ListOptions{LabelSelector: os.Getenv("FLIPBIT_SERVICE_LABEL") + "=true"})
	}

}

func getNodes(clientset *kubernetes.Clientset) (map[string]libflipbit.Node) {
	output := make(map[string]libflipbit.Node)
	var nodes *v1.NodeList
	var err error

	if os.Getenv("FLIPBIT_NODE_LABEL") == "" {
		nodes, err = clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	} else {
		nodes, err = clientset.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: os.Getenv("FLIPBIT_NODE_LABEL") + "=true"})
	}

	if err != nil {
		panic(err.Error())
	}

	for i := 0; i < len(nodes.Items); i++ {
		output[nodes.Items[i].Labels["kubernetes.io/hostname"]] = libflipbit.Node{ Address: nodes.Items[i].Labels["kubernetes.io/hostname"], Services: 0}
	}

	return output
}

func displayNodes(nodes map[string]libflipbit.Node) {
	fmt.Printf("Nodes Available \n")
	for key, value := range nodes {
		fmt.Printf("Hostname %s - Service Count: %d\n", key, value.Services)
	}
	fmt.Printf("\n")
}

