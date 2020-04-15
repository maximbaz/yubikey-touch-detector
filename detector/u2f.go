package detector

import (
	"io/ioutil"
	"path"
	"sync"
	"time"

	"github.com/maximbaz/yubikey-touch-detector/notifier"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

// WatchU2F watches when YubiKey is waiting for a touch on a U2F request
func WatchU2F(notifiers *sync.Map) {
	runWatcher := func(events chan notify.EventInfo) {
		for {
			select {
			case event := <-events:
				switch event.Event() {
				case notify.InOpen:
					notifiers.Range(func(k, v interface{}) bool {
						v.(chan notifier.Message) <- notifier.U2F_ON
						return true
					})

				case notify.InCloseNowrite:
				case notify.InCloseWrite:
					notifiers.Range(func(k, v interface{}) bool {
						v.(chan notifier.Message) <- notifier.U2F_OFF
						return true
					})

				default:
					// Device got removed, unsubscribe and exit
					notify.Stop(events)
					return
				}
			}
		}
	}

	checkAndInitWatcher := func(devicePath string) {
		if isYubikeyHidrawDevice(devicePath) {
			go runWatcher(initInotifyWatcher("U2F", devicePath, notify.InOpen, notify.InCloseNowrite, notify.InCloseWrite, notify.InDeleteSelf))
		}
	}

	devicesEvents := initInotifyWatcher("U2F", "/dev", notify.Create)
	defer notify.Stop(devicesEvents)

	if devices, err := ioutil.ReadDir("/dev"); err == nil {
		for _, device := range devices {
			checkAndInitWatcher(path.Join("/dev", device.Name()))
		}
	} else {
		log.Errorf("Cannot list devices in '/dev' to find connected YubiKeys: %v", err)
	}

	for {
		select {
		case event := <-devicesEvents:
			// Give a second for device to initialize before establishing a watcher
			time.Sleep(1 * time.Second)
			checkAndInitWatcher(event.Path())
		}
	}
}
