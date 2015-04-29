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

type dhcpStruct struct {
	Active    bool   `json:"active"`
	Interface string `json:"interface"`
}

const (
	defaultIface = "eth0"
	dhcpBucket   = "dhcp"
	activeKey    = "active"
)

var db *bolt.DB

// GetDhcp returns the current status of the DHCP client
func GetDhcp(w rest.ResponseWriter, req *rest.Request) {
	dhcp := dhcpStruct{}
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
	}
	log.Printf("GetDhcp requested: %s", dhcp)
	w.WriteJson(dhcp)
}

// PostDhcp set or unset the DHCP client via RESTful request
func PostDhcp(w rest.ResponseWriter, req *rest.Request) {
	// Parameters
	dhcp := dhcpStruct{}
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

// SetDhcp set or unset the DHCP client (using system)
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

// DBinit initializes the DHCP database at startup
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
		dhcp := dhcpStruct{}
		tmp := b.Get([]byte(activeKey))
		if tmp != nil {
			if err := json.Unmarshal(tmp, &dhcp); err != nil {
				log.Printf(err.Error())
			} else if err := SetDhcp(dhcp.Active, dhcp.Interface); err != nil {
				log.Printf(err.Error())
			}
		}
		return
	})
	return
}
