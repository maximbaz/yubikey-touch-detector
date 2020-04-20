package detector

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/maximbaz/yubikey-touch-detector/notifier"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
	"github.com/vtolstov/go-ioctl"
)

// WatchU2F watches when YubiKey is waiting for a touch on a U2F request
func WatchU2F(notifiers *sync.Map) {
	checkAndInitWatcher := func(devicePath string) {
		if isFidoU2FDevice(devicePath) {
			go runU2FWatcher(devicePath, notifiers)
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

type hidrawDescriptor struct {
	Size  uint32
	Value [4096]uint8
}

func isFidoU2FDevice(devicePath string) bool {
	if !strings.HasPrefix(devicePath, "/dev/hidraw") {
		return false
	}

	device, err := os.Open(devicePath)
	if err != nil {
		return false
	}
	defer device.Close()

	var size uint32
	err = ioctl.IOCTL(device.Fd(), ioctl.IOR('H', 1, 4), uintptr(unsafe.Pointer(&size)))
	if err != nil {
		log.Warnf("Cannot get descriptor size for device '%v': %v", devicePath, err)
		return false
	}

	data := hidrawDescriptor{Size: size}
	err = ioctl.IOCTL(device.Fd(), ioctl.IOR('H', 2, unsafe.Sizeof(data)), uintptr(unsafe.Pointer(&data)))
	if err != nil {
		log.Warnf("Cannot get descriptor for device '%v': %v", devicePath, err)
		return false
	}

	isFido := false
	hasU2F := false
	for i := uint32(0); i < size; {
		prefix := data.Value[i]
		tag := (prefix & 0b11110000) >> 4
		typ := (prefix & 0b00001100) >> 2
		size := prefix & 0b00000011

		if typ == 1 && tag == 0 && data.Value[i+1] == 0xd0 && data.Value[i+2] == 0xf1 {
			isFido = true
		} else if typ == 2 && tag == 0 && data.Value[i+1] == 0x01 {
			hasU2F = true
		}

		if isFido && hasU2F {
			return true
		}

		i += uint32(size) + 1
	}

	return false
}

func runU2FWatcher(devicePath string, notifiers *sync.Map) {
	device, err := os.Open(devicePath)
	if err != nil {
		log.Errorf("Cannot open device '%v' to run U2F watcher: %v", devicePath, err)
		return
	}
	defer device.Close()

	payload := make([]byte, 64)
	lastMessage := notifier.U2F_OFF
	for {
		_, err = device.Read(payload)
		if err != nil {
			notifiers.Range(func(k, v interface{}) bool {
				v.(chan notifier.Message) <- notifier.U2F_OFF
				return true
			})
			return
		}

		isU2F := payload[4] == 0x83 && payload[7] == 0x69 && payload[8] == 0x85
		isFIDO2 := payload[4] == 0xbb && payload[7] == 0x02

		newMessage := notifier.U2F_OFF
		if isU2F || isFIDO2 {
			newMessage = notifier.U2F_ON
		}

		if newMessage != lastMessage {
			notifiers.Range(func(k, v interface{}) bool {
				v.(chan notifier.Message) <- newMessage
				return true
			})
		}

		lastMessage = newMessage
	}
}
