<div align="center">

<img src="web/logo.png" alt="Discord Quest Tracker Logo" width="88" height="88" />

# Discord Quest Tracker

**Automated desktop manager and tracker for Discord Quests & Orbs on Windows**

[![Platform](https://img.shields.io/badge/Platform-Windows%2010%2F11%20(x64)-0078D6?style=flat-square&logo=windows)](https://github.com/topajary-hub/discord-quest-tracker/releases)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev/)
[![Project](https://img.shields.io/badge/Project-Kastrix%20Product-5865F2?style=flat-square&logo=telegram&logoColor=white)](https://t.me/kastrix_mod)
[![Author](https://img.shields.io/badge/Author-topajary--hub-181717?style=flat-square&logo=github)](https://github.com/topajary-hub)

**English** | [Русский](README_RU.md)

[Download Release](../../releases) • [Interface](#interface) • [Features](#features) • [Building from Source](#building-from-source) • [Author & Project](#author--project)

</div>

---

## About the Project

**Discord Quest Tracker** is a lightweight Windows desktop application built with **Go** and a modern **React + Framer Motion** interface. It connects directly to the official Discord desktop client, tracks all available quests alongside your **Discord Orbs** balance, and automates quest completion in the background.

The entire web UI, scripts, and assets are embedded into a single standalone executable (`QuestTracker.exe`) — no Python, Node.js, or external dependencies required.

## Interface

<img width="1919" height="1079" alt="image" src="https://github.com/user-attachments/assets/ff24b377-14ee-44bd-aacb-908ca1266090" />


---

## Features

- **Automatic Discord Connection:** Automatically locates your Discord installation, launches it with the debugging port enabled, or restarts an active session in one click without logging you out.
- **Auto-Enroll & Auto-Start:** Continuously monitors for new quests, automatically accepts available tasks, and queues them for completion.
- **Smart Queue & Single-Quest Mode:** Run all eligible quests sequentially (prioritizing fast video quests first) or trigger individual tasks on demand.
- **Discord Orbs Tracking:** Real-time display of your current wallet balance, Orbs earned during the active session, and total rewards available across pending quests.
- **System Tray Integration:** Choose whether to completely exit the app or minimize it to the Windows system tray to keep quests running silently in the background.
- **Single-Instance Protection:** Launching `QuestTracker.exe` while it is already minimized brings the existing window back into focus instead of spawning duplicate processes.
- **Responsive Modern UI:** Dark and light themes, collapsible sidebar (`Ctrl + B`), instant search, multi-criteria sorting, and a live event log.

---

## Supported Quest Types

| Discord Task Type | Tracker Label | Execution Mode | Notes |
| :--- | :--- | :---: | :--- |
| `WATCH_VIDEO` | Video | Automatic | Simulates video playback progress respecting server-side timestamp rules |
| `WATCH_VIDEO_ON_MOBILE` | Video (Mobile) | Automatic | Completes directly from desktop without needing a mobile device |
| `PLAY_ON_DESKTOP` | Desktop Game | Automatic | Spoofs the running game process without downloading or installing the title |
| `PLAY_ACTIVITY` | Activity | Automatic | Sends activity heartbeat requests to a voice or DM channel |
| `STREAM_ON_DESKTOP` | Voice Stream | Semi-Auto | Spoofs streamed game metadata; requires screen sharing in a voice channel with 1+ other user |
| `ACHIEVEMENT_IN_ACTIVITY` | Manual in Activity | Manual | Progress is validated by the third-party activity server; automatically skipped in the main queue |

---

## Quick Start

1. Go to the **[Releases](../../releases)** page and download the latest **`QuestTracker.exe`**.
2. Place `QuestTracker.exe` in any folder and run it.
3. If Discord was already running beforehand, click **Connect Discord** in the top-right corner to restart the client with debugging enabled.
4. Click **Start All** or select a specific quest from the list.

> **Windows SmartScreen Notice:**  
> Because the binary is not signed with a commercial code-signing certificate, Windows Defender SmartScreen may display a warning on first launch. Click **More info → Run anyway** to proceed. Microsoft Edge (pre-installed on Windows 10/11) or Google Chrome is required to render the application window.

---

## How It Works

1. **Local Server & App Window:** On startup, Go spins up a local HTTP server on `127.0.0.1:17395` and opens the UI in standalone app mode (`--app`) without browser tabs or address bars.
2. **Chrome DevTools Protocol (CDP):** The backend communicates with the Discord desktop client via WebSocket on `127.0.0.1:9223` (`--remote-debugging-port=9223`).
3. **Client Store Integration:** The injected controller interfaces with Discord's internal stores (`QuestsStore`, `RunningGameStore`, `ApplicationStreamingStore`, `VirtualCurrencyStore`, `FluxDispatcher`) and cleanly restores original client state upon stopping or exiting.
4. **Native Windows Integration:** System tray icon (`Shell_NotifyIconW`), context menus, and background process checks are implemented directly via WinAPI (`syscall`) with zero console window flicker.

### Repository Structure

```text
├── main.go                  # Entry point, embedded asset server, window & lifecycle management
├── discord_windows.go       # Discord client discovery, debug launch & CDP WebSocket client
├── handlers.go              # REST API endpoints connecting the UI to the backend
├── tray_windows.go          # Native Windows system tray & context menu via WinAPI
├── rsrc_windows_amd64.syso  # Windows application icon resource
├── go.mod                   # Go module dependencies
├── .gitignore               # Git ignore rules
└── web/
    ├── index.html           # Frontend UI (React 18 + Tailwind CSS + Framer Motion + React-Toastify)
    ├── inject.js            # Quest automation & state parsing controller
    └── logo.png             # Application icon
```

---

## Building from Source

Requires **Go 1.21+** and **Git** on Windows:

```bash
git clone [https://github.com/topajary-hub/discord-quest-tracker.git](https://github.com/topajary-hub/discord-quest-tracker.git)
cd discord-quest-tracker
go mod tidy
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H windowsgui" -o QuestTracker.exe .
```

---

## FAQ

**Why doesn't the stream quest (`STREAM_ON_DESKTOP`) progress automatically?**  
Discord's backend requires an active WebRTC media stream inside a voice channel with at least one other participant present. The tracker spoofs the streamed game metadata — you only need to join a voice channel and share any window.

**Why are some quests labeled as "Manual in Activity"?**  
Quests of type `ACHIEVEMENT_IN_ACTIVITY` verify specific in-game milestones on the activity developer's own servers. The tracker automatically skips these quests during queue execution so they do not block other tasks.

**How do I completely close the application?**  
Click the window close button and select **Close completely** (or right-click the system tray icon → **Exit**). The app will stop all active quest hooks in Discord and terminate its process cleanly.

---

## Author & Project

- **Project:** [Kastrix Product (Telegram)](https://t.me/kastrix_mod)
- **Developer:** Akim
- **GitHub:** [topajary-hub](https://github.com/topajary-hub)
- **Discord:** `FunTrue` (`! тома джери`)

---

<div align="center">

If this tool saved you time and helped you farm Discord Orbs, consider giving the repository a **⭐ Star**!

</div>
