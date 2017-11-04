package detector

import (
	"os"
	"os/exec"
	"time"

	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

// WatchGPG watches for hints that YubiKey is maybe waiting for a touch on a GPG request
func WatchGPG(requestGPGCheck chan bool) {
	// No need for a buffered channel,
	// we are interested only in the first event, it's ok to skip all subsequent ones
	events := make(chan notify.EventInfo)
	file := os.ExpandEnv("$HOME/.gnupg/pubring.kbx")
	if err := notify.Watch(file, events, notify.InOpen); err != nil {
		log.Error("Cannot establish a watch on GPG file: ", err)
		return
	}
	defer notify.Stop(events)

	for {
		select {
		case <-events:
			select {
			case requestGPGCheck <- true:
			default:
			}
		}
	}
}

// CheckGPGOnRequest checks whether Yubikey is actually waiting for a touch on a GPG request
func CheckGPGOnRequest(requestGPGCheck chan bool) {
	for {
		<-requestGPGCheck

		for i := 0; i < 20; i++ {
			cmd := checkGPGCardStatus()
			timer := time.AfterFunc(100*time.Millisecond, func() {
				cmd.Process.Kill()
			})
			err := cmd.Wait()
			timer.Stop()

			if err != nil {
				log.Debug("GPG START")
				checkGPGCardStatus().Wait()
				log.Debug("GPG STOP")
				break
			}
		}
	}
}

func checkGPGCardStatus() *exec.Cmd {
	cmd := exec.Command("gpg", "--no-tty", "--card-status")
	if err := cmd.Start(); err != nil {
		log.Error(err)
	}
	return cmd
}
