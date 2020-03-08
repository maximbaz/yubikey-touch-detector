package detector

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"time"

	"github.com/maximbaz/yubikey-touch-detector/notifier"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

// WatchU2F watches when YubiKey is waiting for a touch on a U2F request
func WatchU2F(notifiers map[string]chan notifier.Message) {
	initWatcher := func(path string, eventTypes ...notify.Event) chan notify.EventInfo {
		events := make(chan notify.EventInfo, 10)
		if err := notify.Watch(path, events, eventTypes...); err != nil {
			log.Errorf("Cannot establish a watch on '%v': %v", path, err)
			return nil
		}
		log.Debugf("U2F watcher on '%v' is successfully established", path)
		return events
	}

	runWatcher := func(events chan notify.EventInfo) {
		for {
			select {
			case event := <-events:
				switch event.Event() {
				case notify.InOpen:
					for _, n := range notifiers {
						n <- notifier.U2F_ON
					}

				case notify.InCloseNowrite:
				case notify.InCloseWrite:
					for _, n := range notifiers {
						n <- notifier.U2F_OFF
					}

				default:
					// Device got removed, unsubscribe and exit
					notify.Stop(events)
					return
				}
			}
		}
	}

	checkAndInitWatcher := func(devicePath string) {
		if strings.Contains(devicePath, "hidraw") {
			// Give a second for device to initialize before establishing a watcher
			time.Sleep(1 * time.Second)

			if info, err := ioutil.ReadFile(fmt.Sprintf("/sys/class/hidraw/%v/device/uevent", path.Base(devicePath))); err == nil {
				if strings.Contains(string(info), "YubiKey") {
					watcher := initWatcher(devicePath, notify.InOpen, notify.InCloseNowrite, notify.InCloseWrite, notify.InDeleteSelf)
					go runWatcher(watcher)
				}
			}
		}
	}

	devicesEvents := initWatcher("/dev", notify.Create)
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
			checkAndInitWatcher(event.Path())
		}
	}
}
