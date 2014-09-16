package interfaces

import (
    "log"
    "net"
    "net/http"

    "github.com/ant0ine/go-json-rest/rest"
)

type Interface struct {
    Name string `json:"link"`
    HardwareAddr string `json:"hardwareaddr"`
    MTU int `json:"mtu"`
}

type Address struct {
    IP string `json:"ip"`
    Mask string `json:"mask"`
}

func GetIfaces(w rest.ResponseWriter, req *rest.Request) {
    dumb_interfaces, err := net.Interfaces()
    if err != nil {
        log.Printf(err.Error())
        rest.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    interfaces := make([]Interface, len(dumb_interfaces))
    for index, i := range dumb_interfaces {
        interfaces[index].Name = i.Name
        interfaces[index].HardwareAddr = i.HardwareAddr.String()
        interfaces[index].MTU = i.MTU
    }
    w.WriteJson(interfaces)
}

func GetIface(w rest.ResponseWriter, req *rest.Request) {
    iface, err := net.InterfaceByName(req.PathParam("iface"))
    if err != nil {
        log.Printf(err.Error())
        rest.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    dumb_addresses, err := iface.Addrs()
    if err != nil {
        log.Printf(err.Error())
        rest.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    addresses := make([]Address, len(dumb_addresses))
    for index, a := range dumb_addresses {
        ipnet, _ := a.(*net.IPNet)
        addresses[index].IP = ipnet.IP.String()
        addresses[index].Mask = ipnet.Mask.String()
    }
    w.WriteJson(addresses)
}
