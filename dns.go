package main

import (
	"log"
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/guilhem/dnsconfig"
	"github.com/guilhem/dnsconfig/resolvconf"
)

func GetDNS(w rest.ResponseWriter, req *rest.Request) {
	dns, err := dnsconfig.DnsReadConfig(useResolvPath())
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteJson(dns)
}

func PostDNS(w rest.ResponseWriter, req *rest.Request) {
	dns := dnsconfig.DnsConfig{}
	if err := req.DecodeJsonPayload(&dns); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := dnsconfig.DnsWriteConfig(&dns, useResolvPath()); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteJson(&dns)
}

func useResolvPath() string {
	if resolvconf.IsResolvconf() {
		log.Printf("use Resolvconf")
		return resolvconf.ResolvPath
	} else {
		return dnsconfig.ResolvPath
	}
}
