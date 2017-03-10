package interfaces

import (
	"net"
	"net/http"

	log "github.com/Sirupsen/logrus"

	"github.com/ant0ine/go-json-rest/rest"
)

type interfaceStruct struct {
	Name         string `json:"link"`
	HardwareAddr string `json:"hardwareaddr"`
	MTU          int    `json:"mtu"`
}

type addressStruct struct {
	IP   string `json:"ip"`
	Mask string `json:"mask"`
}

// GetIfaces returns the list of all network interfaces
func GetIfaces(w rest.ResponseWriter, req *rest.Request) {
	dumbInterfaces, err := net.Interfaces()
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	interfaces := make([]interfaceStruct, len(dumbInterfaces))
	for index, i := range dumbInterfaces {
		interfaces[index].Name = i.Name
		interfaces[index].HardwareAddr = i.HardwareAddr.String()
		interfaces[index].MTU = i.MTU
	}
	w.WriteJson(interfaces)
}

// GetIface returns the network interface with the specified name
func GetIface(w rest.ResponseWriter, req *rest.Request) {
	iface, err := net.InterfaceByName(req.PathParam("iface"))
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dumbAddresses, err := iface.Addrs()
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	addresses := make([]addressStruct, len(dumbAddresses))
	for index, a := range dumbAddresses {
		ipnet, _ := a.(*net.IPNet)
		addresses[index].IP = ipnet.IP.String()
		addresses[index].Mask = ipnet.Mask.String()
	}
	w.WriteJson(addresses)
}
