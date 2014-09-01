package gateway

import (
	"log"
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/docker/libcontainer/netlink"
)

type Gateway struct {
	IP   string `json:"ip"`
	Link string `json:"link"`
}

func PostGateway(w rest.ResponseWriter, req *rest.Request) {
	gateway := Gateway{}
	if err := req.DecodeJsonPayload(&gateway); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := netlink.AddDefaultGw(gateway.IP, gateway.Link); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteJson(gateway)
}
