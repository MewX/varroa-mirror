package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/gregdel/pushover"
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

	cntxt = &daemon.Context{
		PidFileName: "pid",
		PidFilePerm: 0644,
		LogFileName: "log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
		Args:        []string{"[irc bot for PTH]"},
	}

	conf = &Config{}
)

func main() {
	flag.Parse()
	daemon.AddCommand(daemon.StringFlag(signal, "quit"), syscall.SIGQUIT, termHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGTERM, termHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "reload"), syscall.SIGHUP, reloadHandler)

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

	if err := conf.load("config.yaml"); err != nil {
		log.Println(" - Error loading configuration: " + err.Error())
		return
	}
	log.Println(" - Configuration loaded.")

	// notifications with pushover
	var notification *pushover.Pushover
	var recipient *pushover.Recipient
	if conf.pushoverUser != "" && conf.pushoverToken != "" {
		notification = pushover.New(conf.pushoverToken)
		recipient = pushover.NewRecipient(conf.pushoverUser)
	}

	tracker := GazelleTracker{rootURL: conf.url}
	if err := tracker.Login(conf.user, conf.password); err != nil {
		fmt.Println(err.Error())
		return
	}
	log.Println(" - Logged in tracker.")

	go checkSignals()
	go ircHandler(conf, tracker, notification, recipient)
	go monitorStats(conf, tracker, notification, recipient)

	if err := daemon.ServeSignals(); err != nil {
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
	if err := conf.load("config.yaml"); err != nil {
		log.Println(" - Error loading configuration: " + err.Error())
		return nil
	}
	log.Println(" - Configuration reloaded.")
	return nil
}

func killDaemon() {
	d, err := cntxt.Search()
	if err != nil {
		log.Fatalln("Unable send signal to the daemon:", err)
	}
	if d != nil {
		if err := d.Signal(syscall.SIGTERM); err != nil {
			log.Fatalf("error killing running daemon: %s\n", err)
		}

		// Ascertain process has exited
		for {
			if err := d.Signal(syscall.Signal(0)); err != nil {
				if err.Error() == "os: process already finished" {
					break
				}
				log.Fatalf("error checking daemon exited: %s\n", err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
