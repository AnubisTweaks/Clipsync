package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image/png"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/AnubisTweaks/Clipsync/utils"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"golang.org/x/image/bmp"
)

const (
	apiVersion = "1"
)

type ClipboardManager struct {
	mu                sync.RWMutex
	lastText          string
	lastTextHash      string      // NEW: Hash of last text
	lastImageHash     string      // NEW: Hash of last image
	lastFileHash      string      // NEW: Hash of last file(s)
	lastSentHash      string      // NEW: Hash of last content sent to iOS
	lastSentTime      time.Time   // NEW: When we last sent to iOS
	lastReceivedHash  string      // NEW: Hash of last content received FROM iOS
	lastReceivedTime  time.Time   // NEW: When we last received FROM iOS
	lastTextTime      time.Time
	cacheTTL          time.Duration
	isWriting         bool
	writeQueue        chan clipboardWrite
	lastGC            time.Time
	operationCount    int
	maxOpsBeforeGC    int
	pendingWrites     int
	maxPendingWrites  int
	largeTextFile     string
	largeTextTime     time.Time
	useLargeFile      bool
	clipboardResets   int
	lastResetTime     time.Time
	emergencyMode     bool
	failedOperations  int
	consecutiveFails  int
	monitor           *utils.ClipboardMonitor
	lastPlayedHash    string      // Track last hash we played sound for
	lastPlayedTime    time.Time   // Track when we last played sound (prevent rapid duplicates)
}

type clipboardWrite struct {
	text     string
	files    []string
	isFile   bool
	done     chan error
}

var clipboardManager *ClipboardManager

func initClipboardManager() {
	log.Info("============================================")
	log.Info("Initializing ClipboardManager v8.1-FIXED")
	log.Info("✓ Hash tracking for duplicate prevention")
	log.Info("✓ Prevents sending same content repeatedly to iOS")
	log.Info("✓ Direct clipboard write with smart retry")
	log.Info("✓ FIXED: No longer clears clipboard during maintenance")
	log.Info("✓ FIXED: Read-only initialization (no writes on startup)")
	log.Info("✓ Real-time clipboard change detection")
	log.Info("============================================")
	
	clipboardManager = &ClipboardManager{
		cacheTTL:         500 * time.Millisecond,  // OPTIMIZED: 0.5s cache (was 5s) - matches iOS polling
		writeQueue:       make(chan clipboardWrite, 10),
		maxOpsBeforeGC:   50,
		maxPendingWrites: 5,
	}
	
	// Initialize clipboard monitor with callback
	clipboardManager.monitor = utils.NewClipboardMonitor(onClipboardChanged)
	
	// Start monitor immediately
	clipboardManager.monitor.Start(0)
	
	// Initialize current clipboard hash WITHOUT writing to clipboard
	// Just read what's there without modifying anything
	go func() {
		time.Sleep(2 * time.Second)
		// Only READ the clipboard, don't write anything
		contentType, err := utils.Clipboard().ContentType()
		if err == nil {
			switch contentType {
			case utils.TypeText:
				if text, err := utils.Clipboard().Text(); err == nil && text != "" {
					clipboardManager.mu.Lock()
					clipboardManager.lastText = text
					clipboardManager.lastTextHash = calculateHash([]byte(text))
					clipboardManager.lastTextTime = time.Now()
					// Don't set lastPlayedHash here - allow first copy to play sound
					clipboardManager.mu.Unlock()
					log.WithField("hash", clipboardManager.lastTextHash[:8]+"...").Debug("initialized text hash (read-only)")
				}
			case utils.TypeBitmap:
				if bmpBytes, err := utils.Clipboard().Bitmap(); err == nil {
					clipboardManager.mu.Lock()
					clipboardManager.lastImageHash = calculateHash(bmpBytes)
					// Don't set lastPlayedHash here - allow first copy to play sound
					clipboardManager.mu.Unlock()
					log.WithField("hash", clipboardManager.lastImageHash[:8]+"...").Debug("initialized image hash (read-only)")
				}
			case utils.TypeFile:
				if files, err := utils.Clipboard().Files(); err == nil && len(files) > 0 {
					filesStr := strings.Join(files, "|")
					clipboardManager.mu.Lock()
					clipboardManager.lastFileHash = calculateHash([]byte(filesStr))
					// Don't set lastPlayedHash here - allow first copy to play sound
					clipboardManager.mu.Unlock()
					log.WithField("hash", clipboardManager.lastFileHash[:8]+"...").Debug("initialized file hash (read-only)")
				}
			}
		}
	}()
	
	go clipboardManager.worker()
	go clipboardManager.periodicMaintenance()
	
	log.Info("ClipboardManager initialized successfully")
}

// Callback when clipboard changes detected by monitor
func onClipboardChanged() {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("panic", r).Error("panic in onClipboardChanged")
		}
	}()
	
	log.Warn("📋 CLIPBOARD CHANGE DETECTED") // Use Warn so it always logs
	
	// Minimal debouncing - only prevent duplicate polling within 50ms
	clipboardManager.mu.Lock()
	timeSinceLastSound := time.Since(clipboardManager.lastPlayedTime)
	
	if timeSinceLastSound < 50*time.Millisecond {
		clipboardManager.mu.Unlock()
		log.WithField("timeSinceMs", timeSinceLastSound.Milliseconds()).Warn("⏱️ SKIPPING SOUND - too soon (debounce <50ms)")
		return
	}
	
	// Update last played time
	clipboardManager.lastPlayedTime = time.Now()
	clipboardManager.mu.Unlock()
	
	// Play sound immediately
	log.WithField("timeSinceLastMs", timeSinceLastSound.Milliseconds()).Warn("🔊 PLAYING SOUND NOW")
	playClipboardSound()
}

// NEW: Calculate MD5 hash of data
func calculateHash(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// NEW: Log to history.txt with colored emojis
func logToHistory(direction, contentType, content string) {
	historyPath := filepath.Join(execPath, "history.txt")
	
	// Open or create history file in append mode
	f, err := os.OpenFile(historyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.WithError(err).Error("failed to open history.txt")
		return
	}
	defer f.Close()
	
	// Format: [timestamp] [emoji arrow] [type] content
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	
	var arrow, typeEmoji string
	if direction == "to_ios" {
		arrow = "🟦 →" // Blue square for Windows → iOS
	} else {
		arrow = "🟩 ←" // Green square for iOS → Windows
	}
	
	// Add type emoji
	switch contentType {
	case "text":
		typeEmoji = "📝"
	case "image":
		typeEmoji = "🖼️"
	case "file":
		typeEmoji = "📁"
	default:
		typeEmoji = "📋"
	}
	
	// Truncate content for display
	displayContent := content
	if len(content) > 100 {
		displayContent = content[:97] + "..."
	}
	
	logLine := fmt.Sprintf("[%s] %s %s %s\n", timestamp, arrow, typeEmoji, displayContent)
	
	if _, err := f.WriteString(logLine); err != nil {
		log.WithError(err).Error("failed to write to history.txt")
	}
}

// NEW: Log error to log.txt
func logError(category, message string, err error) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("[%s] [%s] %s", timestamp, category, message)
	if err != nil {
		logLine += fmt.Sprintf(": %v", err)
	}
	log.Error(logLine)
}

// Play sound when clipboard operation occurs
func playClipboardSound() {
	log.Debug("playClipboardSound called")
	
	if !app.IsSoundEnabled() {
		log.Debug("sound is disabled in settings")
		return
	}
	
	soundPath := filepath.Join(execPath, app.config.Sound.FilePath)
	log.WithField("soundPath", soundPath).Debug("checking sound file")
	
	if !utils.IsExistFile(soundPath) {
		log.WithField("path", soundPath).Error("❌ sound file not found")
		// Try to show error to user once
		go app.ni.ShowError("Sound Error", fmt.Sprintf("Sound file not found: %s", app.config.Sound.FilePath))
		return
	}
	
	log.WithField("path", soundPath).Info("🔊 playing sound file")
	
	// Play sound asynchronously to avoid blocking
	go func() {
		if err := utils.PlaySound(soundPath); err != nil {
			log.WithError(err).WithField("path", soundPath).Error("❌ failed to play sound")
		} else {
			log.Info("✓ sound played successfully")
		}
	}()
}

// NEW: Get current clipboard hash
func (cm *ClipboardManager) GetCurrentHash() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	contentType, _ := utils.Clipboard().ContentType()
	switch contentType {
	case utils.TypeText:
		return cm.lastTextHash
	case utils.TypeBitmap:
		return cm.lastImageHash
	case utils.TypeFile:
		return cm.lastFileHash
	default:
		return ""
	}
}

func (cm *ClipboardManager) worker() {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("panic", r).Error("💥 Clipboard worker panic recovered")
			// Log to crash file
			crashPath := filepath.Join(execPath, "crash.txt")
			if f, err := os.OpenFile(crashPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
				defer f.Close()
				f.WriteString(fmt.Sprintf("\n[%s] Clipboard Worker Panic: %v\n", 
					time.Now().Format("2006-01-02 15:04:05"), r))
			}
		}
	}()
	
	for write := range cm.writeQueue {
		cm.mu.Lock()
		cm.isWriting = true
		cm.operationCount++
		cm.mu.Unlock()
		
		var err error
		if write.isFile {
			err = cm.writeFilesSync(write.files)
		} else {
			err = cm.writeTextSync(write.text)
		}
		
		cm.mu.Lock()
		cm.isWriting = false
		cm.pendingWrites--
		cm.mu.Unlock()
		
		cm.forceCleanup()
		
		if write.done != nil {
			write.done <- err
			close(write.done)
		}
		
		time.Sleep(300 * time.Millisecond)
	}
}

func (cm *ClipboardManager) periodicMaintenance() {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("panic", r).Error("💥 Periodic maintenance panic recovered")
		}
	}()
	
	ticker := time.NewTicker(30 * time.Second) // Changed from 10s to 30s
	defer ticker.Stop()
	
	for range ticker.C {
		cm.mu.Lock()
		
		if cm.consecutiveFails >= 5 { // Changed from 3 to 5
			log.Error("⚠️⚠️⚠️ CLIPBOARD CRITICALLY BROKEN - forcing immediate reset")
			cm.mu.Unlock()
			cm.resetClipboard()
			cm.mu.Lock()
		}
		
		if !cm.isWriting && cm.pendingWrites == 0 {
			cm.forceCleanup()
			
			if time.Since(cm.lastResetTime) > 10*time.Minute { // Changed from 3min to 10min
				cm.mu.Unlock()
				cm.resetClipboard()
				cm.mu.Lock()
			}
		}
		
		cm.mu.Unlock()
	}
}

func (cm *ClipboardManager) resetClipboard() {
	log.Info("============================================")
	log.Info("SMART CLIPBOARD RECOVERY")
	log.Info("============================================")
	
	log.Info("Step 1: Cleaning up old temp files...")
	tempDir := app.GetTempFilePath("")
	if files, err := ioutil.ReadDir(tempDir); err == nil {
		cleaned := 0
		for _, file := range files {
			if strings.HasPrefix(file.Name(), "_clipsync_") {
				if time.Since(file.ModTime()) > 10*time.Minute {
					filePath := filepath.Join(tempDir, file.Name())
					os.Remove(filePath)
					cleaned++
				}
			}
		}
		log.WithField("filesRemoved", cleaned).Info("old temp files cleaned (>10min)")
	}
	
	// REMOVED: The clipboard clearing that was causing the bug!
	// We DO NOT write empty text to clipboard anymore
	log.Info("Step 2: Running memory cleanup (WITHOUT touching clipboard)...")
	
	// Just do aggressive GC cleanup without touching clipboard
	for i := 0; i < 15; i++ {
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
	}
	
	cm.mu.Lock()
	cm.clipboardResets++
	cm.lastResetTime = time.Now()
	cm.emergencyMode = false
	cm.operationCount = 0
	cm.consecutiveFails = 0
	totalResets := cm.clipboardResets
	cm.mu.Unlock()
	
	log.WithFields(logrus.Fields{
		"totalResets": totalResets,
	}).Info("✓✓✓ SMART RECOVERY COMPLETE ✓✓✓")
	log.Info("Cleaned memory WITHOUT touching clipboard")
	log.Info("============================================")
}

func (cm *ClipboardManager) forceCleanup() {
	runtime.GC()
	cm.operationCount = 0
	cm.lastGC = time.Now()
	
	if cm.useLargeFile && time.Since(cm.largeTextTime) > 60*time.Second {
		if utils.IsExistFile(cm.largeTextFile) {
			os.Remove(cm.largeTextFile)
			log.WithField("path", cm.largeTextFile).Debug("cleaned up expired large text file")
		}
		cm.useLargeFile = false
		cm.largeTextFile = ""
	}
	
	log.Debug("performed clipboard cleanup and GC")
}

func (cm *ClipboardManager) GetCachedText() (string, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	if time.Since(cm.lastTextTime) < cm.cacheTTL {
		return cm.lastText, true
	}
	return "", false
}

func (cm *ClipboardManager) InvalidateCache() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.lastText = ""
	cm.lastTextTime = time.Time{}
}

func (cm *ClipboardManager) SetText(text string) error {
	// NEW: Check if this is duplicate content
	textHash := calculateHash([]byte(text))
	
	cm.mu.RLock()
	if textHash == cm.lastTextHash {
		cm.mu.RUnlock()
		log.WithField("hash", textHash[:8]+"...").Debug("skipping duplicate text")
		return nil
	}
	cm.mu.RUnlock()
	
	cm.mu.Lock()
	
	if cm.pendingWrites >= cm.maxPendingWrites {
		cm.mu.Unlock()
		log.Warn("write queue full - will use direct write")
		return cm.writeTextSync(text)
	}
	
	cm.pendingWrites++
	cm.mu.Unlock()
	
	done := make(chan error, 1)
	
	select {
	case cm.writeQueue <- clipboardWrite{
		text:   text,
		isFile: false,
		done:   done,
	}:
	case <-time.After(2 * time.Second):
		cm.mu.Lock()
		cm.pendingWrites--
		cm.mu.Unlock()
		log.Warn("write queue timeout - using direct write")
		return cm.writeTextSync(text)
	}
	
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		log.Error("write timeout")
		return fmt.Errorf("clipboard write timeout")
	}
}

func (cm *ClipboardManager) SetFiles(files []string) error {
	// NEW: Check if this is duplicate content
	filesStr := strings.Join(files, "|")
	fileHash := calculateHash([]byte(filesStr))
	
	cm.mu.RLock()
	if fileHash == cm.lastFileHash {
		cm.mu.RUnlock()
		log.WithField("hash", fileHash[:8]+"...").Debug("skipping duplicate files")
		return nil
	}
	cm.mu.RUnlock()
	
	cm.mu.Lock()
	
	if cm.pendingWrites >= cm.maxPendingWrites {
		cm.mu.Unlock()
		return fmt.Errorf("clipboard write queue full, please wait")
	}
	
	cm.pendingWrites++
	cm.mu.Unlock()
	
	done := make(chan error, 1)
	
	select {
	case cm.writeQueue <- clipboardWrite{
		files:  files,
		isFile: true,
		done:   done,
	}:
	case <-time.After(2 * time.Second):
		cm.mu.Lock()
		cm.pendingWrites--
		cm.mu.Unlock()
		return fmt.Errorf("clipboard write timeout - queue full")
	}
	
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		return fmt.Errorf("clipboard write timeout")
	}
}

func (cm *ClipboardManager) writeTextSync(text string) error {
	textLength := len(text)
	lineCount := strings.Count(text, "\n") + 1
	textHash := calculateHash([]byte(text))
	
	log.WithFields(logrus.Fields{
		"length": textLength,
		"lines":  lineCount,
		"hash":   textHash[:8] + "...",
	}).Info("writeTextSync - attempting DIRECT clipboard write")
	
	maxRetries := 20
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := utils.Clipboard().SetText(text)
		if err == nil {
			log.WithFields(logrus.Fields{
				"attempt": attempt,
				"length":  textLength,
			}).Info("✓✓✓ DIRECT write SUCCESS")
			
			cm.mu.Lock()
			cm.lastText = text
			cm.lastTextHash = textHash
			cm.lastTextTime = time.Now()
			cm.consecutiveFails = 0
			cm.mu.Unlock()
			
			return nil
		}
		
		if attempt == 1 {
			log.WithError(err).Warn("attempt 1 failed, will retry...")
		}
		
		backoff := time.Duration(attempt*200) * time.Millisecond
		time.Sleep(backoff)
		
		if attempt%5 == 0 {
			log.WithFields(logrus.Fields{
				"attempt":     attempt,
				"maxRetries":  maxRetries,
				"nextBackoff": backoff,
			}).Warn("still retrying...")
			runtime.GC()
		}
	}
	
	cm.mu.Lock()
	cm.consecutiveFails++
	cm.failedOperations++
	fails := cm.consecutiveFails
	cm.mu.Unlock()
	
	log.WithFields(logrus.Fields{
		"consecutiveFails": fails,
		"length":           textLength,
	}).Error("✗✗✗ ALL RETRIES EXHAUSTED - clipboard operation FAILED")
	
	return fmt.Errorf("failed to write text after %d attempts", maxRetries)
}

func (cm *ClipboardManager) writeFilesSync(files []string) error {
	filesStr := strings.Join(files, "|")
	fileHash := calculateHash([]byte(filesStr))
	
	log.WithFields(logrus.Fields{
		"fileCount": len(files),
		"hash":      fileHash[:8] + "...",
	}).Info("writeFilesSync - attempting write")
	
	maxRetries := 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := utils.Clipboard().SetFiles(files)
		if err == nil {
			log.WithField("attempt", attempt).Info("✓ files written successfully")
			
			cm.mu.Lock()
			cm.lastFileHash = fileHash
			cm.consecutiveFails = 0
			cm.mu.Unlock()
			
			return nil
		}
		
		backoff := time.Duration(attempt*500) * time.Millisecond
		time.Sleep(backoff)
		
		if attempt < maxRetries {
			log.WithError(err).WithField("attempt", attempt).Warn("file write failed, retrying...")
			runtime.GC()
		}
	}
	
	cm.mu.Lock()
	cm.consecutiveFails++
	cm.failedOperations++
	cm.mu.Unlock()
	
	log.Error("✗ failed to write files after all retries")
	return fmt.Errorf("failed to write files after %d attempts", maxRetries)
}

func setupRoute(engin *gin.Engine) {
	engin.Use(
		RequestLogger(),
		ErrorRecoveryMiddleware(),
		gin.Recovery(),
		AuthMiddleware(),
		ClientNameMiddleware(),
	)

	engin.GET("/", getHandler)
	engin.POST("/", setHandler)
	engin.NoRoute(notFoundHandler)
}

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		ip := c.ClientIP()

		c.Next()

		end := time.Now()
		latency := end.Sub(start)
		
		// Only log server errors (500+), not client errors like 400
		// Clipboard locked errors now return 200 with empty data
		if c.Writer.Status() >= 500 {
			logError("SERVER_ERROR", 
				fmt.Sprintf("%s %s returned status %d from %s", 
					c.Request.Method, c.Request.URL.Path, c.Writer.Status(), ip), nil)
		}

		log.WithFields(logrus.Fields{
			"ip":      ip,
			"method":  c.Request.Method,
			"latency": latency,
			"status":  c.Writer.Status(),
		}).Info("request")
	}
}

// Error recovery middleware with logging
func ErrorRecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.WithFields(logrus.Fields{
					"error": err,
					"path":  c.Request.URL.Path,
					"ip":    c.ClientIP(),
				}).Error("panic recovered in request handler")
				logError("PANIC", fmt.Sprintf("Panic in %s: %v", c.Request.URL.Path, err), nil)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientVersion := c.GetHeader("X-API-Version")
		clientAuth := c.GetHeader("X-Auth")

		if clientVersion != apiVersion {
			log.WithFields(logrus.Fields{
				"clientVersion": clientVersion,
				"serverVersion": apiVersion,
			}).Warn("client version mismatch")
		}

		if app.config.Authkey == "" {
			c.Next()
			return
		}

		timeout := app.config.AuthkeyExpiredTimeout
		now := time.Now().Unix()
		currentSlot := now / timeout

		validHashes := make([]string, 0, 3)
		for i := int64(-1); i <= 1; i++ {
			slot := currentSlot + i
			authString := fmt.Sprintf("%s.%d", app.config.Authkey, slot)
			hash := md5.Sum([]byte(authString))
			validHashes = append(validHashes, hex.EncodeToString(hash[:]))
		}

		isValid := false
		for _, validHash := range validHashes {
			if clientAuth == validHash {
				isValid = true
				break
			}
		}

		if !isValid {
			log.WithFields(logrus.Fields{
				"clientAuth":  clientAuth,
				"validHashes": validHashes,
			}).Warn("auth failed")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}

func ClientNameMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientName := c.GetHeader("X-Client-Name")
		if clientName == "" {
			clientName = "Unknown client"
		}
		c.Set("clientName", clientName)
		c.Next()
	}
}

type ResponseFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func getHandler(c *gin.Context) {
	log.Info("════════════════════════════════════════")
	log.Info("📤 GET REQUEST - Sending clipboard to iOS")
	log.WithFields(logrus.Fields{
		"ip":         c.ClientIP(),
		"userAgent":  c.GetHeader("User-Agent"),
		"clientName": c.GetString("clientName"),
	}).Info("client info")
	
	contentType, err := utils.Clipboard().ContentType()
	if err != nil {
		// OpenClipboard can fail when another app has it locked (temporary)
		log.WithError(err).Debug("⏳ Clipboard temporarily unavailable (may be locked by another app)")
		c.JSON(http.StatusOK, gin.H{
			"type": "text",
			"data": "",
			"hash": "",
		})
		log.Info("════════════════════════════════════════")
		return
	}
	
	log.WithField("contentType", contentType).Info("detected clipboard type")

	if contentType == utils.TypeUnknown {
		log.Info("⚠️  Clipboard is EMPTY or contains unknown type")
		c.JSON(http.StatusOK, gin.H{
			"type": "text",
			"data": "",
			"hash": "",
		})
		log.Info("════════════════════════════════════════")
		return
	}

	if contentType == utils.TypeText {
		log.Info("📝 Processing TEXT from Windows clipboard")
		
		var text string
		
		if cached, found := clipboardManager.GetCachedText(); found {
			text = cached
			log.Info("✓ Using cached text")
		} else {
			log.Info("Reading text from clipboard...")
			var err error
			text, err = utils.Clipboard().Text()
			if err != nil {
				log.WithError(err).Error("❌ Failed to read text from clipboard")
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read text"})
				log.Info("════════════════════════════════════════")
				return
			}
			log.WithField("length", len(text)).Info("✓ Text read successfully")
		}

		textHash := calculateHash([]byte(text))
		
		// NEW: Check if this content was received FROM iOS - if yes, don't send it back
		clipboardManager.mu.RLock()
		isFromIOS := (textHash == clipboardManager.lastReceivedHash && clipboardManager.lastReceivedHash != "")
		alreadySent := (textHash == clipboardManager.lastSentHash && clipboardManager.lastSentHash != "")
		clipboardManager.mu.RUnlock()
		
		if isFromIOS {
			log.WithFields(logrus.Fields{
				"hash": textHash[:8] + "...",
			}).Info("🔄 Skipping - this content was received FROM iOS, preventing bounce-back")
			c.JSON(http.StatusOK, gin.H{
				"type": "text",
				"data": "",
				"hash": "",
			})
			log.Info("════════════════════════════════════════")
			return
		}
		
		if alreadySent {
			log.WithFields(logrus.Fields{
				"hash": textHash[:8] + "...",
			}).Info("⏭️  Skipping - already sent this text to iOS")
			c.JSON(http.StatusOK, gin.H{
				"type": "text",
				"data": "",
				"hash": "",
			})
			log.Info("════════════════════════════════════════")
			return
		}
		
		textLength := len(text)
		lineCount := strings.Count(text, "\n") + 1
		preview := getTruncatedText(text, 80)

		log.WithFields(logrus.Fields{
			"length":  textLength,
			"lines":   lineCount,
			"hash":    textHash[:8] + "...",
			"preview": preview,
		}).Info("✅ Sending TEXT to iOS (type: text)")
		
		// NEW: Log to history.txt
		logToHistory("to_ios", "text", preview)

		c.JSON(http.StatusOK, gin.H{
			"type": "text",
			"data": text,
			"hash": textHash,
		})
		
		// NEW: Mark this content as sent to iOS
		clipboardManager.mu.Lock()
		clipboardManager.lastSentHash = textHash
		clipboardManager.lastSentTime = time.Now()
		clipboardManager.mu.Unlock()
		
		defer sendCopyNotification(log, c.GetString("clientName"), preview)
		log.Info("════════════════════════════════════════")
		return
	}

	if contentType == utils.TypeBitmap {
		log.Info("🖼️  Processing IMAGE/BITMAP from Windows clipboard")
		log.Info("Step 1: Reading bitmap data...")
		
		bmpBytes, err := utils.Clipboard().Bitmap()
		if err != nil {
			log.WithError(err).Error("❌ Failed to read bitmap from clipboard")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read image"})
			log.Info("════════════════════════════════════════")
			return
		}
		
		log.WithFields(logrus.Fields{
			"sizeBMP":   len(bmpBytes),
			"sizeBMPMB": float64(len(bmpBytes)) / 1024.0 / 1024.0,
		}).Info("✓ Bitmap data read successfully")

		log.Info("Step 2: Decoding BMP format...")
		img, err := bmp.Decode(bytes.NewReader(bmpBytes))
		if err != nil {
			log.WithError(err).Error("❌ Failed to decode bitmap")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to decode image"})
			log.Info("════════════════════════════════════════")
			return
		}
		
		log.WithFields(logrus.Fields{
			"width":  img.Bounds().Dx(),
			"height": img.Bounds().Dy(),
		}).Info("✓ BMP decoded successfully")

		log.Info("Step 3: Converting to PNG format...")
		var pngBuffer bytes.Buffer
		if err := png.Encode(&pngBuffer, img); err != nil {
			log.WithError(err).Error("❌ Failed to encode PNG")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to encode image"})
			log.Info("════════════════════════════════════════")
			return
		}
		
		pngBytes := pngBuffer.Bytes()
		imageHash := calculateHash(pngBytes)
		
		// NEW: Check if this image was received FROM iOS - if yes, don't send it back
		clipboardManager.mu.RLock()
		isFromIOS := (imageHash == clipboardManager.lastReceivedHash && clipboardManager.lastReceivedHash != "")
		alreadySent := (imageHash == clipboardManager.lastSentHash && clipboardManager.lastSentHash != "")
		clipboardManager.mu.RUnlock()
		
		if isFromIOS {
			log.WithFields(logrus.Fields{
				"hash": imageHash[:8] + "...",
			}).Info("🔄 Skipping - this image was received FROM iOS, preventing bounce-back")
			c.JSON(http.StatusOK, gin.H{
				"type": "text",
				"data": "",
				"hash": "",
			})
			log.Info("════════════════════════════════════════")
			return
		}
		
		if alreadySent {
			log.WithFields(logrus.Fields{
				"hash": imageHash[:8] + "...",
			}).Info("⏭️  Skipping - already sent this image to iOS")
			c.JSON(http.StatusOK, gin.H{
				"type": "text",
				"data": "",
				"hash": "",
			})
			log.Info("════════════════════════════════════════")
			return
		}
		
		log.WithFields(logrus.Fields{
			"sizePNG":   len(pngBytes),
			"sizePNGMB": float64(len(pngBytes)) / 1024.0 / 1024.0,
			"hash":      imageHash[:8] + "...",
		}).Info("✓ PNG conversion successful")

		log.Info("Step 4: Encoding to Base64...")
		base64Image := base64.StdEncoding.EncodeToString(pngBytes)
		
		log.WithFields(logrus.Fields{
			"base64Size":   len(base64Image),
			"base64SizeMB": float64(len(base64Image)) / 1024.0 / 1024.0,
		}).Info("✓ Base64 encoding complete")

		log.WithFields(logrus.Fields{
			"originalBMP": fmt.Sprintf("%.2f MB", float64(len(bmpBytes))/1024.0/1024.0),
			"finalPNG":    fmt.Sprintf("%.2f MB", float64(len(pngBytes))/1024.0/1024.0),
			"base64":      fmt.Sprintf("%.2f MB", float64(len(base64Image))/1024.0/1024.0),
			"hash":        imageHash[:8] + "...",
		}).Info("✅ Sending IMAGE to iOS (type: image)")
		
		// NEW: Log to history.txt
		imgDesc := fmt.Sprintf("Image %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
		logToHistory("to_ios", "image", imgDesc)

		c.JSON(http.StatusOK, gin.H{
			"type": "image",
			"data": base64Image,
			"hash": imageHash,
		})
		
		// NEW: Mark this image as sent to iOS
		clipboardManager.mu.Lock()
		clipboardManager.lastSentHash = imageHash
		clipboardManager.lastSentTime = time.Now()
		clipboardManager.mu.Unlock()
		
		defer sendCopyNotification(log, c.GetString("clientName"), fmt.Sprintf("[Image %dx%d]", img.Bounds().Dx(), img.Bounds().Dy()))
		log.Info("════════════════════════════════════════")
		return
	}

	if contentType == utils.TypeFile {
		log.Info("📁 Processing FILE(S) from Windows clipboard")
		log.Info("Step 1: Reading file paths...")
		
		filenames, err := utils.Clipboard().Files()
		if err != nil {
			log.WithError(err).Error("❌ Failed to read file paths from clipboard")
			c.Status(http.StatusBadRequest)
			log.Info("════════════════════════════════════════")
			return
		}
		
		log.WithField("fileCount", len(filenames)).Info("✓ File paths read successfully")
		
		for i, filename := range filenames {
			log.WithFields(logrus.Fields{
				"index": i + 1,
				"path":  filename,
			}).Info(fmt.Sprintf("  File %d: %s", i+1, filepath.Base(filename)))
		}

		log.Info("Step 2: Reading and encoding files...")
		responseFiles := make([]ResponseFile, 0, len(filenames))
		var totalSize int64 = 0
		var allFilesContent []byte  // NEW: For content-based hash
		
		for i, path := range filenames {
			fileName := filepath.Base(path)
			log.WithFields(logrus.Fields{
				"index":    i + 1,
				"fileName": fileName,
			}).Info(fmt.Sprintf("Processing file %d/%d...", i+1, len(filenames)))
			
			fileInfo, err := os.Stat(path)
			if err != nil {
				log.WithError(err).WithField("path", path).Warn("  ❌ File not accessible")
				continue
			}
			
			fileSize := fileInfo.Size()
			totalSize += fileSize
			
			log.WithFields(logrus.Fields{
				"size":   fileSize,
				"sizeMB": float64(fileSize) / 1024.0 / 1024.0,
			}).Info("  Reading file...")
			
			// Read raw file bytes for hash calculation
			fileBytes, err := ioutil.ReadFile(path)
			if err != nil {
				log.WithError(err).WithField("path", path).Warn("  ❌ Failed to read file")
				continue
			}
			
			// NEW: Add filename + raw bytes for content-based hash (no paths!)
			allFilesContent = append(allFilesContent, []byte(fileName)...)
			allFilesContent = append(allFilesContent, fileBytes...)
			
			// Encode to base64 for sending
			base64Data := base64.StdEncoding.EncodeToString(fileBytes)
			
			base64Size := len(base64Data)
			log.WithFields(logrus.Fields{
				"base64Size":   base64Size,
				"base64SizeMB": float64(base64Size) / 1024.0 / 1024.0,
			}).Info("  ✓ File encoded successfully")
			
			responseFiles = append(responseFiles, ResponseFile{
				Name:    fileName,
				Content: base64Data,
			})
		}
		
		if len(responseFiles) == 0 {
			log.Error("❌ No files could be read successfully")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read any files"})
			log.Info("════════════════════════════════════════")
			return
		}
		
		// NEW: Calculate hash based on file content only (filename + raw bytes)
		fileHash := calculateHash(allFilesContent)
		
		// NEW: Check if these files were received FROM iOS - if yes, don't send them back
		clipboardManager.mu.RLock()
		isFromIOS := (fileHash == clipboardManager.lastReceivedHash && clipboardManager.lastReceivedHash != "")
		alreadySent := (fileHash == clipboardManager.lastSentHash && clipboardManager.lastSentHash != "")
		lastReceivedHashShort := ""
		if clipboardManager.lastReceivedHash != "" {
			lastReceivedHashShort = clipboardManager.lastReceivedHash[:8] + "..."
		}
		clipboardManager.mu.RUnlock()
		
		log.WithFields(logrus.Fields{
			"currentHash":   fileHash[:8] + "...",
			"lastReceived":  lastReceivedHashShort,
			"isFromIOS":     isFromIOS,
			"alreadySent":   alreadySent,
		}).Info("📊 File bounce-back check")
		
		if isFromIOS {
			log.WithFields(logrus.Fields{
				"hash": fileHash[:8] + "...",
			}).Info("🔄 BOUNCE-BACK PREVENTED - these files were received FROM iOS")
			c.JSON(http.StatusOK, gin.H{
				"type": "text",
				"data": "",
				"hash": "",
			})
			log.Info("════════════════════════════════════════")
			return
		}
		
		if alreadySent {
			log.WithFields(logrus.Fields{
				"hash": fileHash[:8] + "...",
			}).Info("⏭️  Skipping - already sent these files to iOS")
			c.JSON(http.StatusOK, gin.H{
				"type": "text",
				"data": "",
				"hash": "",
			})
			log.Info("════════════════════════════════════════")
			return
		}
		
		log.WithFields(logrus.Fields{
			"filesSuccessful": len(responseFiles),
			"filesTotal":      len(filenames),
			"totalSizeMB":     float64(totalSize) / 1024.0 / 1024.0,
			"hash":            fileHash[:8] + "...",
		}).Info("✅ Sending FILE(S) to iOS (type: file)")
		
		log.Info("File list:")
		fileNamesList := make([]string, 0, len(responseFiles))
		for i, file := range responseFiles {
			log.WithFields(logrus.Fields{
				"index": i + 1,
				"name":  file.Name,
				"size":  fmt.Sprintf("%.2f MB", float64(len(file.Content))/1024.0/1024.0),
			}).Info(fmt.Sprintf("  %d. %s", i+1, file.Name))
			fileNamesList = append(fileNamesList, file.Name)
		}
		
		// NEW: Log to history.txt
		fileList := strings.Join(fileNamesList, ", ")
		if len(fileList) > 100 {
			fileList = fileList[:97] + "..."
		}
		logToHistory("to_ios", "file", fmt.Sprintf("%d file(s): %s", len(fileNamesList), fileList))

		c.JSON(http.StatusOK, gin.H{
			"type": "file",
			"data": responseFiles,
			"hash": fileHash,
		})
		
		// NEW: Mark these files as sent to iOS
		clipboardManager.mu.Lock()
		clipboardManager.lastSentHash = fileHash
		clipboardManager.lastSentTime = time.Now()
		clipboardManager.mu.Unlock()
		
		defer sendCopyNotification(log, c.GetString("clientName"), fmt.Sprintf("[%d File(s)]", len(responseFiles)))
		log.Info("════════════════════════════════════════")
		return
	}
	
	log.WithField("contentType", contentType).Error("❌ Unknown or unsupported clipboard content type")
	log.Info("Supported types:")
	log.Info("  - text: Plain text, URLs")
	log.Info("  - bitmap: Images, screenshots")
	log.Info("  - file: Files from Explorer")
	c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported clipboard content"})
	log.Info("════════════════════════════════════════")
}

func readBase64FromFile(path string) (string, error) {
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(fileBytes), nil
}

func getTruncatedText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

type TextBody struct {
	Text string `json:"data"`
	Hash string `json:"hash"`
}

func setHandler(c *gin.Context) {
	if !app.config.ReserveHistory {
		cleanTempFiles()
	}

	contentType := c.GetHeader("X-Content-Type")
	if contentType == utils.TypeText {
		setTextHandler(c)
		return
	}

	setFileHandler(c)
}

func setTextHandler(c *gin.Context) {
	var body TextBody
	if err := c.ShouldBindJSON(&body); err != nil {
		log.WithError(err).Warn("failed to bind text body")
		c.Status(http.StatusOK)
		return
	}

	textLength := len(body.Text)
	lineCount := strings.Count(body.Text, "\n") + 1
	receivedHash := body.Hash
	
	if receivedHash == "" {
		receivedHash = calculateHash([]byte(body.Text))
	}
	
	clipboardManager.mu.RLock()
	currentHash := clipboardManager.lastTextHash
	clipboardManager.mu.RUnlock()
	
	if receivedHash == currentHash && currentHash != "" {
		log.WithFields(logrus.Fields{
			"length": textLength,
			"hash":   receivedHash[:8] + "...",
		}).Info("📥 received duplicate text from iOS - skipping")
		c.Status(http.StatusOK)
		return
	}
	
	log.WithFields(logrus.Fields{
		"length": textLength,
		"lines":  lineCount,
		"hash":   receivedHash[:8] + "...",
		"ip":     c.ClientIP(),
	}).Info("📥 received text from iOS")
	
	// NEW: Log to history.txt
	preview := getTruncatedText(body.Text, 100)
	logToHistory("from_ios", "text", preview)

	c.Status(http.StatusOK)

	go func() {
		clipboardManager.InvalidateCache()
		
		// NEW: Mark this content as received FROM iOS to prevent bounce-back
		clipboardManager.mu.Lock()
		clipboardManager.lastReceivedHash = receivedHash
		clipboardManager.lastReceivedTime = time.Now()
		clipboardManager.lastSentHash = "" // Clear sent hash
		clipboardManager.mu.Unlock()
		
		log.WithFields(logrus.Fields{
			"receivedHash": receivedHash[:8] + "...",
			"textLength":   textLength,
		}).Info("🔒 Marked text as received FROM iOS (bounce-back prevention)")
		
		err := clipboardManager.SetText(body.Text)
		if err != nil {
			log.WithError(err).Error("SetText reported error")
			logError("CLIPBOARD_ERROR", "Failed to set text from iOS", err)
		} else {
			log.WithFields(logrus.Fields{
				"length": textLength,
				"lines":  lineCount,
				"hash":   receivedHash[:8] + "...",
			}).Info("✓ Text written to Windows clipboard")
		}
	}()

	var notify string = "Paste content is empty"
	if body.Text != "" {
		notify = getTruncatedText(body.Text, 50)
	}
	defer sendPasteNotification(log, c.GetString("clientName"), notify)
}

type FileBody struct {
	Files []File `json:"data"`
	Hash  string `json:"hash"`
}

type File struct {
	Name   string `json:"name"`
	Base64 string `json:"base64"`
	_bytes []byte `json:"-"`
}

func (f *File) Bytes() ([]byte, error) {
	if len(f._bytes) > 0 {
		return f._bytes, nil
	}
	fileBytes, err := base64.StdEncoding.DecodeString(f.Base64)
	if err != nil {
		return []byte{}, nil
	}
	f._bytes = fileBytes
	return fileBytes, nil
}

func setFileHandler(c *gin.Context) {
	contentType := c.GetHeader("X-Content-Type")

	var body FileBody
	if err := c.ShouldBindJSON(&body); err != nil {
		log.WithError(err).Warn("failed to bind file body")
		c.Status(http.StatusBadRequest)
		return
	}

	paths := make([]string, 0, len(body.Files))
	var allFilesContent []byte  // NEW: For content-based hash (filename + raw bytes)
	
	for _, file := range body.Files {
		if file.Name == "-" && file.Base64 == "-" {
			continue
		}
		path := utils.LatestFilename(app.GetTempFilePath(file.Name))
		fileBytes, err := file.Bytes()
		if err != nil {
			log.WithField("filename", file.Name).Warn("failed to read file bytes")
			continue
		}
		if err := newFile(path, fileBytes); err != nil {
			log.WithError(err).WithField("path", path).Warn("failed to create file")
			continue
		}
		paths = append(paths, path)
		
		// NEW: Add filename + raw bytes for content-based hash (same as GET handler)
		allFilesContent = append(allFilesContent, []byte(file.Name)...)
		allFilesContent = append(allFilesContent, fileBytes...)
	}
	
	// NEW: Calculate hash from file content only (not paths!)
	receivedHash := body.Hash
	if receivedHash == "" && len(allFilesContent) > 0 {
		receivedHash = calculateHash(allFilesContent)
	}
	
	clipboardManager.mu.RLock()
	currentHash := clipboardManager.lastFileHash
	clipboardManager.mu.RUnlock()
	
	if receivedHash == currentHash && currentHash != "" {
		log.WithFields(logrus.Fields{
			"fileCount": len(paths),
			"hash":      receivedHash[:8] + "...",
		}).Info("📥 received duplicate files from iOS - skipping")
		c.Status(http.StatusOK)
		return
	}
	
	// NEW: Log to history.txt
	fileNames := make([]string, 0, len(body.Files))
	for _, f := range body.Files {
		if f.Name != "-" {
			fileNames = append(fileNames, f.Name)
		}
	}
	fileList := strings.Join(fileNames, ", ")
	if len(fileList) > 100 {
		fileList = fileList[:97] + "..."
	}
	logToHistory("from_ios", "file", fmt.Sprintf("%d file(s): %s", len(fileNames), fileList))

	if app.config.ReserveHistory {
		setLastFilenames(nil)
	} else {
		setLastFilenames(paths)
	}

	c.Status(http.StatusOK)

	go func() {
		clipboardManager.InvalidateCache()
		
		// NEW: Mark these files as received FROM iOS to prevent bounce-back
		clipboardManager.mu.Lock()
		clipboardManager.lastReceivedHash = receivedHash
		clipboardManager.lastReceivedTime = time.Now()
		clipboardManager.lastSentHash = "" // Clear sent hash
		clipboardManager.mu.Unlock()
		
		log.WithFields(logrus.Fields{
			"receivedHash": receivedHash[:8] + "...",
			"fileCount":    len(paths),
		}).Info("🔒 Marked files as received FROM iOS (bounce-back prevention)")
		
		err := clipboardManager.SetFiles(paths)
		if err != nil {
			log.WithError(err).Error("failed to set clipboard files")
			logError("CLIPBOARD_ERROR", "Failed to set files from iOS", err)
		} else {
			log.WithFields(logrus.Fields{
				"paths": paths,
				"hash":  receivedHash[:8] + "...",
			}).Info("✓ Files written to Windows clipboard")
		}
	}()

	var notify string
	if contentType == utils.TypeMedia {
		notify = "[Media] Copied to clipboard"
	} else {
		notify = "[File] Copied to clipboard"
	}

	defer sendPasteNotification(log, c.GetString("clientName"), notify)
}

func notFoundHandler(c *gin.Context) {
	requestLogger := log.WithFields(logrus.Fields{"user_ip": c.Request.RemoteAddr})
	requestLogger.Info("404 not found")
	c.Status(http.StatusNotFound)
}

func sendCopyNotification(logger *logrus.Logger, client, notify string) {
	if app.config.Notify.Copy {
		sendNotification(logger, "copy", client, notify)
	}
}

func sendPasteNotification(logger *logrus.Logger, client, notify string) {
	if app.config.Notify.Paste {
		sendNotification(logger, "Paste", client, notify)
	}
}

func sendNotification(logger *logrus.Logger, action, client, notify string) {
	if notify == "" {
		notify = action + "Content is empty"
	}
	title := fmt.Sprintf("%s自 %s", action, client)
	if err := app.ni.ShowInfo(title, notify); err != nil {
		logger.WithError(err).WithField("notify", notify).Warn("failed to send notification")
	}
}

func setLastFilenames(filenames []string) {
	path := app.GetTempFilePath("_filename.txt")
	allFilenames := strings.Join(filenames, "\n")
	_ = ioutil.WriteFile(path, []byte(allFilenames), os.ModePerm)
}

func newFile(path string, bytes []byte) error {
	return ioutil.WriteFile(path, bytes, 0644)
}

func cleanTempFiles() {
	path := app.GetTempFilePath("_filename.txt")
	if utils.IsExistFile(path) {
		file, err := os.Open(path)
		if err != nil {
			log.WithError(err).WithField("path", path).Warn("failed to open temp file")
			return
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			delPath := scanner.Text()
			// Check if file exists before trying to delete
			if !utils.IsExistFile(delPath) {
				log.WithField("delPath", delPath).Debug("temp file already deleted, skipping")
				continue
			}
			if err = os.Remove(delPath); err != nil {
				// Only warn if file exists but can't be deleted (permission issue, etc.)
				log.WithError(err).WithField("delPath", delPath).Debug("failed to delete temp file")
			}
		}
	}
}