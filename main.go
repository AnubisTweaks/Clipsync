package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/AnubisTweaks/Clipsync/action"
	"github.com/AnubisTweaks/Clipsync/utils"
	"github.com/gin-gonic/gin"
	"github.com/lxn/walk"
	"github.com/sirupsen/logrus"
)

var app *Application

var execPath string
var execFullPath string
var config *Config
var mode string = "debug"
var version string = ""
var log = logrus.New()
var startTime time.Time

func init() {
	execFullPath = os.Args[0]
	execPath = filepath.Dir(execFullPath)

	var err error
	configFilePath := filepath.Join(execPath, ConfigFile)
	config, err = loadConfig(configFilePath)
	if err != nil {
		log.WithError(err).Warn("failed to load config")
	}
	
	// Set to Warning level - only errors and critical issues
	log.SetLevel(logrus.WarnLevel)

	// Always log to file
	logFilePath := filepath.Join(execPath, LogFile)
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.WithError(err).Fatal("failed to open log file")
	}
	log.SetOutput(f)
	
	if mode != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
}

// logAppStart logs application startup to history.txt
func logAppStart() {
	startTime = time.Now()
	historyPath := filepath.Join(execPath, "history.txt")
	f, err := os.OpenFile(historyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.WithError(err).Error("failed to open history.txt")
		return
	}
	defer f.Close()
	
	f.WriteString(fmt.Sprintf("\n════════════════════════════════════════\n"))
	f.WriteString(fmt.Sprintf("🚀 APP STARTED: %s\n", startTime.Format("2006-01-02 15:04:05")))
	f.WriteString(fmt.Sprintf("Version: %s | Port: %s\n", version, config.Port))
	f.WriteString(fmt.Sprintf("════════════════════════════════════════\n"))
}

// logAppStop logs clean application shutdown to history.txt
func logAppStop() {
	if startTime.IsZero() {
		return
	}
	
	historyPath := filepath.Join(execPath, "history.txt")
	f, err := os.OpenFile(historyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	
	uptime := time.Since(startTime)
	f.WriteString(fmt.Sprintf("✓ APP STOPPED: %s (uptime: %s)\n", 
		time.Now().Format("2006-01-02 15:04:05"), 
		uptime.Round(time.Second)))
	f.WriteString(fmt.Sprintf("════════════════════════════════════════\n\n"))
}

// logCrash logs crash information to both history.txt and crash.txt
func logCrash(panicValue interface{}) {
	crashTime := time.Now()
	stackTrace := string(debug.Stack())
	
	// Log to history.txt
	historyPath := filepath.Join(execPath, "history.txt")
	if f, err := os.OpenFile(historyPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		defer f.Close()
		uptime := crashTime.Sub(startTime)
		f.WriteString(fmt.Sprintf("💥 APP CRASHED: %s (uptime: %s)\n", 
			crashTime.Format("2006-01-02 15:04:05"),
			uptime.Round(time.Second)))
		f.WriteString(fmt.Sprintf("Error: %v\n", panicValue))
		f.WriteString(fmt.Sprintf("See crash.txt for full details\n"))
		f.WriteString(fmt.Sprintf("════════════════════════════════════════\n\n"))
	}
	
	// Create detailed crash dump in crash.txt
	crashPath := filepath.Join(execPath, "crash.txt")
	if f, err := os.OpenFile(crashPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("\n╔════════════════════════════════════════════════════════════╗\n"))
		f.WriteString(fmt.Sprintf("║ CRASH REPORT - %s\n", crashTime.Format("2006-01-02 15:04:05")))
		f.WriteString(fmt.Sprintf("╚════════════════════════════════════════════════════════════╝\n\n"))
		f.WriteString(fmt.Sprintf("Started: %s\n", startTime.Format("2006-01-02 15:04:05")))
		f.WriteString(fmt.Sprintf("Crashed: %s\n", crashTime.Format("2006-01-02 15:04:05")))
		f.WriteString(fmt.Sprintf("Uptime: %s\n\n", crashTime.Sub(startTime).Round(time.Second)))
		f.WriteString(fmt.Sprintf("PANIC: %v\n\n", panicValue))
		f.WriteString(fmt.Sprintf("STACK TRACE:\n%s\n", stackTrace))
		f.WriteString(fmt.Sprintf("════════════════════════════════════════════════════════════\n\n"))
	}
	
	// Log to main log file
	log.WithFields(logrus.Fields{
		"panic":  panicValue,
		"uptime": crashTime.Sub(startTime).Round(time.Second),
	}).Fatal("💥 APPLICATION CRASHED")
}

func main() {
	// Global panic recovery - catch ALL panics
	defer func() {
		if r := recover(); r != nil {
			logCrash(r)
			// Show error to user before exiting
			if app != nil && app.ni != nil {
				app.ni.ShowError("ClipSync Crashed", 
					fmt.Sprintf("The application has crashed unexpectedly.\n\nError: %v\n\nCheck crash.txt for details.", r))
			}
			time.Sleep(3 * time.Second) // Give user time to see the error
			os.Exit(1)
		}
	}()
	
	var err error

	// Log app startup
	logAppStart()
	defer logAppStop()

	app, err = NewApplication(config)
	if err != nil {
		log.WithError(err).Fatal("failed to create applicaton")
	}
	defer app.BeforeExit()

	if err := utils.CreateDirectory(app.GetTempFilePath("")); err != nil {
		log.WithError(err).Fatal("failed to create temp directory")
	}

	icon, err := walk.NewIconFromResourceId(2)
	if err != nil {
		log.WithError(err).Fatal("failed to get icon")
	}

	if err := app.ni.SetIcon(icon); err != nil {
		log.WithError(err).Fatal("failed to set icon")
	}

	if err := app.ni.SetToolTip("ClipSync" + version + " :" + config.Port); err != nil {
		log.WithError(err).Fatal("failed to set tooltip")
	}

	autoRunAction, err := action.NewAutoRunAction()
	if err != nil {
		log.WithError(err).Fatal("failed to create AutoRunAction")
	}
	soundAction, err := action.NewSoundAction(app.IsSoundEnabled, app.SetSoundEnabled)
	if err != nil {
		log.WithError(err).Fatal("failed to create SoundAction")
	}
	exitAction, err := action.NewExitAction()
	if err != nil {
		log.WithError(err).Fatal("failed to create ExitAction")
	}
	if err := app.AddActions(autoRunAction, soundAction, exitAction); err != nil {
		log.WithError(err).Fatal("failed to add action")
	}

	if err := app.ni.SetVisible(true); err != nil {
		log.WithError(err).Fatal("failed to set notify visible")
	}

	// Initialize clipboard manager BEFORE starting HTTP server
	log.Info("initializing clipboard manager")
	initClipboardManager()

	log.Info("start http server")
	app.RunHTTPServer()
	log.Info("app started successfully")
	app.Run()
}