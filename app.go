package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lxn/walk"
)

type Application struct {
	config       *Config
	soundEnabled bool
	soundMutex   sync.RWMutex
	*walk.MainWindow
	ni *walk.NotifyIcon
	wg sync.WaitGroup
}

func (app *Application) RunHTTPServer() {
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.WithField("panic", r).Error("💥 HTTP server panic recovered")
				// Log to crash file
				crashPath := filepath.Join(execPath, "crash.txt")
				if f, err := os.OpenFile(crashPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
					defer f.Close()
					f.WriteString(fmt.Sprintf("\n[%s] HTTP Server Panic: %v\n", 
						time.Now().Format("2006-01-02 15:04:05"), r))
				}
			}
		}()
		
		engin := gin.New()
		setupRoute(engin)
		if err := engin.Run(":" + app.config.Port); err != nil {
			app.ni.ShowError("HTTP Server Error", 
				fmt.Sprintf("Server failed to start on port %s\n\nError: %v\n\nCheck if port is already in use.", app.config.Port, err))
			log.WithError(err).Error("❌ HTTP server failed to start")
			// Log to history for critical errors
			historyPath := filepath.Join(execPath, "history.txt")
			f, _ := os.OpenFile(historyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if f != nil {
				defer f.Close()
				f.WriteString(fmt.Sprintf("[%s] ❌ HTTP Server failed: %v\n", 
					time.Now().Format("2006-01-02 15:04:05"), err))
			}
			// Don't exit the app - just log the error
			// User can still use clipboard monitoring even if HTTP server fails
			return
		}
	}()
}

func (app *Application) StopHTTPServer() {
	// Don't call Done() here - it's called in the goroutine's defer
}

func (app *Application) BeforeExit() {
	app.StopHTTPServer()
	app.ni.Dispose()
}

func (app *Application) AddActions(actions ...*walk.Action) error {
	for _, action := range actions {
		if err := app.ni.ContextMenu().Actions().Add(action); err != nil {
			return err
		}
	}
	return nil
}

func (app *Application) GetTempFilePath(filename string) string {
	if !filepath.IsAbs(app.config.TempDir) {
		// temp files path in exec path but not pwd
		tempAbsPath := path.Join(execPath, app.config.TempDir)
		return filepath.Join(tempAbsPath, filename)
	}
	return filepath.Join(app.config.TempDir, filename)
}

func (app *Application) IsSoundEnabled() bool {
	app.soundMutex.RLock()
	defer app.soundMutex.RUnlock()
	return app.soundEnabled
}

func (app *Application) SetSoundEnabled(enabled bool) {
	app.soundMutex.Lock()
	defer app.soundMutex.Unlock()
	app.soundEnabled = enabled
	app.config.Sound.Enabled = enabled
	
	// Save config with new sound state
	app.saveConfig()
}

func (app *Application) saveConfig() {
	// This will be called to persist the sound setting
	configPath := filepath.Join(execPath, ConfigFile)
	configBytes, err := json.MarshalIndent(app.config, "", "  ")
	if err != nil {
		log.WithError(err).Error("failed to marshal config")
		return
	}
	
	if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
		log.WithError(err).Error("failed to save config")
	}
}

func NewApplication(config *Config) (*Application, error) {
	app := new(Application)
	var err error
	app.config = config
	app.soundEnabled = config.Sound.Enabled
	app.MainWindow, err = walk.NewMainWindow()
	if err != nil {
		return nil, err
	}

	app.ni, err = walk.NewNotifyIcon(app.MainWindow)
	if err != nil {
		return nil, err
	}

	return app, nil
}
