# YubiKey touch detector

This is a tool that can detect when YubiKey is waiting for your touch. It is designed to be integrated with other UI components to display a visible indicator.

For example, an integration with [i3wm](https://i3wm.org/) and [py3status](https://github.com/ultrabug/py3status) looks like this:

![demo](https://user-images.githubusercontent.com/1177900/46533233-2bcf5580-c8a4-11e8-99e7-1418e89615f5.gif)

_See also: [FAQ: Which UI components are already integrated with this app?](#faq-existing-ui-integrations)_

## Installation

**This tool only works on Linux**. If you want to help implementing (at least partial) support for other OS, pull requests are very welcome!

On Arch Linux, you can install it with `pacman -S yubikey-touch-detector`

The package also installs a systemd service and socket. If you want the app to launch on startup, just enable the service like so:

```
$ systemctl --user daemon-reload
$ systemctl --user enable --now yubikey-touch-detector.service
```

If you want the service to be started only when there is a listener on Unix socket, enable the socket instead like so:

```
$ systemctl --user daemon-reload
$ systemctl --user enable --now yubikey-touch-detector.socket
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

#### Configuring the app

The app supports the following environment variables and CLI arguments (CLI args take precedence):

| Environment var                    | CLI arg       |
| ---------------------------------- | ------------- |
| `YUBIKEY_TOUCH_DETECTOR_VERBOSE`   | `-v`          |
| `YUBIKEY_TOUCH_DETECTOR_LIBNOTIFY` | `--libnotify` |

You can configure the systemd service by defining any of these environment variables in `$XDG_CONFIG_HOME/yubikey-touch-detector/service.conf`, e.g. like so:

```
YUBIKEY_TOUCH_DETECTOR_VERBOSE=true
YUBIKEY_TOUCH_DETECTOR_LIBNOTIFY=true
```

#### Integrating with other UI components

First of all, make sure the app is always running (e.g. start a provided systemd user service or socket).

Next, in order to integrate the app with other UI components to display a visible indicator, use any of the available notifiers in the `notifier` subpackage.

##### notifier/unix_socket

`unix_socket` notifier allows anyone to connect to the socket `$XDG_RUNTIME_DIR/yubikey-touch-detector.socket` and receive the following events:

| event   | description                                        |
| ------- | -------------------------------------------------- |
| `GPG_1` | when a `gpg` operation started waiting for a touch |
| `GPG_0` | when a `gpg` operation stopped waiting for a touch |
| `U2F_1` | when a `u2f` operation started waiting for a touch |
| `U2F_0` | when a `u2f` operation stopped waiting for a touch |

All messages have a fixed length of 5 bytes to simplify the code on the receiving side.

## How it works

Your YubiKey may require a physical touch to confirm these operations:

- `sudo` request (via `pam-u2f`)
- [WebAuthn](https://webauthn.io/)
- `gpg --sign`
- `gpg --decrypt`
- `ssh` to a remote host (and related operations, such as `scp`, `rsync`, etc.)
- `ssh` on a remote host to a different remote host (via forwarded `ssh-agent`)

_See also: [FAQ: How do I configure my YubiKey to require a physical touch?](#faq-configure-yubikey-require-touch)_

### Detecting u2f operations

In order to detect whether a U2F/FIDO2 operation requests a touch on YubiKey, the app is listening on the appropriate `/dev/hidraw*` device for corresponding messages as per FIDO spec.

See `detector/u2f.go` for more info on implementation details, the source code is documented and contains relevant links to the spec.

### Detecting gpg operations

This detection is based on a "busy check" - when the card is busy (i.e. `gpg --card-status` hangs), it is assumed that it is waiting on a touch. This of course leads to false positives, when the card is busy for other reasons, but it is a good guess anyway.

In order to not run the `gpg --card-status` indefinitely (which leads to YubiKey be constantly blinking), the check is being performed only after `$GNUPGHOME/pubring.kbx` (or `$HOME/.gnupg/pubring.kbx`) file is opened (the app is thus watching for `OPEN` events on that file).

> If the path to your `pubring.kbx` file differs, define `$GNUPGHOME` environment variable, globally or in `$XDG_CONFIG_HOME/yubikey-touch-detector/service.conf`.

### Detecting ssh operations

The requests performed on a local host will be captured by the `gpg` detector. However, in order to detect the use of forwarded `ssh-agent` on a remote host, an additional detector was introduced.

This detector runs as a proxy on the `$SSH_AUTH_SOCK`, it listens to all communications with that socket and starts a `gpg --card-status` check in case an event was captured.

## FAQ

<a name="faq-configure-yubikey-require-touch"></a>

#### How do I configure my YubiKey to require a physical touch?

For `sudo` requests with `pam-u2f`, please refer to the documentation on [Yubico/pam-u2f](https://github.com/Yubico/pam-u2f) and online guides (e.g. [official one](https://support.yubico.com/support/solutions/articles/15000011356-ubuntu-linux-login-guide-u2f)).

For `gpg` and `ssh` operations, install [ykman](https://github.com/Yubico/yubikey-manager) and use the following commands:

```
$ ykman openpgp set-touch sig on   # For sign operations
$ ykman openpgp set-touch enc on   # For decrypt operations
$ ykman openpgp set-touch aut on   # For ssh operations
```

If you are going to frequently use OpenPGP operations, `cached` or `cached-fixed` may be better for you. See more details [here](https://github.com/drduh/YubiKey-Guide#require-touch).

Make sure to unplug and plug back in your YubiKey after changing any of the options above.

<a name="faq-existing-ui-integrations"></a>

#### Which UI components are already integrated with this app?

- [waybar](https://github.com/Alexays/Waybar) can display an indicator via custom [yubikey](https://github.com/maximbaz/dotfiles/blob/master/.local/bin/waybar-yubikey) module.
- [py3status](https://github.com/ultrabug/py3status) can display an indicator via [yubikey](https://github.com/ultrabug/py3status/blob/master/py3status/modules/yubikey.py) module.
- [barista](https://github.com/soumya92/barista) can display an indicator via [yubikey](https://github.com/soumya92/barista/blob/master/samples/yubikey/yubikey.go) module.
- [polybar](https://github.com/polybar/polybar) can be made to display an indicator via `polybar-msg`, [like so](https://github.com/maximbaz/yubikey-touch-detector/issues/20#issuecomment-736136283)
- [xfce4-panel](https://docs.xfce.org/xfce/xfce4-panel/start) can display an indicator via [xfce4-yubikey-touch-detector-plugin](https://github.com/invidian/xfce4-yubikey-touch-detector-plugin).
