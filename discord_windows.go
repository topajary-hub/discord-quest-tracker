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
	discordDir := filepath.Join(localAppData, "Discord")
	matches, err := filepath.Glob(filepath.Join(discordDir, "app-*", "Discord.exe"))
	if err == nil && len(matches) > 0 {
		sort.Strings(matches)
		return matches[len(matches)-1]
	}
	updateExe := filepath.Join(discordDir, "Update.exe")
	if _, err := os.Stat(updateExe); err == nil {
		return updateExe
	}
	return ""
}

func isDiscordRunning() bool {
	cmd := hiddenCmd("tasklist", "/FI", "IMAGENAME eq Discord.exe", "/NH")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(out.String()), "discord.exe")
}

func launchOrRestartDiscordWithDebug() error {
	exe := findDiscordExe()
	if exe == "" {
		return fmt.Errorf("Discord не найден в %%LOCALAPPDATA%%\\Discord")
	}

	if isDiscordRunning() {
		_ = hiddenCmd("taskkill", "/F", "/IM", "Discord.exe").Run()
		time.Sleep(1500 * time.Millisecond)
	}

	if strings.HasSuffix(strings.ToLower(exe), "update.exe") {
		return hiddenCmd(exe, "--processStart", "Discord.exe", "--process-start-args=--remote-debugging-port="+cdpPort).Start()
	}

	return hiddenCmd(exe, "--remote-debugging-port="+cdpPort).Start()
}

func getDiscordWSTarget() (string, error) {
	client := &http.Client{Timeout: 1200 * time.Millisecond}
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

	for _, t := range targets {
		if t.Type == "page" && (strings.Contains(t.URL, "discord.com") || strings.Contains(t.URL, "discordapp.com")) {
			return t.WebSocketDebuggerURL, nil
		}
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

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
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