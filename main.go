package main

import (
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

func checkGPGCardStatus() *exec.Cmd {
	cmd := exec.Command("gpg", "--no-tty", "--card-status")
	if err := cmd.Start(); err != nil {
		log.Error(err)
	}
	return cmd
}

func checkGPGOnRequest(requestGPGCheck chan bool) {
	for {
		<-requestGPGCheck

		for i := 0; i < 20; i++ {
			cmd := checkGPGCardStatus()
			timer := time.AfterFunc(100*time.Millisecond, func() {
				cmd.Process.Kill()
			})
			err := cmd.Wait()
			timer.Stop()

			if err != nil {
				log.Debug("GPG START")
				checkGPGCardStatus().Wait()
				log.Debug("GPG STOP")
				break
			}
		}
	}
}

func watchGPG(requestGPGCheck chan bool) {
	// We are only interested in the first event, should skip all subsequent ones
	events := make(chan notify.EventInfo)
	file := os.ExpandEnv("$HOME/.gnupg/pubring.kbx")
	if err := notify.Watch(file, events, notify.InOpen); err != nil {
		log.Error("Cannot establish a watch on GPG file: ", err)
		return
	}
	defer notify.Stop(events)

	for {
		select {
		case <-events:
			select {
			case requestGPGCheck <- true:
			default:
			}
		}
	}
}

func watchU2F() {
	// It's important to not miss a single event, so have a small buffer
	events := make(chan notify.EventInfo, 10)
	file := os.ExpandEnv("$HOME/.config/Yubico/u2f_keys")
	if err := notify.Watch(file, events, notify.InOpen); err != nil {
		log.Error("Cannot establish a watch on U2F file: ", err)
		return
	}
	defer notify.Stop(events)

	for {
		select {
		case <-events:
			log.Debugln("U2F START")
			<-events
			log.Debugln("U2F STOP")
		}
	}
}

func watchSSH(requestGPGCheck chan bool, exits map[string]chan bool) {
	socketFile := os.Getenv("SSH_AUTH_SOCK")
	if _, err := os.Stat(socketFile); err != nil {
		log.Error("Cannot watch SSH, $SSH_AUTH_SOCK does not exist: ", err)
		return
	}

	originalSocketFile := socketFile + ".original"
	if _, err := os.Stat(originalSocketFile); err == nil {
		log.Error("Cannot watch SSH, $SSH_AUTH_SOCK.original already exists")
		return
	}

	if err := os.Rename(socketFile, originalSocketFile); err != nil {
		log.Error("Cannot move original SSH socket file to setup a proxy: ", err)
		return
	}

	proxySocket, err := net.Listen("unix", socketFile)
	if err != nil {
		log.Error("Cannot establish a proxy SSH socket: ", err)
		if err := os.Rename(originalSocketFile, socketFile); err != nil {
			log.Error("Cannot restore original SSH socket: ", err)
		}
		return
	}

	exit := make(chan bool)
	exits["ssh"] = exit
	go func() {
		<-exit
		if err := proxySocket.Close(); err != nil {
			log.Error("Cannot cleanup proxy SSH socket: ", err)
		}
		if err := os.Rename(originalSocketFile, socketFile); err != nil {
			log.Error("Cannot restore original SSH socket: ", err)
		}
		exit <- true
	}()

	for {
		proxyConnection, err := proxySocket.Accept()
		if err != nil {
			log.Error("Cannot accept incoming proxy connection: ", err)
			return
		}
		originalConnection, err := net.Dial("unix", originalSocketFile)
		if err != nil {
			log.Error("Cannot establish connection to original socket: ", err)
			proxyConnection.Close()
			return
		}

		go proxyUnixSocket(proxyConnection, originalConnection, requestGPGCheck)
		go proxyUnixSocket(originalConnection, proxyConnection, requestGPGCheck)
	}
}

func proxyUnixSocket(reader net.Conn, writer net.Conn, requestGPGCheck chan bool) {
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

		select {
		case requestGPGCheck <- true:
		default:
		}
	}
}

func setupExitSignalWatch(exits map[string]chan bool) {
	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, os.Interrupt, syscall.SIGTERM)

	<-exitSignal
	println()

	for _, exit := range exits {
		exit <- true // Notify exit watcher
		<-exit       // Wait for confirmation
	}

	log.Info("Stopping Yubikey touch detector")
	os.Exit(0)
}

func main() {
	log.SetLevel(log.DebugLevel)
	log.Info("Starting Yubikey touch detector")

	exits := make(map[string]chan bool)
	go setupExitSignalWatch(exits)

	requestGPGCheck := make(chan bool)
	go checkGPGOnRequest(requestGPGCheck)

	go watchU2F()
	go watchGPG(requestGPGCheck)
	go watchSSH(requestGPGCheck, exits)

	wait := make(chan bool)
	<-wait
}
