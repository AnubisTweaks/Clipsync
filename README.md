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

**ClipSync** is a powerful clipboard synchronization tool that enables seamless text and file sharing between your iOS device and Windows PC over your local network. Copy on one device, paste on another - it's that simple!

### ✨ Key Features

- 🔄 **Bidirectional Sync**: Copy from iOS to Windows and vice versa
- ⚡ **Lightning Fast**: Direct clipboard write with smart retry mechanism
- 📝 **Large Text Support**: Handle code files with 800+ lines effortlessly
- 🖼️ **Image & File Transfer**: Sync images and files between devices
- 🔒 **Secure**: Optional authentication with MD5 hashing
- 🎯 **Smart Recovery**: Automatic clipboard recovery when Windows clipboard gets stuck
- 🔕 **Silent Operation**: Runs quietly in the system tray
- 💾 **Temp File Fallback**: Uses temporary files when clipboard is locked
- 🔔 **Notifications**: Optional desktop notifications for sync events

---

## 🚀 How It Works

ClipSync consists of two components:

### **1. Windows Server (ClipSync.exe)**
- Runs in the background as a system tray application
- Provides HTTP API on port 8086
- Handles clipboard read/write operations
- Manages temporary file storage for large content

### **2. ClipSync iOS Tweak (Jailbreak Required)**
- Monitors clipboard changes on iOS
- Automatically syncs clipboard content via HTTP
- Works seamlessly in the background



---

## 📥 Installation

### Windows

1. **Download** `ClipSync.exe`
2. **Run** the executable - it will create `config.json` automatically
3. **Configure** (optional):
   ```json
   {
     "port": "8086",
     "authkey": "",
     "logLevel": 4,
     "tempDir": "./temp"
   }
   ```
4. **Allow** through Windows Firewall if prompted
5. Check system tray for the 🦜
   
### iOS (Jailbroken)

1. **Install** the ClipSync tweak via your package manager
2. **Configure** in Settings → ClipSync:
   - Windows IP: `192.168.x.x` (your PC's local IP)
   - Server Port: `8086`
   - Auth Key: (leave empty or match Windows config)
3. **Apply Settings** and test!

---

## 🎯 Usage

### Basic Usage

**iOS → Windows:**
1. Select and copy text on iOS
2. Wait 2-3 seconds
3. Paste on Windows (`Ctrl+V`) ✨

**Windows → iOS:**
1. Copy text on Windows (`Ctrl+C`)
2. Paste on iOS (tap & hold → Paste) ✨

### Advanced Features

**Large Text (800+ lines):**
- don't worry about long codes
- Background sync ensures Windows paste works eventually

**Notifications:**
- Enable in `config.json`:
  ```json
  "notify": {
    "copy": true,
    "paste": true
  }
  ```

**Authentication:**
- Set matching `authkey` in both Windows and iOS configs

---

## ⚙️ Configuration

### Windows (config.json)

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `port` | string | `"8086"` | HTTP server port |
| `authkey` | string | `""` | Authentication key (empty = disabled) |
| `authkeyExpiredTimeout` | int | `30` | Auth token validity (seconds) |
| `logLevel` | int | `4` | Log verbosity (0-6) |
| `tempDir` | string | `"./temp"` | Temporary file storage |
| `reserveHistory` | bool | `false` | Keep temp file history |
| `notify.copy` | bool | `false` | Show copy notifications |
| `notify.paste` | bool | `false` | Show paste notifications |

### iOS Tweak Settings

- **Windows IP**: Your PC's local network IP
- **Server Port**: 8086 (default)
- **Auth Key**: Must match Windows config
- **Sync Text**: Enable/disable text sync
- **Sync Images**: Enable/disable image sync

---

## 🔧 Troubleshooting

### Common Issues

**❌ "OpenClipboard failed" Error**
- **Cause**: Another app is locking the clipboard (Office, OneNote, Teams)
- **Solution**: Close clipboard-using apps or wait for automatic retry (20 attempts)

**❌ Status Code 403**
- **Cause**: Authentication failure
- **Solution**: Set `authkey` to `""` in both configs to disable auth

**❌ Paste Nothing on Windows**
- **Cause**: Clipboard locked during sync
- **Solution**: Wait 5-10 seconds for background sync to complete
- **Check**: Look for "✅ SUCCESS! Text synced" in logs



### Logs

Windows logs: `log.txt` (same folder as ClipSync.exe)

Check for:
- `✅ Direct clipboard write SUCCESS` - Good!
- `⏳ clipboard locked, retrying...` - Normal, will succeed
- `❌ Direct clipboard write failed` - Check which app is blocking

---



## 🙏 Credits & Thanks

This project is based on the excellent work of **[clipboard-online](https://github.com/YanxinTang/clipboard-online)** by [YanxinTang](https://github.com/YanxinTang).

### Major Enhancements in ClipSync

- ✨ **Direct Clipboard Write**: Removed complex temp file logic for iOS→Windows
- 🔄 **Smart Retry**: 20-attempt retry with progressive delays
- 💾 **Intelligent Fallback**: Temp files only when absolutely necessary
- 🩹 **Clipboard Recovery**: Automatic recovery from Windows clipboard exhaustion
- 🎯 **Optimized Performance**: Better handling of large text (800+ lines)
- 🔧 **Enhanced Logging**: Detailed operation tracking and troubleshooting
- 🦜 **Modern UI**: Custom hummingbird icon and branding

### Special Thanks

- **YanxinTang** for the original clipboard-online foundation
- **lxn/walk** for Windows GUI framework
- The Go community for excellent tooling

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Areas for Improvement

- [ ] Support for clipboard history
- [ ] Multi-device support (>2 devices)
- [ ] Cross-platform (macOS, Linux)
- [ ] Encryption for network transfer
- [ ] GUI configuration tool



## ⭐ Show Your Support

If ClipSync helps your workflow, please give it a ⭐ on GitHub!

---

<div align="center">

**Made with ❤️ for seamless clipboard sync**

*Based on clipboard-online by YanxinTang*

</div>
