package utils

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	winmm       = syscall.NewLazyDLL("winmm.dll")
	playSound   = winmm.NewProc("PlaySoundW")
	sndAsync    = 0x0001
	sndFilename = 0x00020000
	sndNoStop   = 0x00000010
)

// PlaySound plays a WAV file asynchronously, allowing multiple simultaneous playbacks
func PlaySound(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename is empty")
	}
	
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
		return fmt.Errorf("failed to play sound")
	}
	
	return nil
}
