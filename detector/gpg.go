package detector

import (
	"os/exec"
	"sync"
	"time"

	"github.com/maximbaz/yubikey-touch-detector/notifier"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
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

	for {
		select {
		case event := <-events:
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
}

// CheckGPGOnRequest checks whether YubiKey is actually waiting for a touch on a GPG request
func CheckGPGOnRequest(requestGPGCheck chan bool, notifiers *sync.Map) {
	for {
		<-requestGPGCheck

		for i := 0; i < 20; i++ {
			if isGPGCardBusy() {
				notifiers.Range(func(k, v interface{}) bool {
					v.(chan notifier.Message) <- notifier.GPG_ON
					return true
				})

				for isGPGCardBusy() || isGPGCardBusy() {
					// wait...
				}

				notifiers.Range(func(k, v interface{}) bool {
					v.(chan notifier.Message) <- notifier.GPG_OFF
					return true
				})
				break
			}
		}
	}
}

func isGPGCardBusy() bool {
	cmd := exec.Command("gpg", "--lock-never", "--card-status")
	if err := cmd.Start(); err != nil {
		log.Error(err)
		return false
	}

	timer := time.AfterFunc(1000*time.Millisecond, func() {
		cmd.Process.Kill()
	})

	cmd.Wait()
	timer.Stop()

	return cmd.ProcessState.ExitCode() == -1
}
