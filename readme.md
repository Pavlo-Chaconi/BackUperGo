# BackUper

Система централизованного резервного копирования с архитектурой **агент-сервер**.

- **Агент** (Windows) — сканирует файлы, создаёт ZIP-архивы с SHA256, отправляет на сервер
- **Receiver** — TCP-приёмник архивов, проверяет целостность, сохраняет в хранилище
- **Web-панель** — управление агентами, генерация скриптов установки, мониторинг

---

## 📦 Быстрый старт (Docker)

### 1. Требования

- Docker и Docker Compose
- Go 1.24+ (для сборки агента)

### 2. Сборка агента

```bash
# Сборка агента для Windows
GOOS=windows GOARCH=amd64 go build -o dist/backuper-agent.exe ./cmd/agent
```

### 3. Запуск сервера

```bash
# Опционально: задать API-ключ для проверки агентов
export BACKUPER_API_KEY="my-secret-key-123"

# Запуск всех сервисов
docker-compose up -d
```

**Порты:**
- `8080` — Web-панель
- `9000` — TCP-приёмник (агенты)
- `5432` — PostgreSQL

**Проверка:**
```bash
docker-compose ps
curl http://localhost:8080/login
```

---

## 🖥️ Установка агента (Windows)

### Вариант 1: PowerShell скрипт (через Web-панель)

1. Откройте **Settings** в Web-панели (`http://localhost:8080/settings`)
2. Нажмите **"Сгенерировать скрипт"**
3. Скопируйте скрипт и выполните на машине с данными:

```powershell
powershell -ExecutionPolicy Bypass -File install.ps1
```

### Вариант 2: Ручная установка

1. Скопируйте `backuper-agent.exe` на целевую машину
2. Создайте конфиг в `%APPDATA%\BackUperAgent\config.json`:

```json
{
  "home_dir": "C:\\Data\\Backups",
  "schedule_time": "03:00",
  "temp_archive_dir": "C:\\Temp\\BackUper",
  "server_addr": "192.168.1.100:9000",
  "api_key": "my-secret-key-123",
  "agent_id": "",
  "poll_interval_seconds": 60,
  "api_url": "http://192.168.1.100:8080",
  "event_buffer_path": "%APPDATA%\\BackUperAgent\\events.log"
}
```

3. Запустите агента:
```powershell
.\backuper-agent.exe
```

### Вариант 3: Inno Setup Installer

Сборка установщика:
```batch
iscc /DSourceExe="dist\backuper-agent.exe" /DAppVersion="0.1.0" scripts\installer\BackUperAgent.iss
```

Запустите полученный `BackUperAgentSetup.exe` на целевой машине.

### Вариант 4: Как Windows-сервис

```powershell
# Установка сервиса
sc.exe create BackUperAgent binPath= "C:\Program Files\BackUperAgent\backuper-agent.exe --service" start= auto DisplayName= "BackUper Agent"
sc.exe start BackUperAgent
```

---

## ⚙️ Конфигурация

### Переменные окружения (сервер)

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| `DATABASE_URL` | Строка подключения к PostgreSQL | (обязательно) |
| `BACKUPER_SERVER_ADDR` | Адрес TCP-приёмника | `localhost:9000` |
| `BACKUPER_API_URL` | URL Web-панели | определяется автоматически |
| `BACKUPER_AGENT_BINARY` | Путь к `backuper-agent.exe` | `backuper-agent.exe` |
| `BACKUPER_API_KEY` | Ключ авторизации агентов | (не задан) |
| `BACKUPER_STORAGE_DIR` | Папка для хранения архивов | `C:\BackUper\backups` |

### Конфиг агента (`config.json`)

| Поле | Описание |
|------|----------|
| `home_dir` | Папка с данными для архивации |
| `schedule_time` | Время запуска (HH:MM) |
| `temp_archive_dir` | Временная папка для архивов |
| `server_addr` | Адрес receiver (host:port) |
| `api_key` | Ключ авторизации (должен совпадать с `BACKUPER_API_KEY`) |
| `agent_id` | Уникальный ID (автогенерация при enroll) |
| `poll_interval_seconds` | Интервал опроса сервера |
| `api_url` | URL Web-панели для отчётов |

---

## 🏗️ Архитектура

```
┌─────────────────┐      ZIP + SHA256      ┌─────────────────┐
│   Агент (Win)   │ ─────────────────────► │   Receiver      │
│  cmd/agent      │   TCP :9000            │  cmd/server     │
└─────────────────┘                        └────────┬────────┘
       ▲                                            │
       │ poll/heartbeat/report                      │ save
       │                                            ▼
┌─────────────────┐                        ┌─────────────────┐
│   Web-панель    │◄────── PostgreSQL ────►│  Хранилище      │
│  cmd/web :8080  │      :5432             │  /data          │
└─────────────────┘                        └─────────────────┘
```

### Структура проекта

```
BackUperGo/
├── cmd/
│   ├── agent/     # Точка входа агента
│   ├── server/    # Точка входа receiver
│   └── web/       # Web-панель + API
├── internal/
│   ├── agent/     # Логика polling, service, enroll
│   ├── archive/   # Создание ZIP + SHA256
│   ├── config/    # Конфигурация агента
│   ├── hash/      # Общие функции хеширования
│   ├── protocol/  # Типы протокола (HelloRequest, FinalResponse)
│   ├── receiver/  # Логика TCP-приёмника
│   ├── sender/    # Отправка архивов (retry, jitter)
│   ├── store/     # Работа с PostgreSQL
│   └── transport/ # Length-prefixed JSON по TCP
├── scripts/
│   ├── installer/     # Inno Setup скрипт
│   └── install-agent.ps1
├── docker-compose.yml
├── Dockerfile.receiver
├── Dockerfile.web
└── readme.md
```

---

## 🔧 Разработка

### Сборка всех компонентов

```bash
# Агент (Windows)
GOOS=windows GOARCH=amd64 go build -o dist/backuper-agent.exe ./cmd/agent

# Receiver (Linux/Docker)
go build -o dist/backuper-receiver ./cmd/server

# Web (Linux/Docker)
go build -o dist/backuper-web ./cmd/web
```

### Запуск без Docker

```bash
# PostgreSQL (локально)
docker run -d --name backuper-db \
  -e POSTGRES_DB=backuper \
  -e POSTGRES_USER=backuper \
  -e POSTGRES_PASSWORD=backuper \
  -p 5432:5432 postgres:16

# Receiver
set BACKUPER_API_KEY=my-secret-key-123
set BACKUPER_STORAGE_DIR=C:\BackUper\backups
.\dist\backuper-receiver.exe

# Web
set DATABASE_URL=postgres://backuper:backuper@localhost:5432/backuper?sslmode=disable
set BACKUPER_API_KEY=my-secret-key-123
.\dist\backuper-web.exe
```

### Тестирование

```bash
go build ./...
go vet ./...
```

---

## 📊 Web-панель

| Страница | Описание |
|----------|----------|
| `/login` | Вход (заглушка) |
| `/dashboard` | Сводка по бэкапам |
| `/agents` | Список агентов, конфигурация |
| `/settings` | Генерация скрипта установки |
| `/schedule` | Расписание (заглушка) |

### API endpoints

| Метод | Endpoint | Описание |
|-------|----------|----------|
| `POST` | `/api/enroll/script` | Генерация скрипта установки |
| `GET` | `/api/agent/download` | Скачивание агента |
| `POST` | `/api/agent/enroll` | Регистрация агента |
| `POST` | `/api/agent/heartbeat` | Обновление статуса |
| `GET` | `/api/agent/poll` | Опрос конфигурации |
| `GET` | `/api/agents` | Список всех агентов |
| `POST` | `/api/agents/{id}/config` | Обновление конфига |
| `POST` | `/api/agent/report` | Отчёт о выполнении |
| `POST` | `/api/agent/event` | Событие агента |

---

## 🔐 Безопасность

### API-ключи

- Задайте `BACKUPER_API_KEY` при запуске сервера
- Укажите тот же ключ в конфиге агента (`api_key`)
- Если ключ не задан — проверка отключена (локальный режим)

### Планы
- [ ] Индивидуальные ключи для каждого агента (БД)
- [ ] TLS для TCP-соединений
- [ ] Rate limiting

---

## 📝 Планы развития

- [ ] Полноценная авторизация в Web-панели
- [ ] Dashboard с реальными данными
- [ ] Расписание бэкапов (cron)
- [ ] Уведомления (Telegram, Email)
- [ ] Сжатие и инкрементальные бэкапы
- [ ] Метрики (Prometheus)

---

## 📄 Лицензия

MIT
