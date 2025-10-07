Структура проекта Sender
cmd/sender/main.go - Описание взаимодействия модулей между собой, конфигурации (Отправитель, приемщик) запуска, расписания 
internal/sender.go - Модуль взаимодействия при отправке архива к receiver (Готов на 25%)
internal/archive/archivier.go - Модуль архивации (Готов на 75%, нуждается в тестировании)

*Оффтоп будет добавлена переменная окружения для хранения конфигурационных значений*
*После написания всех модулей проект будет выложен на Github*

Общие для проекта данные
internal/protocol/types.go
internal/transport/transport.go - реализован на 50 %

Структура проекта Receiver
cmd/receiver/main.go - Описание взаимодействия модулей мжду собой 
internal/receiver/receiver.go- Получающий модуль (Готов на 25%)
internal/notify/telegram.go - Оповещающий модуль

Формат Json-полей для хендшейка между серверной конфигурацией и клиентской (В нашем случаей Sender и Receiver)

HELLO

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

FINAL

{
  "job_id": "...",
  "status": "OK|FAIL",
  "reason": "… если FAIL",
  "size": 123456789,
  "sha256": "<64-hex>",
  "received_at": "2025-09-04T15:27:03Z",
  "stored_path": "/backups/db1/2025-09-04/db1_2025-09-04_1500.tar.zst"
}