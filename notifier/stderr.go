package notifier

import log "github.com/sirupsen/logrus"

// SetupStdErrNotifier configures a notifier to print all touch requests to STDOUT
func SetupStdErrNotifier(notifiers map[string]chan Message) {
	touch := make(chan Message, 10)
	notifiers["notifier/stdout"] = touch

	for {
		value := <-touch
		log.Debug("[notifiers/stdout] ", value)
	}
}
