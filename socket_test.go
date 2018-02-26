package varroa

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDaemonSocket(t *testing.T) {
	fmt.Println("+ Testing SocketCom...")
	check := assert.New(t)

	// setup logger
	c := &Config{General: &ConfigGeneral{LogLevel: 3}}
	env := &Environment{config: c}
	logThis = NewLogThis(env)
	// cleanup if things fail
	// defer os.Remove(daemonSocket)

	dcServer := NewDaemonComServer()
	dcClient := NewDaemonComClient()

	go dcServer.RunServer()
	<-dcServer.ServerUp
	go dcClient.RunClient()
	<-dcClient.ClientConnected
	<-dcServer.ClientConnected

	// goroutines to process what the server / client receives
	// the server routines echoes what it gets back to the client
	go func() {
		for {
			a := <-dcServer.Incoming
			fmt.Println("SERVER received: " + string(a))
			outMessage := append([]byte("echo: "), a...)
			dcServer.Outgoing <- outMessage
			if string(a) == "command before stop" {
				time.Sleep(1 * time.Millisecond)
				dcServer.Outgoing <- []byte(stopCommand)
			}
		}
	}()
	go func() {
		for {
			a := <-dcClient.Incoming
			fmt.Println("CLIENT received: " + string(a))
		}
	}()

	// connection is active and messages are exchanges
	check.True(dcClient.IsActive)
	check.True(dcServer.IsActive)
	dcServer.Outgoing <- []byte("one")
	dcClient.Outgoing <- []byte("two")
	dcClient.Outgoing <- []byte("three !!!")
	verylong := make([]byte, 3000)
	for i := range verylong {
		verylong[i] = 'O'
	}
	verylong[0] = 'a'
	verylong[len(verylong)-1] = 'b'
	dcClient.Outgoing <- verylong

	// stopping programmatically
	time.Sleep(1 * time.Millisecond)
	fmt.Println("- Stopping client.")
	dcClient.StopCurrent()
	<-dcClient.ClientDisconnected
	<-dcServer.ClientDisconnected
	check.False(dcClient.IsActive)

	for i := 0; i < 20; i++ {
		time.Sleep(1 * time.Millisecond)
		fmt.Println("- Start client again.")
		go dcClient.RunClient()
		<-dcClient.ClientConnected
		<-dcServer.ClientConnected

		dcClient.Outgoing <- []byte("this is a quite formidable test.")
		time.Sleep(1 * time.Millisecond)
		dcClient.Outgoing <- []byte("command before stop")

		<-dcClient.ClientDisconnected
		<-dcServer.ClientDisconnected
		fmt.Println("- Client should have been stopped by command.")
		check.False(dcClient.IsActive)
	}

	check.True(dcServer.IsListening)
	fmt.Println("- Stopping server")
	dcServer.StopCurrent()

	time.Sleep(1 * time.Millisecond)
	check.False(dcServer.IsListening)
}
