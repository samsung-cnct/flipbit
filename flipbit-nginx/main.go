package main

import (
	"fmt"
	"net/http"
	"os"
	"github.com/samsung-cnct/flipbit/libflipbit"
	"encoding/json"
)


func main() {
	http.HandleFunc("/update", update)
	http.HandleFunc("/", homepage)
	http.ListenAndServe(":"+determinePort(), nil)
}

func determinePort() (string) {
	if os.Getenv("FLIPBIT_PORT") == "" {
		return "8080"
	}
	return os.Getenv("FLIPBIT_PORT")

}

func update(w http.ResponseWriter, r *http.Request) {
	var entries map[string]libflipbit.Entry
	err := json.NewDecoder(r.Body).Decode(&entries)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	fmt.Printf("There are %d services in the cluster\n", len(entries))
	for key, value := range entries {
		fmt.Printf("Key: %s - Service name is %s, namespace is %s, ports are: ", key, value.Name, value.Namespace)
		for i := 0; i < len(value.NodePorts); i++ {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%d", value.NodePorts[i])
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

func homepage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<html><title>Welcome to flipbit-nginx</title><body><p>Welcome to flipbit!</p></body></html>")
}