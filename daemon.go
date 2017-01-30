package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	daemon "github.com/sevlyar/go-daemon"
)

/*
- /nick NEWNICK
- /msg NickServ register PSWWF ADDR
*/

//------------------

var (
	signal = flag.String("s", "", `send signal to the daemon
		quit — graceful shutdown
		stop — fast shutdown
		reload — reloading the configuration file`)
)

func main() {
	flag.Parse()
	daemon.AddCommand(daemon.StringFlag(signal, "quit"), syscall.SIGQUIT, termHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGTERM, termHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "reload"), syscall.SIGHUP, reloadHandler)

	cntxt := &daemon.Context{
		PidFileName: "pid",
		PidFilePerm: 0644,
		LogFileName: "log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
		Args:        []string{"[irc bot for PTH]"},
	}

	if len(daemon.ActiveFlags()) > 0 {
		d, err := cntxt.Search()
		if err != nil {
			log.Fatalln("Unable send signal to the daemon:", err)
		}
		daemon.SendCommands(d)
		return
	}

	d, err := cntxt.Reborn()
	if err != nil {
		log.Fatalln(err)
	}
	if d != nil {
		return
	}
	defer cntxt.Release()

	log.Println("+ dronelistener started")

	conf := Config{}
	conf.load("config.yaml")
	log.Println(" - Configuration loaded.")

	tracker := GazelleTracker{rootURL: conf.url}
	if err := tracker.Login(conf.user, conf.password); err != nil {
		fmt.Println(err.Error())
		return
	}
	log.Println(" - Logged in tracker.")

	go checkSignals()
	go ircHandler(conf, tracker)
	go monitorStats(conf, tracker)

	err = daemon.ServeSignals()
	if err != nil {
		log.Println("Error:", err)
	}
	log.Println("+ dronelistener stopped")
}

var (
	stop = make(chan struct{})
	done = make(chan struct{})
)

func checkSignals() {
	for {
		time.Sleep(time.Second)
		if _, ok := <-stop; ok {
			break
		}
	}
	done <- struct{}{}
}

func termHandler(sig os.Signal) error {
	log.Println("terminating...")
	stop <- struct{}{}
	if sig == syscall.SIGQUIT {
		<-done
	}
	return daemon.ErrStop
}

func reloadHandler(sig os.Signal) error {
	log.Println("configuration reloaded")
	return nil
}
