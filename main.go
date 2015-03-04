package main

import (
	"flag"
	"log"
	"log/syslog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/boltdb/bolt"
	"github.com/docker/libcontainer/netlink"
	"github.com/rakyll/globalconf"

	"github.com/optiflows/tentacool/addresses"
	"github.com/optiflows/tentacool/dns"
	"github.com/optiflows/tentacool/gateway"
	"github.com/optiflows/tentacool/interfaces"
)

const (
	appName       = "tentacool"
	addressBucket = "address"
)

var (
	flagSetIP   = flag.String("setip", "", "CLI to set an IP without launching the Tentacool server ('ID:CIDR')")
	flagBind    = flag.String("bind", "/var/run/"+appName, "Adress to bind. Format Path or IP:PORT")
	flagOwner   = flag.String("owner", "tentacool", "Ownership for socket")
	flagGroup   = flag.Int("group", -1, "Group for socket")
	flagDB      = flag.String("db", "/var/lib/"+appName+"/db", "Path for DB")
	flagConsole = flag.Bool("console", false, "Log in console for debug purposes")
	// flagMode   = flag.Int("mode", 0640, "FileMode for socket")

	db *bolt.DB
	ln net.Listener
)

func main() {

	conf, err := globalconf.New(appName)
	if err != nil {
		log.Fatal(err)
	}
	conf.ParseAll()

	if *flagConsole == false {
		logwriter, err := syslog.New(syslog.LOG_NOTICE, appName)
		if err == nil {
			log.SetOutput(logwriter)
		}
	}

	db, err := bolt.Open(*flagDB, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if *flagSetIP != "" {
		if err := addresses.DBinit(db); err != nil {
			log.Fatal(err)
		}
		splited := strings.Split(*flagSetIP, ":")
		if len(splited) < 2 {
			log.Fatal("ID:CIDR required")
		}
		id, ip := splited[0], splited[1]
		addresses.CommandSetIP(id, ip)
		os.Exit(0)
	}

	api := rest.NewApi()

	router, err := rest.MakeRouter(
		&rest.Route{"GET", "/interfaces", interfaces.GetIfaces},
		&rest.Route{"GET", "/interfaces/:iface", interfaces.GetIface},

		&rest.Route{"GET", "/addresses", addresses.GetAddresses},
		&rest.Route{"POST", "/addresses", addresses.PostAddress},
		&rest.Route{"GET", "/addresses/:address", addresses.GetAddress},
		&rest.Route{"PUT", "/addresses/:address", addresses.PutAddress},
		&rest.Route{"DELETE", "/addresses/:address", addresses.DeleteAddress},

		&rest.Route{"GET", "/dns", dns.GetDNS},
		&rest.Route{"POST", "/dns", dns.PostDNS},

		&rest.Route{"GET", "/routes", GetRoutes},
		&rest.Route{"POST", "/routes/gateway", gateway.PostGateway},
		&rest.Route{"GET", "/routes/gateway", gateway.GetGateway},
	)
	if err != nil {
		log.Fatal(err)
	}

	api.SetApp(router)

	var network string
	if _, err = net.ResolveTCPAddr("tcp", *flagBind); err == nil {
		network = "tcp"
	} else {
		network = "unix"
	}
	ln, err = net.Listen(network, *flagBind)
	if nil != err {
		log.Fatal(err)
	}
	defer ln.Close()

	if *flagOwner != "" && network == "unix" {
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
		if err := os.Chown(*flagBind, uid, gid); err != nil {
			log.Fatal(err)
		}
		if err := os.Chmod(*flagBind, 0660); err != nil {
			log.Fatal(err)
		}
	}

	if err := addresses.DBinit(db); err != nil {
		log.Fatal(err)
	}
	if err := dns.DBinit(db); err != nil {
		log.Fatal(err)
	}
	if err := gateway.DBinit(db); err != nil {
		log.Fatal(err)
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

	log.Printf("Now listening to bind %s", *flagBind)
	log.Fatal(http.Serve(ln, api.MakeHandler()))
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
