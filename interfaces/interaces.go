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
    ip, err := iface.Addrs()
    log.Printf(ip[0].String())
    if err != nil {
        log.Printf(err.Error())
        rest.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    w.WriteJson(ip)
}
