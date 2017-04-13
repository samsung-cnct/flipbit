package main

import (
	"net/http"
	"fmt"
	"os"
	"io"
	"strings"
	"crypto/sha1"
	"encoding/hex"
	"github.com/samsung-cnct/flipbit/libflipbit"
	"encoding/json"
	"io/ioutil"
	"log"
	"sync"
)

type Update struct {
	AvailableIPs map[string]Service
	Entries map[string]libflipbit.Entry
	Services map[string]string

	UpdateResult []libflipbit.LBUpdateResult
}

func (u Update) Update(w http.ResponseWriter, r *http.Request) {

	// The flow of the update is as follows
	// Read in current filesystem (readCurrentState)
	//   - This may remove some files that can no longer be supported because an IP Address is out of range
	//     or the service overlaps with another file, etc.
	// Parse the updated Configuration (getDesiredState)
	// Ensure current services are up to date (verifyCurrentServices)
	//   - There may be a change in the service configuration
	//   - This will remove entries from both `Entries` as well as `Services`
	// Remove any leftover services (removeLeftoverServices)
	//   - They shouldn't exist apparently, this will free up AvailableIPs
	// Assign new services out (assignNewServices)

	u.readCurrentState()
	err := u.getDesiredState(w, r)
	if err != nil {
		return
	}

	u.verifyCurrentServices()

	u.removeLeftoverServices()

	u.assignNewServices()

	fmt.Printf("\nUpdate Results:\n")
	for _, value := range u.UpdateResult {
		fmt.Printf("Service: %s, IP Address: %s, Result: %s\n", value.Service, value.IPAddress, value.Status)
	}
	fmt.Printf("End of Update Results:\n\n")

	u.writeResults(w,r)

}

func (u *Update) readCurrentState() {
	u.Services = make(map[string]string)
	u.AvailableIPs = u.getRealIPs()

	files := u.getFiles()
	for _, value := range files {
		var file, _ = os.OpenFile(u.getFullFilePath(value), os.O_RDWR, 0644)
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

		_, isIPAvailable := u.AvailableIPs[ipAddress]

		if !isIPAvailable {
			fmt.Printf("Service -->%s<-- is hosted on IP -->%s<-- but this is no longer a valid ip address!", service, ipAddress )
		} else if u.AvailableIPs[ipAddress].Service != "open" {
			fmt.Printf("Service -->%s<-- is hosted on IP -->%s<-- but this is also allocated to %s", service, ipAddress, u.AvailableIPs[ipAddress].Service )
			os.Remove(os.Getenv("FLIPBIT_STREAM_DIRECTORY")+"/"+value)
		} else {
			hasher := sha1.New()
			hasher.Write(text)
			u.AvailableIPs[ipAddress] = Service{ Hash: hex.EncodeToString(hasher.Sum(nil)), Service: service, Filename: value}
			u.Services[service] = ipAddress
		}
	}
}

func (u *Update) getRealIPs() (map[string]Service) {
	addresses := strings.Split(os.Getenv("FLIPBIT_LB_ADDRESSES"),",")
	output := make(map[string]Service)
	for _, value := range addresses {
		output[value] = Service{Service: "open"}
	}
	return output
}

func (u *Update) getFiles() (map [string]string) {
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

func (u *Update) getDesiredState(w http.ResponseWriter, r *http.Request) error {
	err := json.NewDecoder(r.Body).Decode(&u.Entries)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return err
	}
	return nil
}

func (u *Update) verifyCurrentServices() {
	var wg sync.WaitGroup
	serviceKeys := make([]string, 0)
	ipAddresses := make([]string, 0)

	// Ranging over the entries
	for serviceKey, entry := range u.Entries {
		wg.Add(1)
		go func(serviceKey string, entry libflipbit.Entry) {
			ipAddress, reason, _ := u.matchIdenticalService(entry, serviceKey)
			if reason == "Changed" || reason == "NoChange" {
				u.UpdateResult = append(u.UpdateResult, libflipbit.LBUpdateResult{IPAddress: ipAddress, Service: serviceKey, Status: reason})
				serviceKeys = append(serviceKeys, serviceKey)
				ipAddresses = append(ipAddresses, ipAddress)
			}
			wg.Done()
		}(serviceKey, entry)
	}

	wg.Wait()

	for _, serviceKey := range serviceKeys {
		delete(u.Entries, serviceKey)
		delete(u.Services, serviceKey)
	}
	for _, ipAddress := range ipAddresses {
		delete(u.AvailableIPs, ipAddress)
	}
}

func (u *Update) matchIdenticalService(entry libflipbit.Entry, serviceKey string) (string, string, bool) {
	ipAddress, verifyKey := u.Services[serviceKey]
	if verifyKey {
		// match was found, so let's see if the configuration is unchanged
		host := NginxStream{Service: serviceKey, IPAddress: ipAddress, Ports: entry.Ports, Upstreams: entry.Hosts}
		host.generateConfiguration()
		host.generateHash()
		if u.AvailableIPs[ipAddress].Hash != host.Hash {
			fmt.Println("Service " + serviceKey + ", filename: " + u.AvailableIPs[ipAddress].Filename + " has changed, regenerating file")
			u.writeStream(u.AvailableIPs[ipAddress].Filename, host.Configuration)
			return ipAddress, "Changed", true
		}
		return ipAddress, "NoChange", true
	}
	return "", "NoMatch", true
}

func (u *Update) removeLeftoverServices() {
	for serviceName, ipAddress := range u.Services {
		fmt.Println("Service " + serviceName + ", filename: " + u.AvailableIPs[ipAddress].Filename + " is no longer needed, freeing")
		os.Remove(u.getFullFilePath(u.AvailableIPs[ipAddress].Filename))
		u.AvailableIPs[ipAddress] = Service{Service: "open"}
		delete(u.Services, serviceName)
	}
}

func (u *Update) assignNewServices() {

	// Let's ensure there is sanity and make an easily-accessed list of IPs
	ipAddressArray := make([]string,0)
	for key, value := range u.AvailableIPs {
		if value.Service == "open" {
			ipAddressArray = append(ipAddressArray, key)
		}
	}

	ipAddressIndex := 0
	var wg sync.WaitGroup

	for serviceKey, entry := range u.Entries {
		if ipAddressIndex >= len(ipAddressArray) {
			fmt.Println("Too many services, not enough IPs.  Ditching service -->" + serviceKey + "<--")
			u.UpdateResult = append(u.UpdateResult, libflipbit.LBUpdateResult{IPAddress: "none", Service: serviceKey, Status: "Cannot Assign IP"})
			continue
		}
		ipAddress := ipAddressArray[ipAddressIndex]
		ipAddressIndex++

		wg.Add(1)
		go func(serviceKey string, ipAddress string, ports libflipbit.Ports, upstreams []string) {
			defer wg.Done()
			u.createNewService(serviceKey, ipAddress, ports, upstreams)
			u.UpdateResult = append(u.UpdateResult, libflipbit.LBUpdateResult{IPAddress: ipAddress, Service: serviceKey, Status: "Created"})
		}(serviceKey, ipAddress, entry.Ports, entry.Hosts)
	}
	wg.Wait()
}

func (u *Update) createNewService(serviceKey string, ipAddress string, ports libflipbit.Ports, upstreams []string) bool {
	host := NginxStream{ Service: serviceKey, IPAddress: ipAddress, Ports: ports, Upstreams: upstreams}
	host.generateConfiguration()
	host.generateHash()

	u.writeStream(serviceKey + ".conf", host.Configuration)
	fmt.Println("Service " + serviceKey + ", filename: " + serviceKey + ".conf was created!")

	return true
}

func (u Update) writeStream(filename string, configuration string) {
	// Changes need to be made - let's start...
	fmt.Printf("Writing out -->%s<-- to -->%s<--\n", configuration, filename)
	err := ioutil.WriteFile(u.getFullFilePath(filename), []byte(configuration), 0644)
	u.fileError(err)
}

func (u Update) writeResults(w http.ResponseWriter, r *http.Request) {
	js, err := json.Marshal(u.UpdateResult)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func (u Update) getFullFilePath(filename string) string {
	return os.Getenv("FLIPBIT_STREAM_DIRECTORY")+"/"+ filename
}

func (u Update) fileError(err error) {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(0)
	}
}
