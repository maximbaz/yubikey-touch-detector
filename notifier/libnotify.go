package notifier

import (
	"sync"

	libnotify "github.com/menefotto/go-libnotify"
	log "github.com/sirupsen/logrus"
)

// SetupLibnotifyNotifier configures a notifier to show all touch requests with libnotify
func SetupLibnotifyNotifier(notifiers *sync.Map) {
	touch := make(chan Message, 10)
	notifiers.Store("notifier/libnotify", touch)

	if !libnotify.Init("yubikey-touch-detector") {
		log.Error("Cannot initialize desktop notifications!")
		return
	}
	defer libnotify.UnInit()

	notification := libnotify.NotificationNew("YubiKey is waiting for a touch", "", "")
	if notification == nil {
		log.Error("Cannot create desktop notification!")
		return
	}

	activateTouchWaits := 0

	for {
		value := <-touch
		if value == GPG_ON || value == U2F_ON || value == HMAC_ON {
			activateTouchWaits++
		}
		if value == GPG_OFF || value == U2F_OFF || value == HMAC_OFF {
			activateTouchWaits--
		}
		if activateTouchWaits > 0 {
			// Error check (!= nil) not possible because menefotto/go-libnotify
			// uses a custom wrapper instead of builtin 'error'
			if err := notification.Show(); err.Error() != "" {
				log.Error("Cannot show notification: ", err.Error())
			}
		} else {
			// Error check (!= nil) not possible because menefotto/go-libnotify
			// uses a custom wrapper instead of builtin 'error'
			if err := notification.Close(); err.Error() != "" {
				log.Error("Cannot close notification: ", err.Error())
			}
		}
	}
}
