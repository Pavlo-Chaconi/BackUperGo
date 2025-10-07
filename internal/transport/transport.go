package transport

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

func SendMessage(conn net.Conn, v any) error {
	jsonData, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("ошибка сериализации данных: %w", err)
	}
	// jsonDataLength = uint32(len(jsonData))
	err = binary.Write(conn, binary.BigEndian, uint32(len(jsonData)))
	if err != nil {
		return fmt.Errorf("ошибка отправки длинны JSON данных: %w", err)
	}

	_, err = conn.Write(jsonData)
	if err != nil {
		return fmt.Errorf("ошибка отправки данных JSON: %w", err)
	}
	return nil
}

func ReceiveMessage(conn net.Conn, v any) error {
	var length uint32

	err := binary.Read(conn, binary.BigEndian, &length)
	if err != nil {
		return fmt.Errorf("ошибка чтения длинны сообщения JSON: %w", err)
	}
	buf := make([]byte, length)

	_, err = io.ReadFull(conn, buf)
	if err != nil {
		return fmt.Errorf("ошибка чтения документа JSON: %w", err)
	}

	err = json.Unmarshal(buf, v)
	if err != nil {
		return fmt.Errorf("ошибка парсинга JSON файла: %w", err)
	}

	return nil
}
