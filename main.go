package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/rakyll/globalconf"
	"github.com/milosgajdos83/tenus"
)

const (
	appName = "surysys"
	)

var (
	flagSocket = flag.String("socket", "/var/run/" + appName, "Path for unix socket")
	flagOwner  = flag.String("owner", "", "Ownership for socket")
	flagGroup  = flag.Int("group", -1, "Group for socket")
	// flagMode   = flag.Int("mode", 0640, "FileMode for socket")
)

type Message struct {
	Body string
}

type Iface struct {
	Name string
	Ip string
}

func main() {
	conf, err := globalconf.New(appName)
	if err != nil {
		log.Fatal(err)
	}
	conf.ParseAll()

	handler := rest.ResourceHandler{}
	err = handler.SetRoutes(
		&rest.Route{"GET", "/message", Hello},
		&rest.Route{"GET", "/interfaces/:iface", GetIface},
		&rest.Route{"POST", "/interfaces", PostIface},
	)
	if err != nil {
		log.Fatal(err)
	}

	ln, err := net.Listen("unix", *flagSocket)
	if err != nil {
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
	// if err := os.Chmod(*flagSocket, *flagMode); err != nil {
	// 	log.Fatal(err)
	// }

	// Unix sockets must be unlink()ed before being reused again.

	// Handle common process-killing signals so we can gracefully shut down:
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill, syscall.SIGTERM)
	go func(c chan os.Signal) {
		// Wait for a SIGINT or SIGKILL:
		sig := <-c
		log.Printf("Caught signal %s: shutting down.", sig)
		// Stop listening (and unlink the socket if unix type):
		ln.Close()
		// And we're done:
		os.Exit(0)
	}(sigc)

	log.Fatal(http.Serve(ln, &handler))
}


func Hello(w rest.ResponseWriter, req *rest.Request) {
	w.WriteJson(&Message{
		Body: "Hello World!",
	})
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

func PostIface(w rest.ResponseWriter, req *rest.Request) {
	iface := Iface{}
  if err := req.DecodeJsonPayload(&iface); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ip, ipnet, err := net.ParseCIDR(iface.Ip)
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	link, err := tenus.NewLink(iface.Name)
	if err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := link.SetLinkIp(ip, ipnet); err != nil {
		log.Printf(err.Error())
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteJson(ipnet)
}
