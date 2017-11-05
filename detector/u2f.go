package detector

import (
	"os"

	"github.com/maximbaz/yubikey-touch-detector/notifier"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

// WatchU2F watches when YubiKey is waiting for a touch on a U2F request
func WatchU2F(notifiers map[string]chan notifier.Message) {
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
			for _, n := range notifiers {
				n <- notifier.U2F_ON
			}

			<-events

			for _, n := range notifiers {
				n <- notifier.U2F_OFF
			}
		}
	}
}
