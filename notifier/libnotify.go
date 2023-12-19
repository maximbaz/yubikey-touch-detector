package notifier

import (
	"sync"
	"sync/atomic"

	"github.com/esiqveland/notify"
	"github.com/godbus/dbus/v5"
	log "github.com/sirupsen/logrus"
)

// SetupLibnotifyNotifier configures a notifier to show all touch requests with libnotify
func SetupLibnotifyNotifier(notifiers *sync.Map) {
	touch := make(chan Message, 10)
	notifiers.Store("notifier/libnotify", touch)

	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		log.Error("Cannot initialize desktop notifications, unable to create session bus: ", err)
		return
	}
	defer conn.Close()

	if err := conn.Auth(nil); err != nil {
		log.Error("Cannot initialize desktop notifications, unable to authenticate: ", err)
		return
	}

	if err := conn.Hello(); err != nil {
		log.Error("Cannot initialize desktop notifications, unable get bus name: ", err)
		return
	}

	notification := notify.Notification{
		AppName: "yubikey-touch-detector",
		AppIcon: "yubikey-touch-detector",
		Summary: "YubiKey is waiting for a touch",
	}

	reset := func(msg *notify.NotificationClosedSignal) {
		atomic.CompareAndSwapUint32(&notification.ReplacesID, msg.ID, 0)
	}

	notifier, err := notify.New(
		conn,
		notify.WithOnClosed(reset),
		notify.WithLogger(log.StandardLogger()),
	)
	if err != nil {
		log.Error("Cannot initialize desktop notifications, unable to initialize D-Bus notifier interface: ", err)
		return
	}
	defer notifier.Close()

	activeTouchWaits := 0

	for {
		value := <-touch
		if value == GPG_ON || value == U2F_ON || value == HMAC_ON {
			activeTouchWaits++
		}
		if value == GPG_OFF || value == U2F_OFF || value == HMAC_OFF {
			activeTouchWaits--
		}
		if activeTouchWaits > 0 {
			id, err := notifier.SendNotification(notification)
			if err != nil {
				log.Error("Cannot show notification: ", err)
				continue
			}

			atomic.CompareAndSwapUint32(&notification.ReplacesID, 0, id)
		} else if id := atomic.LoadUint32(&notification.ReplacesID); id != 0 {
			if _, err := notifier.CloseNotification(id); err != nil {
				log.Error("Cannot close notification: ", err)
				continue
			}
		}
	}
}
