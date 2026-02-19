# BackUper

## ✅ Тестовый основной функционал — ГОТОВ!!! УРА

Архивация, отправка и приём работают. Меню для переключения Сервер/Клиент, логирование в консоль.

---

## Структура проекта

### Sender (сервер — отправляет архив)
- `cmd/sender/main.go` — точка входа, меню, конфигурация
- `internal/sender/sender.go` — отправка архива клиенту
- `internal/archive/archiver.go` — архивация (ZIP, SHA256)
### Client (клиент — принимает архив)
- `internal/receiver/receiver.go` — приём архива

### Общее
- `internal/protocol/types.go` — типы HELLO, FINAL
- `internal/transport/transport.go` — length-prefixed JSON по TCP

---

## Планы развития

- [ ] **Логирование в отдельные файлы**  
  Весь лог — в файлы. Факт наличия лог-файла отправляется в БД.

- [ ] **База данных для архивов**  
  Запись метаданных передаваемых архивов для аналитики. Решение о способе хранения (холодный/горячий) по этим данным.

- [ ] **Оповещения в Telegram**  
  Уведомления администратору о статусе бэкапов.

---

## Протокол (JSON handshake)

### HELLO
```json
{
  "ver": 1,
  "auth": "ApiKey <token>",
  "job_id": "...",
  "name": "db1_2025-09-04_1500.tar.zst",
  "size": 123456789,
  "sha256": "<64-hex>",
  "compression": "zstd",
  "encryption": "none"
}
```

### FINAL
```json
{
  "job_id": "...",
  "status": "OK|FAIL",
  "reason": "… если FAIL",
  "size": 123456789,
  "sha256": "<64-hex>",
  "received_at": "2025-09-04T15:27:03Z",
  "stored_path": "/backups/db1/2025-09-04/db1_2025-09-04_1500.tar.zst"
}
```
