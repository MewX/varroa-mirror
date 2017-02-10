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

//------------------

const (
	// PTH only allows 5 API calls every 10s
	allowedAPICallsByPeriod = 5
	apiCallsPeriodS         = 10
)

var (
	signal = flag.String("s", "", `send orders to the daemon:
		reload — reload the configuration file
		quit   — graceful shutdown
		stop   — fast shutdown`)
	daemonContext = &daemon.Context{
		PidFileName: "pid",
		PidFilePerm: 0644,
		LogFileName: "log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
		Args:        []string{"[irc bot for PTH]"},
	}
	conf         = &Config{}
	notification = &Notification{}

	// daemon control channels
	stop = make(chan struct{})
	done = make(chan struct{})

	// channel of allowedAPICallsByPeriod elements, which will rate-limit the requests
	limiter = make(chan bool, allowedAPICallsByPeriod)
)

func main() {
	flag.Parse()
	daemon.AddCommand(daemon.StringFlag(signal, "quit"), syscall.SIGQUIT, quitDaemon)
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGTERM, quitDaemon)
	daemon.AddCommand(daemon.StringFlag(signal, "reload"), syscall.SIGHUP, loadConfiguration)

	if len(daemon.ActiveFlags()) > 0 {
		d, err := daemonContext.Search()
		if err != nil {
			log.Fatalln("Unable send signal to the daemon:", err)
		}
		daemon.SendCommands(d)
		return
	}
	d, err := daemonContext.Reborn()
	if err != nil {
		log.Fatalln(err)
	}
	if d != nil {
		return
	}
	defer daemonContext.Release()

	log.Println("+ varroa musica started")
	// load configuration
	if err := loadConfiguration(nil); err != nil {
		return
	}
	// init notifications with pushover
	if conf.pushoverConfigured() {
		notification.client = pushover.New(conf.pushoverToken)
		notification.recipient = pushover.NewRecipient(conf.pushoverUser)
	}
	// log in tracker
	tracker := GazelleTracker{rootURL: conf.url}
	if err := tracker.Login(conf.user, conf.password); err != nil {
		fmt.Println(err.Error())
		return
	}
	log.Println(" - Logged in tracker.")

	go checkSignals()
	go ircHandler(tracker)
	go monitorStats(tracker)
	go apiCallRateLimiter()

	if err := daemon.ServeSignals(); err != nil {
		log.Println("Error:", err)
	}
	log.Println("+ varroa musica stopped")
}

func checkSignals() {
	for {
		time.Sleep(time.Second)
		if _, ok := <-stop; ok {
			break
		}
	}
	done <- struct{}{}
}

func loadConfiguration(sig os.Signal) error {
	newConf := &Config{}
	if err := newConf.load("config.yaml"); err != nil {
		log.Println(" - Error loading configuration: " + err.Error())
		return err
	}
	conf = newConf
	log.Println(" - Configuration reloaded.")
	return nil
}

func quitDaemon(sig os.Signal) error {
	log.Println("terminating...")
	stop <- struct{}{}
	if sig == syscall.SIGQUIT {
		<-done
	}
	return daemon.ErrStop
}

func killDaemon() {
	d, err := daemonContext.Search()
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

func apiCallRateLimiter() {
	// fill the rate limiter the first time
	for i := 0; i < allowedAPICallsByPeriod; i++ {
		limiter <- true
	}
	// every apiCallsPeriodS, refill the limiter channel
	for range time.Tick(time.Second * time.Duration(apiCallsPeriodS)) {
		for i := 0; i < allowedAPICallsByPeriod; i++ {
			select {
			case limiter <- true:
			default:
				// if channel is full, do nothing
				break
			}
		}
	}
}
