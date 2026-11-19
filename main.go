package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const serverAddr = "127.0.0.1:17395"

//go:embed web/index.html web/inject.js web/logo.png
var webFS embed.FS

var (
	injectedControllerJS string
	appStateMu           sync.Mutex
	hiddenToTray         bool
	exitTimer            *time.Timer
)

func setHiddenToTray(v bool) {
	appStateMu.Lock()
	defer appStateMu.Unlock()
	hiddenToTray = v
	if exitTimer != nil {
		exitTimer.Stop()
		exitTimer = nil
	}
}

func isHiddenToTray() bool {
	appStateMu.Lock()
	defer appStateMu.Unlock()
	return hiddenToTray
}

func cancelScheduledExit() {
	appStateMu.Lock()
	defer appStateMu.Unlock()
	if exitTimer != nil {
		exitTimer.Stop()
		exitTimer = nil
	}
}

func scheduleExitIfNotHidden() {
	appStateMu.Lock()
	defer appStateMu.Unlock()
	if hiddenToTray {
		return
	}
	if exitTimer != nil {
		exitTimer.Stop()
	}
	exitTimer = time.AfterFunc(1400*time.Millisecond, func() {
		if !isHiddenToTray() {
			shutdownAppCompletely()
		}
	})
}

func shutdownAppCompletely() {
	_, _ = evalInDiscord("window.__questTrackerV9 && window.__questTrackerV9.stop()")
	removeTrayIcon()
	os.Exit(0)
}

func openUIWindow(url string) {
	browsers := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, b := range browsers {
		if _, err := os.Stat(b); err == nil {
			if exec.Command(b, "--app="+url, "--window-size=1220,780").Start() == nil {
				return
			}
		}
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func wakeExistingInstance() bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Post("http://"+serverAddr+"/api/show", "application/json", nil)
	if err == nil {
		_ = resp.Body.Close()
		return true
	}
	return false
}

func main() {
	if wakeExistingInstance() {
		return
	}

	jsBytes, err := fs.ReadFile(webFS, "web/inject.js")
	if err != nil {
		fmt.Println("Read error:", err)
		return
	}
	injectedControllerJS = string(jsBytes)

	logoBytes, _ := fs.ReadFile(webFS, "web/logo.png")
	startSystemTray(logoBytes)

	if !isDiscordRunning() && findDiscordExe() != "" {
		_ = launchOrRestartDiscordWithDebug()
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		cancelScheduledExit()
		htmlBytes, err := fs.ReadFile(webFS, "web/index.html")
		if err != nil {
			http.Error(w, "UI not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(htmlBytes)
	})

	http.HandleFunc("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(logoBytes)
	})

	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/start", handleAction)
	http.HandleFunc("/api/start-one", handleAction)
	http.HandleFunc("/api/stop", handleAction)
	http.HandleFunc("/api/enroll-all", handleAction)
	http.HandleFunc("/api/settings", handleAction)
	http.HandleFunc("/api/connect", handleAction)
	http.HandleFunc("/api/hide-tray", handleAction)
	http.HandleFunc("/api/quit", handleAction)
	http.HandleFunc("/api/window-closed", handleAction)
	http.HandleFunc("/api/show", handleAction)

	go func() {
		time.Sleep(350 * time.Millisecond)
		openUIWindow("http://" + serverAddr)
	}()

	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		fmt.Println("Server error:", err)
	}
}
