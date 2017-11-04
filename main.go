package main

import (
	"os"

	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

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

	wait := make(chan bool)
	<-wait
}
