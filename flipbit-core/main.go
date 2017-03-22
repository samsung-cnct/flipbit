package main

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"k8s.io/client-go/pkg/api/v1"
)

func main() {
	config := getConfig()
	var services *v1.ServiceList
	var err error

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(&config)
	if err != nil {
		panic(err.Error())
	}
	for {
		//if os.Getenv("FLIPBIT_SERVICE_LABEL") == "" {
		//	services, err = clientset.CoreV1().Services("").List(metav1.ListOptions{})
		//} else {
		//	services, err = clientset.CoreV1().Services("").List(metav1.ListOptions{LabelSelector: os.Getenv("FLIPBIT_SERVICE_LABEL") + "=true"})
		//}
		services, err = getCandidateServices(clientset)
		if err != nil {
			panic(err.Error())
		}
		fmt.Printf("There are %d services in the cluster\n", len(services.Items))
		for i := 0; i < len(services.Items); i++ {
			fmt.Printf("Service name is %s, namespace is %s, type is %s\n", services.Items[i].Name, services.Items[i].Namespace, services.Items[i].Spec.Type)
		}
		fmt.Printf("\n")
		time.Sleep(10 * time.Second)
	}
}

func getConfig() (rest.Config) {
	var config rest.Config

	// If we have a bearer token, let's use that
	if os.Getenv("K8S_BEARER_TOKEN") != "" {
		return createTokenConfig()
	}

	if os.Getenv("K8S_USERNAME") != "" {
		return createUserPassConfig()
	}

	// Assuming it is inside of K8S instead
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	config = *inClusterConfig

	return config
}

func createTokenConfig() (rest.Config) {
	token := os.Getenv("K8S_BEARER_TOKEN")
	apiserver := os.Getenv("K8S_API_SERVER")

	if (token == "") || (apiserver == "") {
		panic("K8S_BEARER_TOKEN or K8S_API_SERVER not set")
	}

	tlsConfig := rest.TLSClientConfig{
		CAFile: "../cadata",
	}

	config := rest.Config{
		Host: apiserver,
		BearerToken: token,
		TLSClientConfig: tlsConfig,
	}

	return config
}

func createUserPassConfig() (rest.Config) {
	username := os.Getenv("K8S_USERNAME")
	password := os.Getenv("K8S_PASSWORD")
	apiserver := os.Getenv("K8S_API_SERVER")

	if (username == "") || (password == "") || (apiserver == "") {
		panic("K8S_USERNAME or K8S_PASSWORD or K8S_API_SERVER not set")
	}

	tlsConfig := rest.TLSClientConfig{
		CAFile: "./cadata",
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