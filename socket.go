package varroa

import (
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/pkg/errors"
)

// DaemonCom is a standalone struct to handle communication with the daemon, using a unix domain socket.
type DaemonCom struct {
	IsServer           bool
	IsActive           bool
	IsListening        bool
	Incoming           chan []byte
	Outgoing           chan []byte
	ServerUp           chan struct{}
	ClientConnected    chan struct{}
	ClientDisconnected chan struct{}
	socket             string
	endThisConnection  chan struct{}
	err                chan error
	connection         *net.Conn
	listener           *net.Listener
}

func NewDaemonCom(server bool) *DaemonCom {
	in := make(chan []byte)
	out := make(chan []byte)
	err := make(chan error)
	end := make(chan struct{})
	disconnect := make(chan struct{})
	connect := make(chan struct{})
	serverUp := make(chan struct{})
	return &DaemonCom{socket: daemonSocket, Incoming: in, Outgoing: out, err: err, endThisConnection: end, ServerUp: serverUp, ClientDisconnected: disconnect, ClientConnected: connect, IsServer: server}
}

func NewDaemonComServer() *DaemonCom {
	return NewDaemonCom(true)
}

func NewDaemonComClient() *DaemonCom {
	return NewDaemonCom(false)
}

func (dc *DaemonCom) RunServer() error {
	// to be run as a goroutine
	if !dc.IsServer {
		return errors.New("not a server")
	}

	listener, err := net.Listen("unix", dc.socket)
	if err != nil {
		logThis.Error(errors.Wrap(err, "error creating socket"), NORMAL)
		return errors.Wrap(err, "error creating socket")
	}
	dc.listener = &listener
	dc.IsListening = true
	dc.ServerUp <- struct{}{}

	// it sends things from the socket to in
	go dc.waitForErrors(false)
	dc.serverReceive()

	return nil
}

func (dc *DaemonCom) RunClient() error {
	// to be run as a goroutine
	if dc.IsServer {
		return errors.New("not a client")
	}
	conn, err := net.Dial("unix", dc.socket)
	if err != nil {
		return errors.Wrap(err, errorDialingSocket)
	}
	dc.connection = &conn
	dc.IsActive = true
	dc.ClientConnected <- struct{}{}

	// it writes things from out to the socket
	go dc.waitForErrors(true)
	go dc.send()
	go dc.clientReceive()

	return nil
}

func (dc *DaemonCom) waitForErrors(exitOnError bool) {
	for {
		err := <-dc.err
		logThis.Info(fmt.Sprintf("Got error %s, closing connection (server:%v)", err.Error(), dc.IsServer), VERBOSESTEST)
		dc.endThisConnection <- struct{}{}
		if exitOnError {
			break
		}
	}
}

func (dc *DaemonCom) serverReceive() {
	// goroutine to read from socket
	for dc.IsListening {
		logThis.Info("Server waiting for connection.", VERBOSESTEST)
		conn, err := (*dc.listener).Accept()
		if err != nil {
			if dc.IsListening {
				logThis.Info("Error accepting from unix socket: "+err.Error(), VERBOSESTEST)
				continue
			} else {
				break
			}
		}
		dc.connection = &conn
		dc.IsActive = true
		dc.ClientConnected <- struct{}{}
		logThis.Info("Server connected.", VERBOSESTEST)

		go dc.send()
		go dc.read()

		// waiting for the other instance to be warned that communication is over
		<-dc.endThisConnection
		dc.IsActive = false
		(*dc.connection).Close()
		dc.ClientDisconnected <- struct{}{}
		logThis.Info("Closing server connection.", VERBOSESTEST)
	}
}

func (dc *DaemonCom) read() {
	for {
		logThis.Info(fmt.Sprintf("Waiting for read (server:%v).", dc.IsServer), VERBOSESTEST)
		buf := make([]byte, 2048)
		n, err := (*dc.connection).Read(buf[:])
		if err != nil {
			if dc.IsActive {
				if err != io.EOF {
					logThis.Error(errors.Wrap(err, errorReadingFromSocket+fmt.Sprintf(" (server:%v)", dc.IsServer)), NORMAL)
				}
				dc.err <- err
			}
			break
		}
		//fmt.Println("HHHH" + string(buf[:n]))

		stopAfterThis := false
		for _, part := range strings.Split(string(buf[:n]), unixSocketMessageSeparator) {
			if part == "" {
				continue
			}
			dc.Incoming <- []byte(part)

			// if we are the client and receive stop, the server just told us it's the end of the communication.
			// -1 because of unixSocketMessageSeparator
			if part == "stop" && !dc.IsServer {
				stopAfterThis = true
			}
		}
		if stopAfterThis {
			logThis.Info("Client: Received order to stop connection", VERBOSESTEST)
			dc.endThisConnection <- struct{}{}
			break
		}
	}
}

func (dc *DaemonCom) clientReceive() {
	// goroutine to read from socket
	go dc.read()

	// waiting for the other instance to be warned that communication is over
	<-dc.endThisConnection
	dc.IsActive = false
	(*dc.connection).Close()
	dc.ClientDisconnected <- struct{}{}
}

func (dc *DaemonCom) send() {
	// goroutine to write to socket
	for {
		logThis.Info(fmt.Sprintf("Waiting for something to send (server:%v).", dc.IsServer), VERBOSESTEST)
		messageToLog := <-dc.Outgoing
		messageToLog = append(messageToLog, []byte(unixSocketMessageSeparator)...)
		// writing to socket with a separator, so that the other instance, reading more slowly,
		// can separate messages that might have been written one after the other
		if _, err := (*dc.connection).Write(messageToLog); err != nil {
			if dc.IsActive {
				if err != io.EOF {
					logThis.Error(errors.Wrap(err, errorWritingToSocket), NORMAL)
				}
				dc.err <- err
			}
			break
		}
		// we've just told the other instance talking was over, ending this connection.
		if string(messageToLog) == "stop" && dc.IsServer {
			logThis.Info("Server: Sent order to stop connection", VERBOSESTEST)
			dc.endThisConnection <- struct{}{}
			dc.IsActive = false
			break
		}

	}
}

func (dc *DaemonCom) StopCurrent() {
	if dc.IsActive {
		logThis.Info("Stopping current connection.", VERBOSESTEST)
		dc.endThisConnection <- struct{}{}
		dc.IsActive = false
	}

	if dc.IsServer {
		dc.IsListening = false
		(*dc.listener).Close()
	}

}
