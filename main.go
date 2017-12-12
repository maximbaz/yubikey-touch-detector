package main

import (
	"flag"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/maximbaz/yubikey-touch-detector/detector"
	"github.com/maximbaz/yubikey-touch-detector/notifier"
	log "github.com/sirupsen/logrus"
)

func main() {
	defaultGpgPubringPath := "$GNUPGHOME/pubring.kbx or $HOME/.gnupg/pubring.kbx"

	var verbose bool
	var u2fKeysPath string
	var gpgPubringPath string
	flag.BoolVar(&verbose, "v", false, "print verbose output")
	flag.StringVar(&u2fKeysPath, "u2f-keys-path", "$HOME/.config/Yubico/u2f_keys", "path to u2f_keys file")
	flag.StringVar(&gpgPubringPath, "gpg-pubring-path", defaultGpgPubringPath, "path to gpg's pubring.kbx file")
	flag.Parse()

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

	u2fKeysPath = os.ExpandEnv(u2fKeysPath)
	gpgPubringPath = os.ExpandEnv(gpgPubringPath)

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.Debug("Starting Yubikey touch detector")

	exits := make(map[string]chan bool)
	go setupExitSignalWatch(exits)

	notifiers := make(map[string]chan notifier.Message)
	go notifier.SetupStdErrNotifier(notifiers)
	go notifier.SetupUnixSocketNotifier(notifiers, exits)

	requestGPGCheck := make(chan bool)
	go detector.CheckGPGOnRequest(requestGPGCheck, notifiers)

	go detector.WatchU2F(u2fKeysPath, notifiers)
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

	log.Debug("Stopping Yubikey touch detector")
	os.Exit(0)
}
