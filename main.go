package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
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
		fmt.Println("YubiKey touch detector version:", appVersion())
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
	initGPGBasedDetectors(notifiers, exits)

	wait := make(chan bool)
	<-wait
}

func initGPGBasedDetectors(notifiers, exits *sync.Map) {
	ctx, err := gpgme.New()
	if err != nil {
		log.Debugf("Cannot initialize GPG context: %v. Disabling GPG and SSH watchers.", err)
		return
	}

	if ctx.SetProtocol(gpgme.ProtocolAssuan) != nil {
		log.Debugf("Cannot initialize Assuan IPC: %v. Disabling GPG and SSH watchers.", err)
		return
	}

	var gpgPrivateKeysDirPath = path.Join(gpgme.GetDirInfo("homedir"), "private-keys-v1.d")
	if _, err := os.Stat(gpgPrivateKeysDirPath); err != nil {
		log.Debugf("Directory '%s' does not exist or cannot stat it\n", gpgPrivateKeysDirPath)
		return
	}

	filesToWatch, err := findShadowedPrivateKeys(gpgPrivateKeysDirPath)
	if err != nil {
		log.Debugf("Error finding shadowed private keys: %v\n", err)
		return
	}

	if len(filesToWatch) == 0 {
		log.Debugf("No shadowed private keys found.\n")
		return
	}

	requestGPGCheck := make(chan bool)
	go detector.CheckGPGOnRequest(requestGPGCheck, notifiers, ctx)
	go detector.WatchGPG(filesToWatch, requestGPGCheck)
	go detector.WatchSSH(requestGPGCheck, exits)
}

func findShadowedPrivateKeys(folderPath string) ([]string, error) {
	var result []string
	err := filepath.WalkDir(folderPath, func(path string, info os.DirEntry, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "shadowed-private-key") {
			result = append(result, path)
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

func appVersion() string {
	version := "$Format:%(describe)$"
	if strings.HasPrefix(version, "$") {
		out, err := exec.Command("git", "describe", "--tags").Output()
		if err != nil {
			panic(fmt.Sprintf("Failed to determine version using 'git describe': %v", err))
		}
		version = strings.TrimSpace(string(out))
	}
	return version
}
