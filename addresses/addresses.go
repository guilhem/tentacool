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
	"os/exec"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/boltdb/bolt"
	"github.com/docker/libcontainer/netlink"
	"github.com/docker/libcontainer/network"
)

type Address struct {
	ID   string `json:"id"`
	Link string `json:"link"`
	IP   string `json:"ip"`
}

type DHCP struct {
	Active bool `json:"active"`
}

const (
	defaultIface = "eth0"
	addressBucket = "address"
	dhcpKey = "dhcp"
)

var db *bolt.DB

func GetAddresses(w rest.ResponseWriter, req *rest.Request) {
	addresses := []Address{}
	err := db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(addressBucket))
		address := Address{}
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

func GetAddress(w rest.ResponseWriter, req *rest.Request) {
	id := req.PathParam("address")
	address := Address{}
	err := db.View(func(tx *bolt.Tx) (err error) {
		tmp := tx.Bucket([]byte(addressBucket)).Get([]byte(id))
		if tmp == nil {
			err = errors.New(fmt.Sprintf("ItemNotFound: Could not find address for %s in db", id))
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
	} else {
		log.Printf("GetAddress %s requested : %s", id, address)
		w.WriteJson(address)
	}
}

func PostAddress(w rest.ResponseWriter, req *rest.Request) {
	address := Address{}
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

	if err := SetIP(address); err != nil {
		w.Header().Set("X-ERROR", err.Error())
	}
	w.WriteJson(address)
}

func PutAddress(w rest.ResponseWriter, req *rest.Request) {
	address := Address{}
	if err := req.DecodeJsonPayload(&address); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	address.ID = req.PathParam("address")

	// Removing the old interface address using netlink
	oldAddress := Address{}
	err := db.View(func(tx *bolt.Tx) (err error) {
		tmp := tx.Bucket([]byte(addressBucket)).Get([]byte(address.ID))
		if tmp != nil {
			err = json.Unmarshal(tmp, &oldAddress)
			if oldAddress != address {
				err = DeleteIp(oldAddress)
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

	if err = SetIP(address); err != nil {
		w.Header().Set("X-ERROR", err.Error())
	}
	w.WriteJson(address)
}

func DeleteAddress(w rest.ResponseWriter, req *rest.Request) {
	id := req.PathParam("address")

	address := Address{}
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

	if err = DeleteIp(address); err != nil {
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

func GetDhcp(w rest.ResponseWriter, req *rest.Request) {
	err := db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(addressBucket))
		dhcp, _ := strconv.ParseBool(string(b.Get([]byte(dhcpKey))))

		w.WriteJson(map[string]bool{"active": dhcp})
		return
	})
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func PostDhcp(w rest.ResponseWriter, req *rest.Request) {
	// Get parameters
	dhcp := DHCP{}
	if err := req.DecodeJsonPayload(&dhcp); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Activate/deactivate dhcp client
	if err := SetDhcp(dhcp.Active, defaultIface); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update DB
	err := db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(addressBucket))
		v := strconv.FormatBool(dhcp.Active)
		err = b.Put([]byte(dhcpKey), []byte(v))
		return
	})
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]bool{"active": dhcp.Active})
}

func SetIP(a Address) (err error) {
	log.Printf("Set IP:%s, to:%s", a.IP, a.Link)
	err = network.SetInterfaceIp(a.Link, a.IP)
	if err != nil {
		return
	}
	log.Printf("Adding route for this address")
	_ = netlink.AddRoute("", a.IP, "", a.Link)
	return
}

func DeleteIp(a Address) (err error) {
	log.Printf("Deleting IP: %s, to:%s", a.IP, a.Link)
	err = network.DeleteInterfaceIp(a.Link, a.IP)
	if err != nil {
		return err
	}
	return
}

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

	address := Address{ID: id, Link: "eth0", IP: ip}

	oldAddress := Address{}
	if err := db.View(func(tx *bolt.Tx) (err error) {
		tmp := tx.Bucket([]byte(addressBucket)).Get([]byte(address.ID))
		if tmp != nil {
			err = json.Unmarshal(tmp, &oldAddress)
			if oldAddress != address {
				err = DeleteIp(oldAddress)
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

	SetIP(address)
}

func SetDhcp(active bool, iface string) (err error) {
	if active {
		log.Printf("Starting DHCP client")
		err = exec.Command("sh", "-c", fmt.Sprintf("/sbin/dhclient %s", iface)).Run()
	} else {
		log.Printf("Stoping DHCP client")
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
		_, err = tx.CreateBucketIfNotExists([]byte(addressBucket))
		return
	})
	if err != nil {
		return err
	}

	err = db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(addressBucket))

		dhcp, _ := strconv.ParseBool(string(b.Get([]byte(dhcpKey))))
		if dhcp {
			if err = SetDhcp(true, defaultIface); err != nil {
				log.Printf(err.Error())
			}
		}

		log.Printf("Reinstall previous address from DB")
		address := Address{}
		b.ForEach(func(k, v []byte) (err error) {
			if err := json.Unmarshal(v, &address); err != nil {
				log.Printf(err.Error())
			}
			if err := SetIP(address); err != nil {
				log.Printf(err.Error())
			}
			return
		})
		return
	})
	return
}
