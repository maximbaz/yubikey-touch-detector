package notifier

import (
	"os"
	"sync"
)

// SetupStdoutNotifier configures a notifier to log to stdout
func SetupStdoutNotifier(notifiers *sync.Map) {
	touch := make(chan Message, 10)
	notifiers.Store("notifier/stdout", touch)

	for {
		value := <-touch
		os.Stdout.WriteString(string(value))
		os.Stdout.WriteString("\n")
	}
}
