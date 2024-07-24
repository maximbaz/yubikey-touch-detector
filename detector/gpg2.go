package detector

import (
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

// WatchGPGV2 watches for hints that YubiKey is maybe waiting for a touch on a GPG auth request
func WatchGPGV2(requestGPGCheck chan bool, exits *sync.Map) {
	socketFile := ""

	if socketFile == "" {
		gpgAgentSocket, err := exec.Command("gpgconf", "--list-dirs", "agent-socket").CombinedOutput()
		gpgAgentSocketOutput := strings.TrimSpace(string(gpgAgentSocket))
		if err != nil {
			log.Errorf("Cannot find GPGV2 socket using gpgconf, error: %v, stderr: %v", err, gpgAgentSocketOutput)
		} else {
			socketFile = gpgAgentSocketOutput
		}
	}

	if socketFile == "" {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir != "" {
			socketFile = path.Join(runtimeDir, "gnupg/S.gpg-agent")
		}
	}

	if socketFile == "" {
		log.Error("Cannot watch GPGV2, gpgconf --list-dirs agent-socket didn't help, and $XDG_RUNTIME_DIR is not defined.")
		return
	}

	if _, err := os.Stat(socketFile); err != nil {
		log.Errorf("Cannot watch GPGV2, the socket '%v' does not exist: %v", socketFile, err)
		return
	}

	originalSocketFile := socketFile + ".original"
	if _, err := os.Stat(originalSocketFile); err == nil {
		log.Warnf("'%v' already exists, assuming it's the correct one and trying to recover", originalSocketFile)
		if err = os.Remove(socketFile); err != nil {
			log.Errorf("Cannot remove '%v' in order to recover from possible previous crash", socketFile)
			return
		}
	} else {
		if err := os.Rename(socketFile, originalSocketFile); err != nil {
			log.Error("Cannot move original GPGV2 socket file to setup a proxy: ", err)
			return
		}
	}

	proxySocket, err := net.Listen("unix", socketFile)
	if err != nil {
		log.Error("Cannot establish a proxy GPGV2 socket: ", err)
		if err := os.Rename(originalSocketFile, socketFile); err != nil {
			log.Error("Cannot restore original GPGV2 socket: ", err)
		}
		return
	}
	log.Debug("GPGV2 watcher is successfully established")

	exit := make(chan bool)
	exits.Store("detector/ssh", exit)
	go func() {
		<-exit
		if err := proxySocket.Close(); err != nil {
			log.Error("Cannot cleanup proxy GPGV2 socket: ", err)
		}
		if err := os.Rename(originalSocketFile, socketFile); err != nil {
			log.Error("Cannot restore original GPGV2 socket: ", err)
		}
		exit <- true
	}()

	for {
		proxyConnection, err := proxySocket.Accept()
		if err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				log.Error("Cannot accept incoming proxy connection: ", err)
			}
			return
		}
		originalConnection, err := net.Dial("unix", originalSocketFile)
		if err != nil {
			log.Error("Cannot establish connection to original socket: ", err)
			proxyConnection.Close()
			return
		}

		go proxyGpgv2UnixSocket(proxyConnection, originalConnection, requestGPGCheck)
		go proxyGpgv2UnixSocket(originalConnection, proxyConnection, requestGPGCheck)
	}
}

func proxyGpgv2UnixSocket(reader net.Conn, writer net.Conn, requestGPGCheck chan bool) {
	defer (func() {
		reader.Close()
		writer.Close()
	})()

	buf := make([]byte, 10240)
	for {
		nr, err := reader.Read(buf)
		if err != nil {
			return
		}

		data := buf[0:nr]
		if _, err = writer.Write(data); err != nil {
			return
		}

		if !strings.Contains(string(data), "PKDECRYPT") {
			continue
		}
		log.Debugf("GPG operation detected!")

		select {
		case requestGPGCheck <- true:
		default:
		}
	}
}
