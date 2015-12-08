package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/boltdb/bolt"
)

const (
	routesBucket = "routes"
	defaultKey   = "default"
)

var db *bolt.DB

type gatewayStruct struct {
	IP   string `json:"ip"`
	Link string `json:"link"`
}

// PostGateway apply the given gateway to the network
func PostGateway(w rest.ResponseWriter, req *rest.Request) {
	gateway := gatewayStruct{}
	if err := req.DecodeJsonPayload(&gateway); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(routesBucket))
		data, err := json.Marshal(gateway)
		if err != nil {
			return
		}
		log.Printf("Updating key %s with value %s", defaultKey, data)
		err = b.Put([]byte(defaultKey), []byte(data))
		return
	})

	if err := addDefaultGw(gateway.IP, gateway.Link); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteJson(gateway)
}

// GetGateway returns the list of all gateways
func GetGateway(w rest.ResponseWriter, req *rest.Request) {
	gateway := gatewayStruct{}
	err := db.View(func(tx *bolt.Tx) (err error) {
		tmp := tx.Bucket([]byte(routesBucket)).Get([]byte(defaultKey))
		if tmp == nil {
			err = fmt.Errorf("ItemNotFound: Could not find gateway")
			return
		}
		err = json.Unmarshal(tmp, &gateway)
		return
	})
	if err != nil {
		log.Printf(err.Error())
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "ItemNotFound") {
			code = http.StatusNotFound
		}
		rest.Error(w, err.Error(), code)
		return
	}
	log.Printf("Requested Gateways list : %s", gateway)
	w.WriteJson(gateway)
}

// DBinit initializes the gateway database at startup
func DBinit(d *bolt.DB) (err error) {
	db = d
	err = db.Update(func(tx *bolt.Tx) (err error) {
		_, err = tx.CreateBucketIfNotExists([]byte(routesBucket))
		return
	})
	if err != nil {
		return err
	}

	log.Printf("Reinstall previous gateway from DB")
	err = db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(routesBucket))
		gateway := gatewayStruct{}
		v := b.Get([]byte(defaultKey))
		if v != nil {
			if err := json.Unmarshal(v, &gateway); err != nil {
				log.Printf(err.Error())
			}
			if err := addDefaultGw(gateway.IP, gateway.Link); err != nil {
				log.Printf(err.Error())
			}
		}
		return
	})
	return
}

func addDefaultGw(ip string, linkName string) (err error) {
	err = exec.Command("sh", "-c", fmt.Sprintf("/sbin/route add default gw %s %s", ip, linkName)).Run()
	if err != nil && !strings.Contains(err.Error(), "exit status 7") {
		log.Printf(err.Error())
		return err
	}
	return nil
}
