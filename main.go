package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/proglottis/gpgme"
	log "github.com/sirupsen/logrus"

	"github.com/maximbaz/yubikey-touch-detector/detector"
	"github.com/maximbaz/yubikey-touch-detector/notifier"
)

const appVersion = "1.11.0"

func main() {
	truthyValues := map[string]bool{"true": true, "yes": true, "1": true}

	envVerbose := truthyValues[strings.ToLower(os.Getenv("YUBIKEY_TOUCH_DETECTOR_VERBOSE"))]
	envLibnotify := truthyValues[strings.ToLower(os.Getenv("YUBIKEY_TOUCH_DETECTOR_LIBNOTIFY"))]
	envStdout := truthyValues[strings.ToLower(os.Getenv("YUBIKEY_TOUCH_DETECTOR_STDOUT"))]
	envNosocket := truthyValues[strings.ToLower(os.Getenv("YUBIKEY_TOUCH_DETECTOR_NOSOCKET"))]

	var version bool
	var verbose bool
	var libnotify bool
	var stdout bool
	var nosocket bool

	flag.BoolVar(&version, "version", false, "print version and exit")
	flag.BoolVar(&verbose, "v", envVerbose, "enable debug logging")
	flag.BoolVar(&libnotify, "libnotify", envLibnotify, "show desktop notifications using libnotify")
	flag.BoolVar(&stdout, "stdout", envStdout, "print notifications to stdout")
	flag.BoolVar(&nosocket, "no-socket", envNosocket, "disable unix socket notifier")
	flag.Parse()

	if version {
		fmt.Println("YubiKey touch detector version:", appVersion)
		os.Exit(0)
	}

	if verbose {
		log.SetLevel(log.DebugLevel)
	}

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.Debug("Starting YubiKey touch detector")

	exits := &sync.Map{}
	go setupExitSignalWatch(exits)

	notifiers := &sync.Map{}

	if verbose {
		go notifier.SetupDebugNotifier(notifiers)
	}
	if !nosocket {
		go notifier.SetupUnixSocketNotifier(notifiers, exits)
	}
	if libnotify {
		go notifier.SetupLibnotifyNotifier(notifiers)
	}
	if stdout {
		go notifier.SetupStdoutNotifier(notifiers)
	}

	go detector.WatchU2F(notifiers)
	go detector.WatchHMAC(notifiers)

	if ctx, err := gpgme.New(); err != nil {
		log.Debugf("Cannot initialize GPG context: %v. Disabling GPG and SSH watchers.", err)
	} else if ctx.SetProtocol(gpgme.ProtocolAssuan) != nil {
		log.Debugf("Cannot initialize Assuan IPC: %v. Disabling GPG and SSH watchers.", err)
	} else {
		var gpgPubringPath = path.Join(gpgme.GetDirInfo("homedir"), "private-keys-v1.d")
		if _, err := os.Stat(gpgPubringPath); os.IsNotExist(err) {
			fmt.Printf("Directory '%s' does not exist (you have no private keys).\n", gpgPubringPath)
			return
		}
		var searchTerm = "shadowed-private-key"
		var filesToWatch []string
		filesToWatch, err := findMatchingFiles(gpgPubringPath, searchTerm)
		if err != nil {
			fmt.Printf("Error finding files: %v\n", err)
			return
		}
		if len(filesToWatch) == 0 {
			fmt.Printf("No files matching the term '%s' found.\n", searchTerm)
			return
		}
		requestGPGCheck := make(chan bool)
		go detector.CheckGPGOnRequest(requestGPGCheck, notifiers, ctx)
		go detector.WatchGPG(filesToWatch, requestGPGCheck)
		go detector.WatchSSH(requestGPGCheck, exits)
	}
	wait := make(chan bool)
	<-wait
}

func findMatchingFiles(folderPath, term string) ([]string, error) {
	var result []string
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, term) {
				result = append(result, path)
				break // No need to scan further lines if we already found the string
			}
		}

		if err := scanner.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func setupExitSignalWatch(exits *sync.Map) {
	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, os.Interrupt, syscall.SIGTERM)

	<-exitSignal
	println()

	exits.Range(func(_, v interface{}) bool {
		exit := v.(chan bool)
		exit <- true // Notify exit watcher
		<-exit       // Wait for confirmation
		return true
	})

	log.Debug("Stopping YubiKey touch detector")
	os.Exit(0)
}
