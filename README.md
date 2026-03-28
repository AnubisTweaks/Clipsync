# ClipSync

<div align="center">

![ClipSync Logo](app.ico)

**Seamless Clipboard Synchronization between iOS and Windows**

[![License](https://img.shields.io/badge/license-MIT-blue.svg)]()
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20iOS-lightgrey)]()
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.19-00ADD8)]()

</div>

---

## 📋 Overview

**ClipSync** is a clipboard synchronization tool that enables seamless text, image, and file sharing between your iOS device and Windows PC over your local network. Copy on one device, paste on another — no cloud, no accounts, no data leaving your network.

### ✨ Features

- 🔄 **Bidirectional Sync** — iOS → Windows and Windows → iOS
- ⚡ **Real-time Detection** — clipboard monitor fires instantly on change
- 📝 **Text** — any size, including large code files
- 🖼️ **Images** — BMP → PNG conversion with base64 transfer
- 📁 **Files** — single and multiple files via Filza integration
- 🔒 **Authentication** — time-based MD5 auth with a custom key you set
- 🔔 **Sound Notifications** — plays a sound on clipboard activity
- 🛡️ **Crash Protection** — global panic recovery with detailed crash logs
- 🔕 **Silent Operation** — runs quietly in the system tray

---

## 🚀 How It Works

ClipSync has two components:

**1. ClipSync.exe (Windows)**
Runs in the background as a system tray app. Provides an HTTP server on port 8086 that the iOS tweak polls to read and write clipboard content.

**2. ClipSync iOS Tweak (Jailbreak Required)**
Monitors clipboard changes on iOS and syncs content bidirectionally via HTTP over your local network.

---

## 📥 Installation

### Windows

1. **Download** `ClipSync.exe` from the [Releases](../../releases) page
2. **Run** the executable — `config.json` is created automatically in the same folder
3. **Allow** through Windows Firewall if prompted. If not prompted, add `ClipSync.exe` to your firewall allow-list manually
4. **Edit** `config.json` and set your `authkey`:

```json
{
  "port": "8086",
  "authkey": "your_secret_key_here",
  "authkeyExpiredTimeout": 30,
  "logLevel": "warning",
  "tempDir": "./temp",
  "reserveHistory": false,
  "notify": {
    "copy": false,
    "paste": false
  },
  "sound": {
    "enabled": true,
    "filePath": "notification.wav"
  }
}
```

5. Save and relaunch `ClipSync.exe`
6. Confirm the 🦜 icon appears in your system tray — ClipSync is running

### iOS (Jailbroken)

1. **Install** the ClipSync tweak via your package manager (Havoc)
2. Open **Settings → ClipSync**:
   - Enter your Windows PC's local IP address (e.g. `192.168.1.100`)
   - Enter the same `authkey` you set in `config.json`
   - Leave port as `8086` unless you changed it
3. Tap **Test Connection** — you should see a success alert confirming HTTP 200 OK
4. Tap **Save**, then **Respring**

---

## ⚙️ Configuration

### config.json Reference

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `port` | string | `"8086"` | HTTP server port |
| `authkey` | string | `""` | Authentication key — must match iOS tweak setting |
| `authkeyExpiredTimeout` | int | `30` | Auth token validity in seconds |
| `logLevel` | string | `"warning"` | Log verbosity: `"debug"`, `"info"`, `"warning"`, `"error"` |
| `tempDir` | string | `"./temp"` | Temporary file storage path |
| `reserveHistory` | bool | `false` | Keep temp file history between sessions |
| `notify.copy` | bool | `false` | Show Windows toast notification on copy |
| `notify.paste` | bool | `false` | Show Windows toast notification on paste |
| `sound.enabled` | bool | `true` | Play sound on clipboard activity |
| `sound.filePath` | string | `"notification.wav"` | Path to sound file (relative to exe) |

### System Tray Menu

Right-click the 🦜 tray icon for options:
- **AutoRun** — toggle launch on Windows startup
- **Sound** — toggle sound notifications on/off
- **Exit** — quit ClipSync

---

## 🔧 Troubleshooting

### Connection Issues

**❌ Test connection fails**
- Make sure ClipSync.exe is running (🦜 in system tray)
- Check your Windows Firewall — port 8086 must be allowed
- Verify the IP address in iOS settings matches your PC's local IP
- Make sure `authkey` is identical on both sides

**❌ Status 401 Unauthorized**
- `authkey` mismatch between iOS and `config.json`
- Double-check for typos or trailing spaces

### Clipboard Issues

**❌ OpenClipboard failed**
- Another app is temporarily locking the clipboard (Office, Teams, OneNote)
- ClipSync retries automatically up to 20 times — wait a moment

**❌ Nothing pastes on Windows**
- Check `log.txt` for errors
- Try right-clicking the tray icon and toggling Sound to confirm the app is responsive

### Logs

| File | Contents |
|------|----------|
| `log.txt` | Main application log |
| `history.txt` | Sync history with direction and content type |
| `crash.txt` | Full stack trace if the app crashes |

Set `logLevel` to `"info"` or `"debug"` in `config.json` for more verbose output when troubleshooting.

---

## 🔨 Building from Source

Requirements:
- Go 1.19+
- Windows (required for `lxn/walk` GUI framework)
- `rsrc` tool for embedding the icon resource

```bash
# Windows (PowerShell)
.\build.ps1

# Or using build script
bash build.sh
```

The `rsrc.syso` file is pre-included in the repo so you don't need to regenerate it unless you change `app.ico`.

---

## 🙏 Credits

ClipSync Windows app was inspired by and conceptually references **[clipboard-online](https://github.com/YanxinTang/clipboard-online)** by [YanxinTang](https://github.com/YanxinTang) (MIT License).

### Dependencies

- [lxn/walk](https://github.com/lxn/walk) — Windows GUI and system tray
- [gin-gonic/gin](https://github.com/gin-gonic/gin) — HTTP server
- [sirupsen/logrus](https://github.com/sirupsen/logrus) — structured logging

---

## 📄 License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

<div align="center">

**Made with ❤️ for the jailbreak community**

*© 2025 AnubisTweaks*

</div>
