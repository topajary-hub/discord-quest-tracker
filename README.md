# Discord Quest Tracker (Kastrix)

Десктопное приложение для Windows на Go для автоматического принятия, отслеживания и выполнения заданий Discord Quests.

## Возможности
- Автоматическое подключение к клиенту Discord через Chrome DevTools Protocol (CDP)
- Отслеживание баланса **Discord Orbs**, заработанных монет за сессию и прогресса по каждому заданию
- Автопринятие новых квестов и последовательное выполнение очереди (`WATCH_VIDEO`, `PLAY_ON_DESKTOP`, `STREAM_ON_DESKTOP`, `PLAY_ACTIVITY`, `WATCH_VIDEO_ON_MOBILE`)
- Сворачивание в системный трей для работы в фоне и быстрый запуск одиночных заданий
- Тёмная и светлая темы интерфейса

## Сборка из исходников

```bash
go mod tidy
go run gen_icon.go
go build -ldflags="-H windowsgui" -o QuestTracker.exe .