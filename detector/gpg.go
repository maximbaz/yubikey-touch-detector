package detector

import (
	"sync"
	"time"

	"github.com/proglottis/gpgme"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"

	"github.com/maximbaz/yubikey-touch-detector/notifier"
)

// WatchGPG watches for hints that YubiKey is maybe waiting for a touch on a GPG request
func WatchGPG(gpgPubringPath string, requestGPGCheck chan bool) {
	// No need for a buffered channel,
	// we are interested only in the first event, it's ok to skip all subsequent ones
	events := make(chan notify.EventInfo)

	initWatcher := func() {
		if err := notify.Watch(gpgPubringPath, events, notify.InOpen, notify.InDeleteSelf, notify.InMoveSelf); err != nil {
			log.Errorf("Cannot establish a watch on gpg's pubring.kbx file '%v': %v", gpgPubringPath, err)
			return
		}
		log.Debug("GPG watcher is successfully established")
	}

	initWatcher()
	defer notify.Stop(events)

	for event := range events {
		switch event.Event() {
		case notify.InOpen:
			select {
			case requestGPGCheck <- true:
			default:
			}
		default:
			log.Debugf("pubring.kbx received file event '%+v', recreating the watcher.", event.Event())
			notify.Stop(events)
			time.Sleep(5 * time.Second)
			initWatcher()
		}
	}
}

// CheckGPGOnRequest checks whether YubiKey is actually waiting for a touch on a GPG request
func CheckGPGOnRequest(requestGPGCheck chan bool, notifiers *sync.Map, ctx *gpgme.Context) {
	check := func(response chan error, ctx *gpgme.Context, t *time.Timer) {
		err := ctx.AssuanSend("LEARN", nil, nil, func(status, args string) error {
			log.Debugf("AssuanSend/status: %v, %v", status, args)

			return nil
		})
		if !t.Stop() {
			response <- err
		}
	}
	for range requestGPGCheck {
		resp := make(chan error)

		t := time.AfterFunc(200*time.Millisecond, func() {
			notifiers.Range(func(_, v interface{}) bool {
				v.(chan notifier.Message) <- notifier.GPG_ON
				return true
			})
			err := <-resp
			if err != nil {
				log.Errorf("Agent returned an error: %v", err)
			}
			notifiers.Range(func(_, v interface{}) bool {
				v.(chan notifier.Message) <- notifier.GPG_OFF
				return true
			})
		})

		time.Sleep(100 * time.Millisecond) // wait for GPG to start talking with scdaemon
		check(resp, ctx, t)
	}
}
