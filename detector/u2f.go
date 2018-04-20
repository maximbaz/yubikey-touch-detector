package detector

import (
	"os"
	"time"

	"github.com/maximbaz/yubikey-touch-detector/notifier"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

// WatchU2F watches when YubiKey is waiting for a touch on a U2F request
func WatchU2F(u2fLockPath string, notifiers map[string]chan notifier.Message) {
	// It's important to not miss a single event, so have a small buffer
	events := make(chan notify.EventInfo, 10)
	openCounter := 0

	initWatcher := func() {
		// Ensure the file exists (pam-u2f doesn't create it beforehand)
		os.Create(u2fLockPath)

		// Setup the watcher
		openCounter = 0
		if err := notify.Watch(u2fLockPath, events, notify.InOpen, notify.InCloseWrite, notify.InCloseNowrite, notify.InDeleteSelf, notify.InMoveSelf); err != nil {
			log.Errorf("Cannot establish a watch on u2f lock file '%v': %v", u2fLockPath, err)
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
				openCounter++
				if openCounter == 1 {
					for _, n := range notifiers {
						n <- notifier.U2F_ON
					}
				}
			case notify.InCloseWrite:
			case notify.InCloseNowrite:
				if openCounter == 0 {
					log.Debugf("u2f received an unmatched close event, ignoring it.")
					break
				}
				openCounter--
				if openCounter == 0 {
					for _, n := range notifiers {
						n <- notifier.U2F_OFF
					}
				}
			default:
				log.Debugf("u2f received file event '%+v', recreating the watcher.", event.Event())
				notify.Stop(events)
				if openCounter > 0 {
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
