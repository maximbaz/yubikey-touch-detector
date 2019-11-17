# YubiKey touch detector

This is a tool that can detect when YubiKey is waiting for your touch. It is designed to be integrated with other UI components to display a visible indicator.

For example, an integration with [i3wm](https://i3wm.org/) and [py3status](https://github.com/ultrabug/py3status) looks like this:

![demo](https://user-images.githubusercontent.com/1177900/46533233-2bcf5580-c8a4-11e8-99e7-1418e89615f5.gif)

_See also: [FAQ: Which UI components are already integrated with this app?](#faq-existing-ui-integrations)_

## Installation

**This tool only works on Linux**. If you want to help implementing (at least partial) support for other OS, pull requests are very welcome!

On Arch Linux, you can install it with `pacman -S yubikey-touch-detector`

The package also installs a systemd service, make sure to start and enable it:

```
$ systemctl --user daemon-reload
$ systemctl --user enable --now yubikey-touch-detector.service
```

Alternatively you can download the latest release from the [GitHub releases](https://github.com/maximbaz/yubikey-touch-detector/releases) page. All releases are signed with [my PGP key](https://keybase.io/maximbaz).

Finally you can install the app with `go`:

```
$ go get -u github.com/maximbaz/yubikey-touch-detector
```

This places the binary in your `$GOPATH/bin` folder, as well as the sources in `$GOPATH/src` for you to use the detection functions in your own code.

## Usage

#### Command line

To test how the app works, run it in verbose mode to print every event on STDERR:

```
$ yubikey-touch-detector -v
```

Now try different commands that require a physical touch and see if the app can successfully detect them.

#### Desktop notifications

You can make the app show desktop notifications using `libnotify` if you run it with corresponding flag:

```
$ yubikey-touch-detector --libnotify
```

#### Integrating with other UI components

First of all, make sure the app is always running (e.g. start a provided systemd user service).

Next, in order to integrate the app with other UI components to display a visible indicator, use any of the available notifiers in the `notifier` subpackage.

##### notifier/unix_socket

`unix_socket` notifier allows anyone to connect to the socket `$XDG_RUNTIME_DIR/yubikey-touch-detector.socket` and receive the following events:

| event   | description                                        |
| ------- | -------------------------------------------------- |
| `GPG_1` | when a `gpg` operation started waiting for a touch |
| `GPG_0` | when a `gpg` operation stopped waiting for a touch |
| `U2F_1` | when `pam-u2f` started waiting for a touch         |
| `U2F_0` | when `pam-u2f` stopped waiting for a touch         |

All messages have a fixed length of 5 bytes to simplify the code on the receiving side.

## How it works

Your YubiKey may require a physical touch to confirm these operations:

- `sudo` request (via `pam-u2f`)
- `gpg --sign`
- `gpg --decrypt`
- `ssh` to a remote host (and related operations, such as `scp`, `rsync`, etc.)
- `ssh` on a remote host to a different remote host (via forwarded `ssh-agent`)

_See also: [FAQ: How do I configure my YubiKey to require a physical touch?](#faq-configure-yubikey-require-touch)_

#### Detecting a sudo request (via `pam-u2f`)

In order to detect when `pam-u2f` requests a touch on YubiKey, make sure you use `pam-u2f` of at least `v1.0.7`.

With that in place, `pam-u2f` will open `/var/run/$UID/pam-u2f-authpending` when it starts waiting for a user to touch the device, and close it when it stops waiting for a touch.

> If the path to your authpending file differs, provide it via `--u2f-auth-pending-path` CLI argument.

This app will thus watch for `OPEN` events on that file, and when event occurs will toggle the touch indicator.

### Detecting gpg operations

This detection is based on a "busy check" - when the card is busy (i.e. `gpg --card-status` hangs), it is assumed that it is waiting on a touch. This of course leads to false positives, when the card is busy for other reasons, but it is a good guess anyway.

In order to not run the `gpg --card-status` indefinitely (which leads to YubiKey be constantly blinking), the check is being performed only after `$GNUPGHOME/pubring.kbx` (or `$HOME/.gnupg/pubring.kbx`) file is opened (the app is thus watching for `OPEN` events on that file).

> If the path to your `pubring.kbx` file differs, provide it via `--gpg-pubring-path` CLI argument.

### Detecting ssh operations

The requests performed on a local host will be captured by the `gpg` detector. However, in order to detect the use of forwarded `ssh-agent` on a remote host, an additional detector was introduced.

This detector runs as a proxy on the `$SSH_AUTH_SOCK`, it listens to all communications with that socket and starts a `gpg --card-status` check in case an event was captured.

## FAQ

<a name="faq-configure-yubikey-require-touch"></a>

#### How do I configure my YubiKey to require a physical touch?

For `sudo` requests with `pam-u2f`, please refer to the documentation on [Yubico/pam-u2f](https://github.com/Yubico/pam-u2f) and online guides.

For `gpg` and `ssh` operations, install [ykman](https://github.com/Yubico/yubikey-manager) and use the following commands:

```
$ ykman openpgp touch sig on   # For sign operations
$ ykman openpgp touch enc on   # For decrypt operations
$ ykman openpgp touch aut on   # For ssh operations
```

Make sure to unplug and plug back in your YubiKey after changing any of the options above.

<a name="faq-existing-ui-integrations"></a>

#### Which UI components are already integrated with this app?

- [py3status](https://github.com/ultrabug/py3status) provides an indicator for [i3wm](https://i3wm.org/) via [yubikey](https://github.com/ultrabug/py3status/blob/master/py3status/modules/yubikey.py) module.
- [barista](https://github.com/soumya92/barista) provides an indicator for [i3wm](https://i3wm.org/) via [yubikey](https://github.com/soumya92/barista/blob/master/samples/yubikey/yubikey.go) module.
