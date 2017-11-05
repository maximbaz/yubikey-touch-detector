package notifier

import (
	"net"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
)

// SetupUnixSocketNotifier configures a unix socket to transmit touch requests to other apps
func SetupUnixSocketNotifier(notifiers map[string]chan Message, exits map[string]chan bool) {
	socketDir := os.Getenv("XDG_RUNTIME_DIR")
	if _, err := os.Stat(socketDir); err != nil {
		log.Error("Cannot setup unix socket notifier, $XDG_RUNTIME_DIR does not exist: ", err)
		return
	}

	socketFile := path.Join(socketDir, "yubikey-touch-detector.socket")
	if _, err := os.Stat(socketFile); err == nil {
		log.Errorf("Cannot setup unix socket notifier, %v already exists\n", socketFile)
		return
	}

	socket, err := net.Listen("unix", socketFile)
	if err != nil {
		log.Error("Cannot establish a proxy SSH socket: ", err)
		return
	}

	exit := make(chan bool)
	exits["notifier/unix_socket"] = exit
	go func() {
		<-exit
		if err := socket.Close(); err != nil {
			log.Error("Cannot cleanup unix socket notifier: ", err)
		}
		exit <- true
	}()

	touch := make(chan Message, 10)
	notifiers["notifier/unix_socket"] = touch

	touchListeners := make(map[*net.Conn]chan []byte)
	go func() {
		for {
			value := <-touch
			for _, listener := range touchListeners {
				listener <- []byte(value)
			}
		}
	}()

	for {
		listener, err := socket.Accept()
		if err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				log.Error("Cannot accept incoming unix socket notifier connection: ", err)
			}
			return
		}

		go notify(listener, touchListeners)
	}
}

func notify(listener net.Conn, touchListeners map[*net.Conn]chan []byte) {
	values := make(chan []byte)
	touchListeners[&listener] = values
	defer (func() {
		delete(touchListeners, &listener)
		listener.Close()
	})()

	for {
		value := <-values
		if _, err := listener.Write(value); err != nil {
			return
		}
	}
}
