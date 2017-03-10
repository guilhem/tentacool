package web

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/boltdb/bolt"
	"github.com/rakyll/globalconf"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vishvananda/netlink"

	"github.com/guilhem/tentacool/addresses"
	"github.com/guilhem/tentacool/dhcp"
	"github.com/guilhem/tentacool/dns"
	"github.com/guilhem/tentacool/gateway"
	"github.com/guilhem/tentacool/interfaces"
)

const (
	appName       = "tentacool"
	addressBucket = "address"
)

var (
	db *bolt.DB
	ln net.Listener
)

func Web(cmd *cobra.Command, args []string) {

	conf, err := globalconf.New(appName)
	if err != nil {
		log.WithError(err).Fatal()
	}
	conf.ParseAll()

	db, err := bolt.Open(viper.GetString("db"), 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.WithError(err).Fatal()
	}
	defer db.Close()

	if viper.GetString("setip") != "" {
		if err = addresses.DBinit(db); err != nil {
			log.Fatal(err)
		}
		splited := strings.Split(viper.GetString("setip"), ":")
		if len(splited) < 3 {
			log.Fatal("ID:Link:CIDR required")
		}
		id, link, ip := splited[0], splited[1], splited[2]
		addresses.CommandSetIP(id, link, ip)
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

		&rest.Route{"GET", "/dhcp", dhcp.GetDhcp},
		&rest.Route{"POST", "/dhcp", dhcp.PostDhcp},

		&rest.Route{"GET", "/dns", dns.GetDNS},
		&rest.Route{"POST", "/dns", dns.PostDNS},

		&rest.Route{"GET", "/routes", getRoutes},
		&rest.Route{"POST", "/routes/gateway", gateway.PostGateway},
		&rest.Route{"GET", "/routes/gateway", gateway.GetGateway},
	)
	if err != nil {
		log.Fatal(err)
	}

	api.SetApp(router)

	var network string
	if _, err = net.ResolveTCPAddr("tcp", viper.GetString("bind")); err == nil {
		network = "tcp"
	} else {
		network = "unix"
	}
	ln, err = net.Listen(network, viper.GetString("bind"))
	if nil != err {
		log.WithError(err).Fatal()
	}
	defer ln.Close()

	if viper.GetString("owner") != "" && network == "unix" {
		user, err := user.Lookup(viper.GetString("owner"))
		if err != nil {
			log.WithError(err).Fatal()
		}
		uid, err := strconv.Atoi(user.Uid)
		if err != nil {
			log.WithError(err).Fatal()
		}
		var gid int
		if viper.GetInt("group") != -1 {
			gid = viper.GetInt("group")
		} else {
			gid, err = strconv.Atoi(user.Gid)
			if err != nil {
				log.WithError(err).Fatal()
			}
		}
		if err := os.Chown(viper.GetString("bind"), uid, gid); err != nil {
			log.WithError(err).Fatal()
		}
		if err := os.Chmod(viper.GetString("bind"), 0660); err != nil {
			log.WithError(err).Fatal()
		}
	}

	if err := dhcp.DBinit(db); err != nil {
		log.WithError(err).Fatal()
	}
	if err := addresses.DBinit(db); err != nil {
		log.WithError(err).Fatal()
	}
	if err := dns.DBinit(db); err != nil {
		log.WithError(err).Fatal()
	}
	if err := gateway.DBinit(db); err != nil {
		log.WithError(err).Fatal()
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

	log.Printf("Now listening to bind %s", viper.GetString("bind"))
	log.Fatal(http.Serve(ln, api.MakeHandler()))
}

func getRoutes(w rest.ResponseWriter, req *rest.Request) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteJson(routes)
}
