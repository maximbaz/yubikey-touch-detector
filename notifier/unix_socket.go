package notifier

import (
	"net"
	"os"
	"path"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

// SetupUnixSocketNotifier configures a unix socket to transmit touch requests to other apps
func SetupUnixSocketNotifier(notifiers map[string]chan Message, exits map[string]chan bool) {
	socketDir := os.Getenv("XDG_RUNTIME_DIR")
	if socketDir == "" {
		log.Error("Cannot setup unix socket notifier, $XDG_RUNTIME_DIR is not defined.")
		return
	}

	if _, err := os.Stat(socketDir); err != nil {
		log.Errorf("Cannot setup unix socket notifier, folder '%v' does not exist: %v", socketDir, err)
		return
	}

	socketFile := path.Join(socketDir, "yubikey-touch-detector.socket")
	if _, err := os.Stat(socketFile); err == nil {
		log.Errorf("Cannot setup unix socket notifier, '%v' already exists", socketFile)
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
	touchListenersMutex := sync.RWMutex{}
	go func() {
		for {
			value := <-touch
			touchListenersMutex.RLock()
			for _, listener := range touchListeners {
				listener <- []byte(value)
			}
			touchListenersMutex.RUnlock()
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

		go notify(listener, touchListeners, &touchListenersMutex)
	}
}

func notify(listener net.Conn, touchListeners map[*net.Conn]chan []byte, touchListenersMutex *sync.RWMutex) {
	values := make(chan []byte)
	touchListenersMutex.Lock()
	touchListeners[&listener] = values
	touchListenersMutex.Unlock()
	defer (func() {
		touchListenersMutex.Lock()
		delete(touchListeners, &listener)
		touchListenersMutex.Unlock()
		listener.Close()
	})()

	for {
		value := <-values
		if _, err := listener.Write(value); err != nil {
			return
		}
	}
}
