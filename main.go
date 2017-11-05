package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/maximbaz/yubikey-touch-detector/detector"
	"github.com/maximbaz/yubikey-touch-detector/notifier"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.DebugLevel)
	log.Info("Starting Yubikey touch detector")

	exits := make(map[string]chan bool)
	go setupExitSignalWatch(exits)

	notifiers := make(map[string]chan notifier.Message)
	go notifier.SetupStdOutNotifier(notifiers)
	go notifier.SetupUnixSocketNotifier(notifiers, exits)

	requestGPGCheck := make(chan bool)
	go detector.CheckGPGOnRequest(requestGPGCheck, notifiers)

	go detector.WatchU2F(notifiers)
	go detector.WatchGPG(requestGPGCheck)
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

	log.Info("Stopping Yubikey touch detector")
	os.Exit(0)
}
