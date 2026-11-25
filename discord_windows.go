package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

const cdpPort = "9223"

var cdpMu sync.Mutex

type cdpTarget struct {
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type evalRequest struct {
	ID     int                    `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type evalResponse struct {
	ID     int `json:"id"`
	Result struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	} `json:"result"`
}

func hiddenCmd(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func findDiscordExe() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return ""
	}
	variants := []struct {
		dir string
		exe string
	}{
		{"Discord", "Discord.exe"},
		{"DiscordPTB", "DiscordPTB.exe"},
		{"DiscordCanary", "DiscordCanary.exe"},
	}
	for _, v := range variants {
		discordDir := filepath.Join(localAppData, v.dir)
		matches, err := filepath.Glob(filepath.Join(discordDir, "app-*", v.exe))
		if err == nil && len(matches) > 0 {
			sort.Strings(matches)
			return matches[len(matches)-1]
		}
		updateExe := filepath.Join(discordDir, "Update.exe")
		if _, err := os.Stat(updateExe); err == nil {
			return updateExe
		}
	}
	return ""
}

func isDiscordRunning() bool {
	for _, proc := range []string{"Discord.exe", "DiscordPTB.exe", "DiscordCanary.exe"} {
		cmd := hiddenCmd("tasklist", "/FI", "IMAGENAME eq "+proc, "/NH")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil && strings.Contains(strings.ToLower(out.String()), strings.ToLower(proc)) {
			return true
		}
	}
	return false
}

func getDiscordWSTarget() (string, error) {
	client := &http.Client{Timeout: 900 * time.Millisecond}
	resp, err := client.Get("http://127.0.0.1:" + cdpPort + "/json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return "", err
	}

	var fallbackWS string
	for _, t := range targets {
		if t.Type != "page" || t.WebSocketDebuggerURL == "" {
			continue
		}
		u := strings.ToLower(t.URL)
		if strings.Contains(u, "updater") || strings.Contains(u, "splash") {
			continue
		}
		if strings.Contains(u, "discord.com") || strings.Contains(u, "discordapp.com") {
			if strings.Contains(u, "/channels/") || strings.Contains(u, "/app") {
				return t.WebSocketDebuggerURL, nil
			}
			if fallbackWS == "" {
				fallbackWS = t.WebSocketDebuggerURL
			}
		}
	}
	if fallbackWS != "" {
		return fallbackWS, nil
	}
	return "", fmt.Errorf("главное окно Discord еще загружается")
}

func evalInDiscord(expression string) (string, error) {
	cdpMu.Lock()
	defer cdpMu.Unlock()

	wsURL, err := getDiscordWSTarget()
	if err != nil {
		return "", err
	}

	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	req := evalRequest{
		ID:     1,
		Method: "Runtime.evaluate",
		Params: map[string]interface{}{
			"expression":    expression,
			"returnByValue": true,
			"awaitPromise":  true,
		},
	}

	if err := conn.WriteJSON(req); err != nil {
		return "", err
	}

	_ = conn.SetReadDeadline(time.Now().Add(12 * time.Second))
	var evalResp evalResponse
	if err := conn.ReadJSON(&evalResp); err != nil {
		return "", err
	}

	if evalResp.Result.ExceptionDetails != nil {
		return "", fmt.Errorf("JS exception: %s", evalResp.Result.ExceptionDetails.Text)
	}

	var strVal string
	if err := json.Unmarshal(evalResp.Result.Result.Value, &strVal); err == nil {
		return strVal, nil
	}
	return string(evalResp.Result.Result.Value), nil
}

func launchOrRestartDiscordWithDebug() error {
	if _, err := getDiscordWSTarget(); err == nil {
		return nil
	}

	exe := findDiscordExe()
	if exe == "" {
		return fmt.Errorf("Discord не найден в %%LOCALAPPDATA%%\\Discord")
	}

	if isDiscordRunning() {
		for _, proc := range []string{"Discord.exe", "DiscordPTB.exe", "DiscordCanary.exe"} {
			_ = hiddenCmd("taskkill", "/F", "/IM", proc, "/T").Run()
		}
		waitKill := time.Now().Add(4 * time.Second)
		for time.Now().Before(waitKill) && isDiscordRunning() {
			time.Sleep(200 * time.Millisecond)
		}
		time.Sleep(400 * time.Millisecond)
	}

	var startErr error
	if strings.HasSuffix(strings.ToLower(exe), "update.exe") {
		procName := "Discord.exe"
		lowerPath := strings.ToLower(exe)
		if strings.Contains(lowerPath, "discordptb") {
			procName = "DiscordPTB.exe"
		} else if strings.Contains(lowerPath, "discordcanary") {
			procName = "DiscordCanary.exe"
		}
		startErr = hiddenCmd(exe, "--processStart", procName, "--process-start-args=--remote-debugging-port="+cdpPort).Start()
	} else {
		startErr = hiddenCmd(exe, "--remote-debugging-port="+cdpPort).Start()
	}
	if startErr != nil {
		return startErr
	}

	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := getDiscordWSTarget(); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}
