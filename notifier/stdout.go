package notifier

import log "github.com/sirupsen/logrus"

// SetupStdOutNotifier configures a notifier to print all touch requests to STDOUT
func SetupStdOutNotifier(notifiers map[string]chan Message) {
	touch := make(chan Message, 10)
	notifiers["notifier/stdout"] = touch

	for {
		value := <-touch
		log.Info("[notifiers/stdout] ", value)
	}
}
