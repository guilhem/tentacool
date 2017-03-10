package dns

import (
	"encoding/json"
	"net/http"

	log "github.com/Sirupsen/logrus"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/boltdb/bolt"
	"github.com/guilhem/dnsconfig"
	"github.com/guilhem/dnsconfig/resolvconf"
)

const (
	dnsBucket = "dns"
	key       = "dns"
)

var db *bolt.DB

// GetDNS returns all registered DNS
func GetDNS(w rest.ResponseWriter, req *rest.Request) {
	dns, err := dnsconfig.DnsReadConfig(useResolvPath())
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("Request DNS list : %v", dns)
	w.WriteJson(dns)
}

// PostDNS register the specified list of DNS
func PostDNS(w rest.ResponseWriter, req *rest.Request) {
	dns := dnsconfig.DnsConfig{}
	if err := req.DecodeJsonPayload(&dns); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(dnsBucket))
		data, err := json.Marshal(dns)
		if err != nil {
			return
		}
		err = b.Put([]byte(key), []byte(data))
		return
	})
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
	}
	return dnsconfig.ResolvPath
}

// DBinit initializes the DNS database at startup
func DBinit(d *bolt.DB) (err error) {
	db = d
	err = db.Update(func(tx *bolt.Tx) (err error) {
		_, err = tx.CreateBucketIfNotExists([]byte(dnsBucket))
		return
	})
	if err != nil {
		return err
	}

	log.Printf("Reinstall previous dns from DB")
	err = db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(dnsBucket))
		dns := dnsconfig.DnsConfig{}
		v := b.Get([]byte(key))
		if v != nil {
			if err := json.Unmarshal(v, &dns); err != nil {
				log.Printf(err.Error())
			}
			if err := dnsconfig.DnsWriteConfig(&dns, useResolvPath()); err != nil {
				log.Printf(err.Error())
			}
		}
		return
	})
	return
}
