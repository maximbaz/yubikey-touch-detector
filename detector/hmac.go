package detector

import (
	"io/ioutil"
	"path"
	"sync"
	"time"

	"github.com/maximbaz/yubikey-touch-detector/notifier"
	"github.com/rjeczalik/notify"
	"github.com/scylladb/go-set/strset"
	log "github.com/sirupsen/logrus"
)

// WatchHMAC watches when YubiKey is waiting for a touch on a HMAC request
func WatchHMAC(notifiers *sync.Map) {
	devicesEvents := initInotifyWatcher("HMAC", "/dev", notify.Create, notify.Remove)
	defer notify.Stop(devicesEvents)

	yubikeyHidrawDevices := strset.New()
	if devices, err := ioutil.ReadDir("/dev"); err == nil {
		for _, device := range devices {
			devicePath := path.Join("/dev", device.Name())
			if isYubikeyHidrawDevice(devicePath) {
				yubikeyHidrawDevices.Add(devicePath)
			}
		}
	} else {
		log.Errorf("Cannot list devices in '/dev' to find connected YubiKeys: %v", err)
	}

	for {
		select {
		case event := <-devicesEvents:
			switch event.Event() {
			case notify.Create:
				// Give a second for device to initialize
				time.Sleep(1 * time.Second)

				if isYubikeyHidrawDevice(event.Path()) {
					yubikeyHidrawDevices.Add(event.Path())
					notifiers.Range(func(k, v interface{}) bool {
						v.(chan notifier.Message) <- notifier.HMAC_OFF
						return true
					})
				}
			case notify.Remove:
				if yubikeyHidrawDevices.Has(event.Path()) {
					yubikeyHidrawDevices.Remove(event.Path())

					if yubikeyHidrawDevices.Size() > 0 {
						notifiers.Range(func(k, v interface{}) bool {
							v.(chan notifier.Message) <- notifier.HMAC_ON
							return true
						})
					} else {
						notifiers.Range(func(k, v interface{}) bool {
							v.(chan notifier.Message) <- notifier.HMAC_OFF
							return true
						})
					}
				}
			}
		}
	}
}
