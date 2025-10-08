package sender

import (
	"BackUper/internal/archive"
	"BackUper/internal/protocol"
	"BackUper/internal/transport"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"time"
)

var api_key string
var archivePath string

func senderMain() {
	var files []string
	root := "C:/Users/.../test"

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Ошибка доступа по пути %s: %v", path, err)
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Фатальная ошибка обхода: %v", err)
	}

	api_key = "test_key"
	archivePath = "C:/Users/.../backup.zip"

	size, sha256Hex, err := archive.CreateZipArchive(archivePath, files)
	if err != nil {
		log.Fatalf("Ошибка архивации: %v", err)
	}

	log.Printf("Архив готов: %s (размер=%d байт, sha256=%s)", archivePath, size, sha256Hex)

	helloData := protocol.HelloRequest{
		Ver:         1,
		Auth:        api_key,
		JobID:       0,
		Name:        "testRequest",
		Size:        size,
		SHA256:      sha256Hex,
		Compression: "test",
		Encryption:  "test",
	}

	if err := sendWithRetry(archivePath, helloData, 5); err != nil {
		fmt.Errorf("ошибка при вызове функции SendOnce: %w", err)
	}

}

func sendOnce(archivePath string, request protocol.HelloRequest) error {
	conn, err := net.Dial("tcp", "192.168.0.100:9000")
	if err != nil {
		fmt.Errorf("ошибка при установке соединения с клиентом: %w", err)
	}

	net.DialTimeout("tcp", "192.168.0.100:9000", 1000)

	defer conn.Close()

	if err := transport.SendMessage(conn, request); err != nil {
		fmt.Errorf("ошибка при отправке HELLO: %w", err)
	}

	file, err := os.Open(archivePath)
	if err != nil {
		fmt.Errorf("ошибка при открытии файла: %w", err)
	}

	defer file.Close()

	_, err = io.CopyN(conn, file, request.Size)
	if err != nil {
		fmt.Errorf("ошибка при передаче файла! %w", err)
	}

	var finalRequest protocol.FinalResponse
	if err := transport.ReceiveMessage(conn, &finalRequest); err != nil {
		return fmt.Errorf("ошибка полученя FINAL: %w", err)
	}

	switch finalRequest.Status {
	case "SIZE_FAIL", "HASH_FAIL":
		return fmt.Errorf("получен статус %s: %s", finalRequest.Status, finalRequest.Reason)
	case "OK":
		return nil
	default:
		return fmt.Errorf("неизвестный статус ошибки:%s", finalRequest.Status)

	}
}

func sendWithRetry(archivePath string, request protocol.HelloRequest, maxRetries int) error {
	rand.Seed(time.Now().UnixNano())
	minJitter := 10
	maxJitter := 20
	multiplier := 2
	maxDelay := 300
	delay := 10

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := sendOnce(archivePath, request)
		if err == nil {
			return nil
		}

		totalSeconds := (min(delay*multiplier, maxDelay) + minJitter + rand.Intn(maxJitter-minJitter+1))
		time.Sleep(time.Duration(totalSeconds) * time.Second)

	}
	return fmt.Errorf("все %d попытки на отправку файла были исчерпаны, передача завершилась ошибкой", maxRetries)
}
