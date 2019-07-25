package notifier

import (
	"os/exec"

	log "github.com/sirupsen/logrus"
)

func notifySend(text string) error {
	return exec.Command("notify-send", text).Run()
}

// SetupLibnotifyNotifier configures a notifier to show all touch requests with libnotify
func SetupLibnotifyNotifier(notifiers map[string]chan Message) {
	touch := make(chan Message, 10)
	notifiers["notifier/libnotify"] = touch

	for {
		value := <-touch
		if value == GPG_ON || value == U2F_ON {
			err := notifySend("Yubikey is waiting for a touch")
			if err != nil {
				log.Error("Cannot send desktop notification: ", err)
			}
		}
	}
}
