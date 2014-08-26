package main

import (
	"encoding/json"
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
	ID   string
	Link string
	IP   string
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
		&rest.Route{"POST", "/address", PostAddress},
		&rest.Route{"Get", "/address/:address", GetAddress},
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

func GetAddress(w rest.ResponseWriter, req *rest.Request) {
	id := req.PathParam("address")

	address := Address{}
	db.View(func(tx *bolt.Tx) (err error) {
		tmp := tx.Bucket([]byte(addressBucket)).Get([]byte(id))
		if err = json.Unmarshal(tmp, &address); err != nil {
			log.Printf(err.Error())
			rest.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	})
	w.WriteJson(address)
}

func PostAddress(w rest.ResponseWriter, req *rest.Request) {
	address := Address{}
	if err := req.DecodeJsonPayload(&address); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ip, ipnet, err := net.ParseCIDR(address.IP)
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	iface, err := net.InterfaceByName(address.Link)
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := netlink.NetworkLinkAddIp(iface, ip, ipnet); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = netlink.AddRoute("", address.IP, "", address.Link)

	db.Update(func(tx *bolt.Tx) (err error) {
		b := tx.Bucket([]byte(addressBucket))
		if address.ID == "" {
			int, err := b.NextSequence()
			if err != nil {
				log.Printf(err.Error())
				rest.Error(w, err.Error(), http.StatusInternalServerError)
				return err
			}
			address.ID = strconv.FormatUint(int, 10)
		}
		data, err := json.Marshal(address)
		if err != nil {
			log.Printf(err.Error())
			rest.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = b.Put([]byte(address.ID), []byte(data))
		return
	})
	w.WriteJson(address)
}
