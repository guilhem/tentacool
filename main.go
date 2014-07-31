package main

import (
	"github.com/ant0ine/go-json-rest/rest"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

type Message struct {
	Body string
}

func main() {
	handler := rest.ResourceHandler{}
	err := handler.SetRoutes(
		&rest.Route{"GET", "/message", func(w rest.ResponseWriter, req *rest.Request) {
			w.WriteJson(&Message{
				Body: "Hello World!",
			})
		}},
	)
	if err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", "/tmp/lol")
	if err != nil {
		log.Fatal(err)
	}
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
