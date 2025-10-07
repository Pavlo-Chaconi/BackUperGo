package sender

import (
	"BackUper/internal/archive"
	"BackUper/internal/protocol"
	"BackUper/internal/transport"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
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
		JobID:       "test",
		Name:        "testRequest",
		Size:        size,
		SHA256:      sha256Hex,
		Compression: "test",
		Encryption:  "test",
	}

	// jsonData, err := json.Marshal(helloData)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	conn, err := net.Dial("tcp", "192.168.0.100:9000")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err := transport.SendMessage(conn, helloData); err != nil {
		log.Fatalf("Ошибка при отправке HELLO: %v", err)
	}

	// err = binary.Write(conn, binary.BigEndian, uint32(len(jsonData)))
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// _, err = conn.Write(jsonData)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	file, err := os.Open(archivePath)
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()
	_, err = io.Copy(conn, file)

}
