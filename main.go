package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"
	"time"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/boltdb/bolt"
	"github.com/docker/libcontainer/netlink"
	"github.com/docker/libcontainer/network"
	"github.com/rakyll/globalconf"
)

const (
	appName       = "tentacool"
	addressBucket = "address"
)

var (
	flagSocket = flag.String("socket", "/var/run/"+appName, "Path for unix socket")
	flagOwner  = flag.String("owner", "", "Ownership for socket")
	flagGroup  = flag.Int("group", -1, "Group for socket")
	flagDB     = flag.String("db", "/var/lib/"+appName+"/db", "Path for DB")
	// flagMode   = flag.Int("mode", 0640, "FileMode for socket")

	db *bolt.DB
	ln net.Listener
)

type Address struct {
	ID   string `json:"id"`
	Link string `json:"link"`
	IP   string `json:"ip"`
}

func main() {
	conf, err := globalconf.New(appName)
	if err != nil {
		log.Fatal(err)
	}
	conf.ParseAll()

	handler := rest.ResourceHandler{}
	err = handler.SetRoutes(
		&rest.Route{"GET", "/interfaces", GetIfaces},
		&rest.Route{"GET", "/interfaces/:iface", GetIface},
		&rest.Route{"GET", "/addresses", GetAddresses},
		&rest.Route{"POST", "/addresses", PostAddress},
		&rest.Route{"GET", "/addresses/:address", GetAddress},
		&rest.Route{"PUT", "/addresses/:address", PutAddress},
		&rest.Route{"DELETE", "/addresses/:address", DeleteAddress},
		&rest.Route{"GET", "/routes", GetRoutes},
	)
	if err != nil {
		log.Fatal(err)
	}

	ln, err = net.Listen("unix", *flagSocket)
	if nil != err {
		log.Fatal(err)
	}

	if *flagOwner != "" {
		user, err := user.Lookup(*flagOwner)
		if err != nil {
			log.Fatal(err)
		}
		uid, err := strconv.Atoi(user.Uid)
		var gid int
		if *flagGroup != -1 {
			gid = *flagGroup
		} else {
			gid, err = strconv.Atoi(user.Gid)
		}
		if err := os.Chown(*flagSocket, uid, gid); err != nil {
			log.Fatal(err)
		}
	}

	// Handle common process-killing signals so we can gracefully shut down:
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func(c chan os.Signal) {
		// Wait for a SIGINT or SIGKILL:
		sig := <-c
		log.Printf("Caught signal %s: shutting down.", sig)
		// Stop listening (and unlink the socket if unix type):
		ln.Close()
		db.Close()
		os.Exit(0)
	}(sigc)

	db, err = bolt.Open(*flagDB, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.Update(func(tx *bolt.Tx) (err error) {
		_, err = tx.CreateBucketIfNotExists([]byte(addressBucket))
		if err != nil {
			log.Fatal(err)
		}
		return
	})

	log.Printf("Reinstall previous address from DB")
	db.View(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(addressBucket))
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

	log.Fatal(http.Serve(ln, &handler))
	defer ln.Close()
}

func GetIfaces(w rest.ResponseWriter, req *rest.Request) {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteJson(ip)
}

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
	w.WriteJson(addresses)
}

func GetAddress(w rest.ResponseWriter, req *rest.Request) {
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
	w.WriteJson(address)
}

func PostAddress(w rest.ResponseWriter, req *rest.Request) {
	address := Address{}
	if err := req.DecodeJsonPayload(&address); err != nil {
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
	err := db.Update(func(tx *bolt.Tx) (err error) {
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

	if err := SetIP(address); err != nil {
		w.Header().Set("X-ERROR", err.Error())
	}
	w.WriteJson(address)
}

func DeleteAddress(w rest.ResponseWriter, req *rest.Request) {
	id := req.PathParam("address")
	err := db.Update(func(tx *bolt.Tx) (err error) {
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

func GetRoutes(w rest.ResponseWriter, req *rest.Request) {
	routes, err := netlink.NetworkGetRoutes()
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteJson(routes)
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
