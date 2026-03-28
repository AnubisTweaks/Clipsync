package utils

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	winmm         = syscall.NewLazyDLL("winmm.dll")
	mciSendString = winmm.NewProc("mciSendStringW")
	playSound     = winmm.NewProc("PlaySoundW")
	sndAsync      = 0x0001
	sndFilename   = 0x00020000
	sndNoStop     = 0x00000010
	soundCounter  uint64 // Atomic counter for unique aliases
)

// PlaySound plays audio files (WAV, MP3) asynchronously
// Uses PlaySoundW for WAV files (faster, allows overlapping)
// Uses MCI for MP3 files (supports more formats)
func PlaySound(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename is empty")
	}
	
	ext := strings.ToLower(filepath.Ext(filename))
	
	// Use PlaySoundW for WAV files (faster, native support)
	if ext == ".wav" {
		return playSoundWAV(filename)
	}
	
	// Use MCI for MP3 and other formats
	return playSoundMCI(filename)
}

// playSoundWAV plays WAV files using PlaySoundW API
func playSoundWAV(filename string) error {
	filenamePtr, err := syscall.UTF16PtrFromString(filename)
	if err != nil {
		return err
	}
	
	// SND_ASYNC | SND_FILENAME | SND_NOSTOP
	// NOSTOP allows new sound to play even if one is already playing
	ret, _, _ := playSound.Call(
		uintptr(unsafe.Pointer(filenamePtr)),
		0,
		uintptr(sndFilename|sndAsync|sndNoStop),
	)
	
	if ret == 0 {
		return fmt.Errorf("failed to play WAV sound")
	}
	
	return nil
}

// playSoundMCI plays MP3 and other formats using MCI (Media Control Interface)
func playSoundMCI(filename string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}
	
	// Generate UNIQUE alias for each playback using timestamp + counter
	counter := atomic.AddUint64(&soundCounter, 1)
	alias := fmt.Sprintf("cs_%d_%d", time.Now().UnixNano(), counter)
	
	// Open the sound file
	openCmd := fmt.Sprintf("open \"%s\" type mpegvideo alias %s", absPath, alias)
	openCmdPtr, err := syscall.UTF16PtrFromString(openCmd)
	if err != nil {
		return err
	}
	
	ret, _, _ := mciSendString.Call(
		uintptr(unsafe.Pointer(openCmdPtr)),
		0,
		0,
		0,
	)
	
	if ret != 0 {
		return fmt.Errorf("failed to open sound file (MCI error code: %d)", ret)
	}
	
	// Play the sound (async - no wait)
	playCmd := fmt.Sprintf("play %s", alias)
	playCmdPtr, err := syscall.UTF16PtrFromString(playCmd)
	if err != nil {
		// Close on error
		closeCmd := fmt.Sprintf("close %s", alias)
		closeCmdPtr, _ := syscall.UTF16PtrFromString(closeCmd)
		mciSendString.Call(uintptr(unsafe.Pointer(closeCmdPtr)), 0, 0, 0)
		return err
	}
	
	ret, _, _ = mciSendString.Call(
		uintptr(unsafe.Pointer(playCmdPtr)),
		0,
		0,
		0,
	)
	
	if ret != 0 {
		// Close on error
		closeCmd := fmt.Sprintf("close %s", alias)
		closeCmdPtr, _ := syscall.UTF16PtrFromString(closeCmd)
		mciSendString.Call(uintptr(unsafe.Pointer(closeCmdPtr)), 0, 0, 0)
		return fmt.Errorf("failed to play sound (MCI error code: %d)", ret)
	}
	
	// Schedule cleanup after sound finishes (approximate duration: 1 second)
	go func() {
		time.Sleep(2 * time.Second) // Wait for sound to finish
		closeCmd := fmt.Sprintf("close %s", alias)
		closeCmdPtr, _ := syscall.UTF16PtrFromString(closeCmd)
		mciSendString.Call(uintptr(unsafe.Pointer(closeCmdPtr)), 0, 0, 0)
	}()
	
	return nil
}
