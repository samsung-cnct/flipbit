package main

import (
	"crypto/sha1"
	"encoding/hex"
	"github.com/samsung-cnct/flipbit/libflipbit"
	"strconv"
)

type NginxStream struct {
	Ports libflipbit.Ports
	Service string
	IPAddress string
	Upstreams []string

	Configuration string
	Hash string
}

func (n *NginxStream) generateConfiguration() {
	n.Configuration = "#flipbit realip " + n.IPAddress + "\n"
	n.Configuration += "#flipbit service " + n.Service + "\n"
	for _, port := range n.Ports  {
		n.Configuration += "upstream " + n.Service + "_" + strconv.Itoa(int(port.NativePort)) + "_" + port.Protocol + "_origin { least_conn; "
		for _, upstream := range n.Upstreams {
			n.Configuration += "server " + upstream + ":" + strconv.Itoa(int(port.NodePort))
			if port.Protocol == "UDP" {
				n.Configuration += " udp"
			}
			n.Configuration += "; "
		}
		n.Configuration += "} server { listen " + n.IPAddress + ":" + strconv.Itoa(int(port.NativePort))
		if port.Protocol == "UDP" {
			n.Configuration += " udp"
		}
		n.Configuration += "; proxy_pass " + n.Service + "_" + strconv.Itoa(int(port.NativePort)) + "_" + port.Protocol + "_origin; } "
	}
}

func (n *NginxStream) generateHash() {
	hasher := sha1.New()
	hasher.Write([]byte(n.Configuration))
	n.Hash = hex.EncodeToString(hasher.Sum(nil))
}

type NginxStreams []NginxStream
