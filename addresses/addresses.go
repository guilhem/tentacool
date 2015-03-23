package addresses

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/boltdb/bolt"
	"github.com/docker/libcontainer/netlink"
	"github.com/docker/libcontainer/network"
)

type addressStruct struct {
	ID   string `json:"id"`
	Link string `json:"link"`
	IP   string `json:"ip"`
}

const (
	defaultIface  = "eth0"
	addressBucket = "address"
)

var db *bolt.DB

// GetAddresses returns all registered addresses
func GetAddresses(w rest.ResponseWriter, req *rest.Request) {
	addresses := []addressStruct{}
	err := db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(addressBucket))
		address := addressStruct{}
		b.ForEach(func(k, v []byte) (err error) {
			err = json.Unmarshal(v, &address)
			if err != nil {
				return
			}
			addresses = append(addresses, address)
			return
		})
		return
	})
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("GetAddresses requested : %s", addresses)
	w.WriteJson(addresses)
}

// GetAddress returns the address with the specified ID
func GetAddress(w rest.ResponseWriter, req *rest.Request) {
	id := req.PathParam("address")
	address := addressStruct{}
	err := db.View(func(tx *bolt.Tx) (err error) {
		tmp := tx.Bucket([]byte(addressBucket)).Get([]byte(id))
		if tmp == nil {
			err = fmt.Errorf("ItemNotFound: Could not find address for %s in db", id)
			return
		}
		err = json.Unmarshal(tmp, &address)
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

	log.Printf("GetAddress %s requested : %s", id, address)
	w.WriteJson(address)
}

// PostAddress register a new address
func PostAddress(w rest.ResponseWriter, req *rest.Request) {
	address := addressStruct{}
	if err := req.DecodeJsonPayload(&address); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if address.Link == "" {
		err := errors.New("Link is empty")
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if address.IP == "" {
		err := errors.New("IP is empty")
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, _, err := net.ParseCIDR(address.IP); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err := db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(addressBucket))
		if address.ID == "" {
			int, err := b.NextSequence()
			if err != nil {
				return err
			}
			address.ID = strconv.FormatUint(int, 10)
		} else {
			if _, err := strconv.ParseUint(address.ID, 10, 64); err == nil {
				return errors.New("ID is an integer")
			}
			if a := b.Get([]byte(address.ID)); a != nil {
				return errors.New("ID exists")
			}
		}
		data, err := json.Marshal(address)
		if err != nil {
			return
		}
		err = b.Put([]byte(address.ID), []byte(data))
		return
	})

	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := setIP(address); err != nil {
		w.Header().Set("X-ERROR", err.Error())
	}
	w.WriteJson(address)
}

// PutAddress modify the existing address with the specified ID
func PutAddress(w rest.ResponseWriter, req *rest.Request) {
	address := addressStruct{}
	if err := req.DecodeJsonPayload(&address); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	address.ID = req.PathParam("address")

	// Removing the old interface address using netlink
	oldAddress := addressStruct{}
	err := db.View(func(tx *bolt.Tx) (err error) {
		tmp := tx.Bucket([]byte(addressBucket)).Get([]byte(address.ID))
		if tmp != nil {
			err = json.Unmarshal(tmp, &oldAddress)
			if oldAddress != address {
				err = deleteIP(oldAddress)
			}
		}
		return
	})

	err = db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(addressBucket))
		data, err := json.Marshal(address)
		if err != nil {
			return
		}
		err = b.Put([]byte(address.ID), []byte(data))
		return
	})

	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = setIP(address); err != nil {
		w.Header().Set("X-ERROR", err.Error())
	}
	w.WriteJson(address)
}

// DeleteAddress deletes the address with the specified ID
func DeleteAddress(w rest.ResponseWriter, req *rest.Request) {
	id := req.PathParam("address")

	address := addressStruct{}
	err := db.View(func(tx *bolt.Tx) (err error) {
		tmp := tx.Bucket([]byte(addressBucket)).Get([]byte(id))
		err = json.Unmarshal(tmp, &address)
		return
	})
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = deleteIP(address); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = db.Update(func(tx *bolt.Tx) (err error) {
		err = tx.Bucket([]byte(addressBucket)).Delete([]byte(id))
		return
	})
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func setIP(a addressStruct) (err error) {
	log.Printf("Set IP:%s, to:%s", a.IP, a.Link)
	err = network.SetInterfaceIp(a.Link, a.IP)
	if err != nil {
		return
	}
	log.Printf("Adding route for this address")
	_ = netlink.AddRoute("", a.IP, "", a.Link)
	return
}

func deleteIP(a addressStruct) (err error) {
	log.Printf("Deleting IP: %s, to:%s", a.IP, a.Link)
	err = network.DeleteInterfaceIp(a.Link, a.IP)
	if err != nil {
		return err
	}
	return
}

// CommandSetIP is a command-line tool to set an IP
func CommandSetIP(id string, ip string) {
	if _, _, err := net.ParseCIDR(ip); err != nil {
		log.Printf(err.Error())
		return
	}

	if err := db.Update(func(tx *bolt.Tx) (err error) {
		_, err = tx.CreateBucketIfNotExists([]byte(addressBucket))
		return
	}); err != nil {
		log.Printf(err.Error())
		return
	}

	address := addressStruct{ID: id, Link: "eth0", IP: ip}

	oldAddress := addressStruct{}
	if err := db.View(func(tx *bolt.Tx) (err error) {
		tmp := tx.Bucket([]byte(addressBucket)).Get([]byte(address.ID))
		if tmp != nil {
			err = json.Unmarshal(tmp, &oldAddress)
			if oldAddress != address {
				err = deleteIP(oldAddress)
			}
		}
		return
	}); err != nil {
		log.Printf(err.Error())
		return
	}

	if err := db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(addressBucket))
		data, err := json.Marshal(address)
		if err != nil {
			return
		}
		err = b.Put([]byte(address.ID), []byte(data))
		return
	}); err != nil {
		log.Printf(err.Error())
		return
	}

	setIP(address)
}

// DBinit initializes the addresses database at startup
func DBinit(d *bolt.DB) (err error) {
	db = d
	err = db.Update(func(tx *bolt.Tx) (err error) {
		_, err = tx.CreateBucketIfNotExists([]byte(addressBucket))
		return
	})
	if err != nil {
		return err
	}

	err = db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(addressBucket))

		log.Printf("Reinstall previous address from DB")
		address := addressStruct{}
		b.ForEach(func(k, v []byte) (err error) {
			if err := json.Unmarshal(v, &address); err != nil {
				log.Printf(err.Error())
			} else if err := setIP(address); err != nil {
				log.Printf(err.Error())
			}
			return
		})
		return
	})
	return
}
