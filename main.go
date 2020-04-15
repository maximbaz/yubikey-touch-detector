package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"

	"github.com/maximbaz/yubikey-touch-detector/detector"
	"github.com/maximbaz/yubikey-touch-detector/notifier"
	log "github.com/sirupsen/logrus"
)

const appVersion = "1.5.2"

func main() {
	truthyValues := map[string]bool{"true": true, "yes": true, "1": true}
	defaultGpgPubringPath := "$GNUPGHOME/pubring.kbx or $HOME/.gnupg/pubring.kbx"

	envVerbose := truthyValues[strings.ToLower(os.Getenv("YUBIKEY_TOUCH_DETECTOR_VERBOSE"))]
	envLibnotify := truthyValues[strings.ToLower(os.Getenv("YUBIKEY_TOUCH_DETECTOR_LIBNOTIFY"))]
	envGpgPubringPath := os.Getenv("YUBIKEY_TOUCH_DETECTOR_GPG_PUBRING_PATH")

	var version bool
	var verbose bool
	var libnotify bool
	var gpgPubringPath string

	flag.BoolVar(&version, "version", false, "print version and exit")
	flag.BoolVar(&verbose, "v", envVerbose, "print verbose output")
	flag.BoolVar(&libnotify, "libnotify", envLibnotify, "show desktop notifications using libnotify")
	flag.StringVar(&gpgPubringPath, "gpg-pubring-path", envGpgPubringPath, "path to gpg's pubring.kbx file")
	flag.Parse()

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

	gpgPubringPath = os.ExpandEnv(gpgPubringPath)

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.Debug("Starting YubiKey touch detector")

	exits := &sync.Map{}
	go setupExitSignalWatch(exits)

	notifiers := &sync.Map{}
	go notifier.SetupStdErrNotifier(notifiers)
	go notifier.SetupUnixSocketNotifier(notifiers, exits)
	if libnotify {
		go notifier.SetupLibnotifyNotifier(notifiers)
	}

	requestGPGCheck := make(chan bool)
	go detector.CheckGPGOnRequest(requestGPGCheck, notifiers, gpgPubringPath)

	go detector.WatchU2F(notifiers)
	go detector.WatchGPG(gpgPubringPath, requestGPGCheck)
	go detector.WatchSSH(requestGPGCheck, exits)

	wait := make(chan bool)
	<-wait
}

func setupExitSignalWatch(exits *sync.Map) {
	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, os.Interrupt, syscall.SIGTERM)

	<-exitSignal
	println()

	exits.Range(func(k, v interface{}) bool {
		exit := v.(chan bool)
		exit <- true // Notify exit watcher
		<-exit       // Wait for confirmation
		return true
	})

	log.Debug("Stopping YubiKey touch detector")
	os.Exit(0)
}
