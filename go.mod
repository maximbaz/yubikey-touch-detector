module github.com/maximbaz/yubikey-touch-detector

go 1.18

require (
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/esiqveland/notify v0.11.2
	github.com/godbus/dbus/v5 v5.1.0
	github.com/proglottis/gpgme v0.1.3
	github.com/rjeczalik/notify v0.9.3
	github.com/sirupsen/logrus v1.9.3
	github.com/vtolstov/go-ioctl v0.0.0-20151206205506-6be9cced4810
)

replace github.com/proglottis/gpgme => github.com/mcha-forks/gpgme v0.1.4-0.20230930202035-21b7c62d7377

require (
	github.com/deckarep/golang-set v1.8.0
	golang.org/x/sys v0.16.0 // indirect
)
