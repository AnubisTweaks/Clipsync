package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	
	log "github.com/sirupsen/logrus"
)

type ClipboardMonitor struct {
	callback     func()
	stopChan     chan bool
	pollInterval time.Duration
}

func NewClipboardMonitor(callback func()) *ClipboardMonitor {
	return &ClipboardMonitor{
		callback:     callback,
		stopChan:     make(chan bool),
		pollInterval: 50 * time.Millisecond, // Check every 50ms for faster detection
	}
}

func (cm *ClipboardMonitor) Start(hwnd uintptr) {
	go cm.pollLoop()
}

func (cm *ClipboardMonitor) pollLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("panic", r).Error("💥 Clipboard monitor panic recovered")
			// Log to crash file
			execPath, _ := os.Executable()
			execDir := filepath.Dir(execPath)
			crashPath := filepath.Join(execDir, "crash.txt")
			if f, err := os.OpenFile(crashPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
				defer f.Close()
				f.WriteString(fmt.Sprintf("\n[%s] Clipboard Monitor Panic: %v\n", 
					time.Now().Format("2006-01-02 15:04:05"), r))
			}
		}
	}()
	
	var lastSequence uint32
	
	// Get initial clipboard sequence number
	seq, err := GetClipboardSequenceNumber()
	if err == nil {
		lastSequence = seq
	}
	
	ticker := time.NewTicker(cm.pollInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-cm.stopChan:
			return
		case <-ticker.C:
			currentSequence, err := GetClipboardSequenceNumber()
			if err != nil {
				continue
			}
			
			if currentSequence != lastSequence {
				lastSequence = currentSequence
				if cm.callback != nil {
					// Protect callback execution with panic recovery
					func() {
						defer func() {
							if r := recover(); r != nil {
								log.WithField("panic", r).Error("💥 Clipboard callback panic recovered")
							}
						}()
						cm.callback()
					}()
				}
			}
		}
	}
}

func (cm *ClipboardMonitor) Stop() {
	close(cm.stopChan)
}
