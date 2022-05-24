package notifier

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

// SetupDebugNotifier configures a notifier to log all touch events
func SetupDebugNotifier(notifiers *sync.Map) {
	touch := make(chan Message, 10)
	notifiers.Store("notifier/debug", touch)

	for {
		value := <-touch
		log.Debug("[notifiers/debug] ", value)
	}
}
