package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func handleStatus(w http.ResponseWriter, r *http.Request) {
	cancelScheduledExit()

	resp := map[string]interface{}{
		"discordInstalled": true,
		"discordRunning":   true,
		"cdpConnected":     false,
		"tracker":          nil,
		"error":            "",
	}

	raw, err := evalInDiscord(injectedControllerJS)
	if err == nil && raw != "" {
		var trackerData interface{}
		jsonErr := json.Unmarshal([]byte(raw), &trackerData)
		if jsonErr == nil {
			resp["cdpConnected"] = true
			resp["tracker"] = trackerData
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		resp["error"] = jsonErr.Error()
	} else if err != nil {
		resp["error"] = err.Error()
	}

	resp["discordInstalled"] = findDiscordExe() != ""
	resp["discordRunning"] = isDiscordRunning()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	switch r.URL.Path {
	case "/api/connect":
		if err := launchOrRestartDiscordWithDebug(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
		return

	case "/api/show":
		setHiddenToTray(false)
		go openUIWindow("http://" + serverAddr)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
		return

	case "/api/hide-tray":
		setHiddenToTray(true)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
		return

	case "/api/window-closed":
		scheduleExitIfNotHidden()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
		return

	case "/api/quit":
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
		go shutdownAppCompletely()
		return
	}

	var callExpr string
	switch r.URL.Path {
	case "/api/start":
		callExpr = "window.__questTrackerV9.startAll()"
	case "/api/start-one":
		var body struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		safeID, _ := json.Marshal(body.ID)
		callExpr = fmt.Sprintf("window.__questTrackerV9.startOne(%s)", string(safeID))
	case "/api/stop":
		callExpr = "window.__questTrackerV9.stop()"
	case "/api/enroll-all":
		callExpr = "window.__questTrackerV9.enrollNow()"
	case "/api/settings":
		var body struct {
			AutoEnroll *bool `json:"autoEnroll"`
			AutoStart  *bool `json:"autoStart"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		ae := "null"
		as := "null"
		if body.AutoEnroll != nil {
			if *body.AutoEnroll {
				ae = "true"
			} else {
				ae = "false"
			}
		}
		if body.AutoStart != nil {
			if *body.AutoStart {
				as = "true"
			} else {
				as = "false"
			}
		}
		callExpr = fmt.Sprintf("window.__questTrackerV9.setSettings(%s, %s)", ae, as)
	default:
		http.Error(w, "Unknown endpoint", http.StatusNotFound)
		return
	}

	fullJS := fmt.Sprintf("(function(){ if(!window.__questTrackerV9){ %s; } return %s; })()", injectedControllerJS, callExpr)

	raw, err := evalInDiscord(fullJS)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var trackerData interface{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &trackerData)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"tracker": trackerData,
	})
}
