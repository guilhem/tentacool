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
)

var (
	flagSocket = flag.String("socket", "/var/run/surysys", "Path for unix socket")
	flagOwner  = flag.String("owner", "", "Ownership for socket")
	// flagMode   = flag.Int("mode", 0640, "FileMode for socket")
)

type Message struct {
	Body string
}

func main() {
	conf, err := globalconf.New("myapp")
	if err != nil {
		log.Fatal(err)
	}
	conf.ParseAll()

	handler := rest.ResourceHandler{}
	err = handler.SetRoutes(
		&rest.Route{"GET", "/message", func(w rest.ResponseWriter, req *rest.Request) {
			w.WriteJson(&Message{
				Body: "Hello World!",
			})
		}},
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
		gid, err := strconv.Atoi(user.Gid)
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
