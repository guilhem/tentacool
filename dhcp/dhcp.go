package dhcp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/boltdb/bolt"
)

type DHCP struct {
	Active    bool   `json:"active"`
	Interface string `json:"interface"`
}

const (
	defaultIface = "eth0"
    dhcpBucket = "dhcp"
    activeKey = "active"
)

var db *bolt.DB


func GetDhcp(w rest.ResponseWriter, req *rest.Request) {
	dhcp := DHCP{}
	err := db.View(func(tx *bolt.Tx) (err error) {
		tmp := tx.Bucket([]byte(dhcpBucket)).Get([]byte(activeKey))
		if tmp != nil {
			err = json.Unmarshal(tmp, &dhcp)
			return
		}
		dhcp.Active = false
		dhcp.Interface = defaultIface
		return
	})
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		log.Printf("GetDhcp requested: %s", dhcp)
		w.WriteJson(dhcp)
	}
}

func PostDhcp(w rest.ResponseWriter, req *rest.Request) {
	// Parameters
	dhcp := DHCP{}
	if err := req.DecodeJsonPayload(&dhcp); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if dhcp.Interface == "" {
		dhcp.Interface = defaultIface
		return
	}

	// Activate/deactivate dhcp client
	if err := SetDhcp(dhcp.Active, dhcp.Interface); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update DB
	err := db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(dhcpBucket))
		data, err := json.Marshal(dhcp)
		if err != nil {
			return
		}
		err = b.Put([]byte(activeKey), []byte(data))
		return
	})
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.WriteJson(dhcp)
}

func SetDhcp(active bool, iface string) (err error) {
	if active {
		log.Printf("Starting DHCP client")
		err = exec.Command("sh", "-c", fmt.Sprintf("/sbin/dhclient %s &", iface)).Run()
	} else {
		log.Printf("Stopping DHCP client")
		err = exec.Command("sh", "-c", "/sbin/dhclient -x").Run()
	}
	if err != nil {
		log.Printf(err.Error())
		return err
	}
	return nil
}

func DBinit(d *bolt.DB) (err error) {
	db = d
	err = db.Update(func(tx *bolt.Tx) (err error) {
		_, err = tx.CreateBucketIfNotExists([]byte(dhcpBucket))
		return
	})
	if err != nil {
		return err
	}

	err = db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(dhcpBucket))

		log.Printf("Restore DHCP from DB")
		dhcp := DHCP{}
		tmp := b.Get([]byte(activeKey))
		if tmp != nil {
			if err := json.Unmarshal(tmp, &dhcp); err != nil {
				log.Printf(err.Error())
			} else
			if err := SetDhcp(dhcp.Active, dhcp.Interface); err != nil {
				log.Printf(err.Error())
			}
		}
		return
	})
	return
}
