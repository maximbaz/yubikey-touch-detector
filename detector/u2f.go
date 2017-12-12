package detector

import (
	"github.com/maximbaz/yubikey-touch-detector/notifier"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

// WatchU2F watches when YubiKey is waiting for a touch on a U2F request
func WatchU2F(u2fKeysPath string, notifiers map[string]chan notifier.Message) {
	// It's important to not miss a single event, so have a small buffer
	events := make(chan notify.EventInfo, 10)
	if err := notify.Watch(u2fKeysPath, events, notify.InOpen); err != nil {
		log.Errorf("Cannot establish a watch on u2f_keys file '%v': %v\n", u2fKeysPath, err)
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
