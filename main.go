package main

import (
	"os"
	"os/exec"
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

func main() {
	log.SetLevel(log.DebugLevel)

	go watchU2F()
	go watchGPG()

	wait := make(chan bool)
	<-wait
}
