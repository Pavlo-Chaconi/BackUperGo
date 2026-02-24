# BackUper

Архитектура разделена на **агента** и **сервер**.
Агент работает на машинах с данными, создаёт архив локально и отправляет на сервер.
Сервер принимает архивы, хранит их и предоставляет web‑панель.

## Состав

### Агент
- `cmd/agent` — агент (polling, отправка)
- `internal/agent` — логика polling + буфер событий (AppData)
- `internal/sender` — сбор файлов, архивирование, отправка
- `internal/archive` — ZIP + SHA256
- `internal/transport` — length‑prefixed JSON по TCP

### Сервер
- `cmd/server` — приёмник (TCP receiver)
- `cmd/web` — web‑панель и API (заглушки)
- `internal/receiver` — приём архива, проверка SHA256, запись в хранилище

### Конфиг
- `internal/config/config.go` — единый JSON‑конфиг (agent/server поля)

## Установка агента (Windows)

### Inno Setup
Скрипт: `scripts/installer/BackUperAgent.iss`

Сборка:
```
iscc /DSourceExe="C:\path\to\backuper-agent.exe" /DAppVersion="0.1.0" scripts\installer\BackUperAgent.iss
```

### PowerShell (быстрая установка)
```
powershell -ExecutionPolicy Bypass -File scripts\install-agent.ps1
```

## Логи
Локальные `log.Printf` для агента убраны.
Все события отправляются на сервер, а при недоступности пишутся в буфер:
`%APPDATA%\BackUperAgent\events.log`

## Планы
- [ ] Реальная БД для задач/логов
- [ ] Полноценная web‑панель (дашборды, настройки, расписание)
- [ ] Уведомления (Telegram)
