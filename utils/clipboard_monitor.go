package utils

import (
	"time"
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
		pollInterval: 100 * time.Millisecond, // Check every 100ms
	}
}

func (cm *ClipboardMonitor) Start(hwnd uintptr) {
	go cm.pollLoop()
}

func (cm *ClipboardMonitor) pollLoop() {
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
					cm.callback()
				}
			}
		}
	}
}

func (cm *ClipboardMonitor) Stop() {
	close(cm.stopChan)
}
