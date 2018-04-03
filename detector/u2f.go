package detector

import (
	"time"

	"github.com/maximbaz/yubikey-touch-detector/notifier"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

// WatchU2F watches when YubiKey is waiting for a touch on a U2F request
func WatchU2F(u2fKeysPath string, notifiers map[string]chan notifier.Message) {
	// It's important to not miss a single event, so have a small buffer
	events := make(chan notify.EventInfo, 10)
	isOn := false

	initWatcher := func() {
		isOn = false
		if err := notify.Watch(u2fKeysPath, events, notify.InOpen, notify.InDeleteSelf, notify.InMoveSelf); err != nil {
			log.Errorf("Cannot establish a watch on u2f_keys file '%v': %v\n", u2fKeysPath, err)
			return
		}
		log.Debug("U2F watcher is successfully established")
	}

	initWatcher()
	defer notify.Stop(events)

	for {
		select {
		case event := <-events:
			switch event.Event() {
			case notify.InOpen:
				isOn = !isOn
				for _, n := range notifiers {
					if isOn {
						n <- notifier.U2F_ON
					} else {
						n <- notifier.U2F_OFF
					}
				}
			default:
				log.Debugf("u2f_keys received file event '%+v', recreating the watcher.\n", event.Event())
				notify.Stop(events)
				if isOn {
					for _, n := range notifiers {
						n <- notifier.U2F_OFF
					}
				}
				time.Sleep(5 * time.Second)
				initWatcher()
			}
		}
	}
}
