package notifier

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

// SetupStdErrNotifier configures a notifier to print all touch requests to STDERR
func SetupStdErrNotifier(notifiers *sync.Map) {
	touch := make(chan Message, 10)
	notifiers.Store("notifier/stdout", touch)

	for {
		value := <-touch
		log.Debug("[notifiers/stdout] ", value)
	}
}
