package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/maximbaz/yubikey-touch-detector/detector"
	"github.com/maximbaz/yubikey-touch-detector/notifier"
	log "github.com/sirupsen/logrus"
)

const appVersion = "1.4.0"

func main() {
	truthyValues := map[string]bool{"true": true, "yes": true, "1": true}
	defaultU2fAuthPendingPath := "/var/run/user/1000/pam-u2f-authpending"
	defaultGpgPubringPath := "$GNUPGHOME/pubring.kbx or $HOME/.gnupg/pubring.kbx"

	envVerbose := truthyValues[strings.ToLower(os.Getenv("YUBIKEY_TOUCH_DETECTOR_VERBOSE"))]
	envLibnotify := truthyValues[strings.ToLower(os.Getenv("YUBIKEY_TOUCH_DETECTOR_LIBNOTIFY"))]
	envU2fAuthPendingPath := os.Getenv("YUBIKEY_TOUCH_DETECTOR_U2F_AUTHPENDING_PATH")
	envGpgPubringPath := os.Getenv("YUBIKEY_TOUCH_DETECTOR_GPG_PUBRING_PATH")

	var version bool
	var verbose bool
	var libnotify bool
	var u2fAuthPendingPath string
	var gpgPubringPath string

	flag.BoolVar(&version, "version", false, "print version and exit")
	flag.BoolVar(&verbose, "v", envVerbose, "print verbose output")
	flag.BoolVar(&libnotify, "libnotify", envLibnotify, "show desktop notifications using libnotify")
	flag.StringVar(&u2fAuthPendingPath, "u2f-authpending-path", envU2fAuthPendingPath, "path to pam-u2f-authpending file")
	flag.StringVar(&gpgPubringPath, "gpg-pubring-path", envGpgPubringPath, "path to gpg's pubring.kbx file")
	flag.Parse()

	if u2fAuthPendingPath == "" {
		u2fAuthPendingPath = defaultU2fAuthPendingPath
	}

	if gpgPubringPath == "" {
		gpgPubringPath = defaultGpgPubringPath
	}

	if version {
		fmt.Println("YubiKey touch detector version:", appVersion)
		os.Exit(0)
	}

	if verbose {
		log.SetLevel(log.DebugLevel)
	}

	if gpgPubringPath == defaultGpgPubringPath {
		gpgHome := os.Getenv("GNUPGHOME")
		if gpgHome != "" {
			gpgPubringPath = path.Join(gpgHome, "pubring.kbx")
		} else {
			gpgPubringPath = "$HOME/.gnupg/pubring.kbx"
		}
	}

	u2fAuthPendingPath = os.ExpandEnv(u2fAuthPendingPath)
	gpgPubringPath = os.ExpandEnv(gpgPubringPath)

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.Debug("Starting YubiKey touch detector")

	exits := make(map[string]chan bool)
	go setupExitSignalWatch(exits)

	notifiers := make(map[string]chan notifier.Message)
	go notifier.SetupStdErrNotifier(notifiers)
	go notifier.SetupUnixSocketNotifier(notifiers, exits)
	if libnotify {
		go notifier.SetupLibnotifyNotifier(notifiers)
	}

	requestGPGCheck := make(chan bool)
	go detector.CheckGPGOnRequest(requestGPGCheck, notifiers)

	go detector.WatchU2F(u2fAuthPendingPath, notifiers)
	go detector.WatchGPG(gpgPubringPath, requestGPGCheck)
	go detector.WatchSSH(requestGPGCheck, exits)

	wait := make(chan bool)
	<-wait
}

func setupExitSignalWatch(exits map[string]chan bool) {
	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, os.Interrupt, syscall.SIGTERM)

	<-exitSignal
	println()

	for _, exit := range exits {
		exit <- true // Notify exit watcher
		<-exit       // Wait for confirmation
	}

	log.Debug("Stopping YubiKey touch detector")
	os.Exit(0)
}
