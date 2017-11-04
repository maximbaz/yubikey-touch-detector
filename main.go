package main

import (
	"io"
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

func checkGPG() {
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

func watchGPG() {
	// We are only interested in the first event, should skip all subsequent ones
	events := make(chan notify.EventInfo)
	file := os.ExpandEnv("$HOME/.gnupg/pubring.kbx")
	if err := notify.Watch(file, events, notify.InOpen); err != nil {
		log.Fatal(err)
	}
	defer notify.Stop(events)

	for {
		select {
		case <-events:
			checkGPG()
		}
	}
}

func watchU2F() {
	// It's important to not miss a single event, so have a small buffer
	events := make(chan notify.EventInfo, 10)
	file := os.ExpandEnv("$HOME/.config/Yubico/u2f_keys")
	if err := notify.Watch(file, events, notify.InOpen); err != nil {
		log.Fatal(err)
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

func watchSSH(proxySocket net.Listener, originalSocketFile string) {
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

		go proxyUnixSocket(proxyConnection, originalConnection, "proxy")
		go proxyUnixSocket(originalConnection, proxyConnection, "original")
	}
}

func setupWatchSSH() chan bool {
	socketFile := os.Getenv("SSH_AUTH_SOCK")
	if _, err := os.Stat(socketFile); err != nil {
		log.Error("Cannot watch SSH, $SSH_AUTH_SOCK does not exist: ", err)
		return nil
	}

	originalSocketFile := socketFile + ".original"
	if _, err := os.Stat(originalSocketFile); err == nil {
		log.Error("Cannot watch SSH, $SSH_AUTH_SOCK.original already exists")
		return nil
	}

	if err := os.Rename(socketFile, originalSocketFile); err != nil {
		log.Error("Cannot move original SSH socket file to setup a proxy: ", err)
		return nil
	}

	proxySocket, err := net.Listen("unix", socketFile)
	if err != nil {
		log.Error("Cannot establish a proxy SSH socket: ", err)
		if err := os.Rename(originalSocketFile, socketFile); err != nil {
			log.Error("Cannot restore original SSH socket: ", err)
		}
		return nil
	}

	exit := make(chan bool)
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

	go watchSSH(proxySocket, originalSocketFile)

	return exit
}

func proxyUnixSocket(reader net.Conn, writer net.Conn, id string) {
	defer (func() {
		reader.Close()
		writer.Close()
	})()

	buf := make([]byte, 10240)
	for {
		nr, err := reader.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("[%v] Error reading from the socket: %v", id, err)
			}
			return
		}

		data := buf[0:nr]
		_, err = writer.Write(data)
		if err != nil {
			if err != io.EOF {
				log.Printf("[%v] Error writing to the socket: %v", id, err)
			}
			return
		}

		checkGPG()
	}
}

func main() {
	log.SetLevel(log.DebugLevel)

	log.Info("Starting Yubikey touch detector")

	exits := []chan bool{}
	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-exitSignal
		println()

		for _, exit := range exits {
			exit <- true // Notify exit watcher
			<-exit       // Wait for confirmation
		}

		log.Info("Stopping Yubikey touch detector")
		os.Exit(0)
	}()

	go watchU2F()
	go watchGPG()
	exitSSH := setupWatchSSH()
	if exitSSH != nil {
		exits = append(exits, exitSSH)
	}

	wait := make(chan bool)
	<-wait
}
