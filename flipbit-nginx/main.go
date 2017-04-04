package main

import (
	"fmt"
	"net/http"
	"os"
	"github.com/samsung-cnct/flipbit/libflipbit"
	"encoding/json"
	"io/ioutil"
	"log"
	"io"
	"strings"
	"crypto/sha1"
	"encoding/hex"
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
	availableIPs := getRealIPs()

	err := json.NewDecoder(r.Body).Decode(&entries)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	files := getFiles()
	for _, value := range files {
		fmt.Println("File found: " + value)
		var file, _ = os.OpenFile((os.Getenv("FLIPBIT_STREAM_DIRECTORY")+"/"+value), os.O_RDWR, 0644)
		defer file.Close()

		// read file
		var text = make([]byte, 1024)
		for {
			n, err := file.Read(text)
			if err != io.EOF {
			}
			if n == 0 {
				break
			}
		}

		ipAddress := strings.Split(strings.Split(string(text), "#flipbit realip ")[1], "\n")[0]
		service := strings.Split(strings.Split(string(text), "#flipbit service ")[1], "\n")[0]

		_, isIPAvailable := availableIPs[ipAddress]

		if isIPAvailable {
			fmt.Errorf("Service -->%s<-- is hosted on IP -->%s<-- but this is no longer a valid ip address!", service, ipAddress )
		} else if availableIPs[ipAddress].Service != "open" {
			fmt.Errorf("Service -->%s<-- is hosted on IP -->%s<-- but this is also allocated to %s", service, ipAddress, availableIPs[ipAddress].Service )
			os.Remove(os.Getenv("FLIPBIT_STREAM_DIRECTORY")+"/"+value)
		} else {
			hasher := sha1.New()
			hasher.Write(text)
			availableIPs[ipAddress] = Service{ Hash: hex.EncodeToString(hasher.Sum(nil)), Service: service, Filename: value}
		}
	}

	ipPointer := 0

	fmt.Printf("There are %d services in the cluster\n", len(entries))
	for key, value := range entries {
		fileOutput := "#flipbit realip "
		fmt.Printf("Key: %s - Service name is %s, namespace is %s, ports are: ", key, value.Name, value.Namespace)
		for i := 0; i < len(value.Ports); i++ {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%d", value.Ports[i].NativePort)
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

func getRealIPs() (map[string]Service) {
	addresses := strings.Split(os.Getenv("FLIPBIT_LB_ADDRESSES"),",")
	output := make(map[string]Service)
	for _, value := range addresses {
		output[value] = Service{Service: "open"}
	}
	return output
}

func getFiles() (map [string]string) {
	output := make(map[string]string)
	files, err := ioutil.ReadDir(os.Getenv("FLIPBIT_STREAM_DIRECTORY"))
	if err != nil {
		log.Fatal(err)
	}
	for _, value := range files {
		if !value.IsDir() {
			output[value.Name()] = value.Name()
		}
	}

	return output
}

func homepage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<html><title>Welcome to flipbit-nginx</title><body><p>Welcome to flipbit!</p></body></html>")
}