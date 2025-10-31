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

type SenderOptions struct {
	RootFolder  string
	ArchivePath string
	APIKey      string
	Name        string
	Addr        string
	MaxRetries  int
}

// func BuildAndSendArchive(options SenderOptions) error {
// 	if options.RootFolder == "" || options.ArchivePath == "" || options.Addr == "" {
// 		return fmt.Errorf("rootFolder, archivePath и addr являются обязательными параметрами")
// 	}
// 	if options.MaxRetries <= 0 {
// 		options.MaxRetries = 5
// 	}
// 	if options.Name == "" {
// 		options.Name = "backup"
// 	}
// 	if options.APIKey == "" {
// 		return fmt.Errorf("apiKey является обязательным параметром")
// 	}

// 	files, err := collectFiles(options.RootFolder)
// 	if err != nil {
// 		return fmt.Errorf("ошибка получения файлов для архивации: %w", err)
// 	}

// 	size, sha256Hex, err := archive.CreateZipArchive(options.ArchivePath, files)
// 	if err != nil {
// 		return fmt.Errorf("ошибка архивации: %w", err)
// 	}

// 	helloData := protocol.HelloRequest{
// 		Ver:         1,
// 		Auth:        options.APIKey,
// 		JobID:       0,
// 		Name:        options.Name,
// 		Size:        size,
// 		SHA256:      sha256Hex,
// 		Compression: "zip",
// 		Encryption:  "none",
// 	}

// 	if err := sendWithRetry(options.Addr, options.ArchivePath, helloData, options.MaxRetries); err != nil {
// 		return fmt.Errorf("ошибка при вызове функции sendWithRetry: %w", err)
// 	}
// 	return nil
// }

func BuildAndSendArchive(options SenderOptions) error {
	if options.RootFolder == "" || options.Addr == "" {
		return fmt.Errorf("rootFolder и addr являются обязательными параметрами")
	}
	if options.MaxRetries <= 0 {
		options.MaxRetries = 5
	}
	if options.Name == "" {
		options.Name = "backup"
	}
	if options.APIKey == "" {
		return fmt.Errorf("apiKey является обязательным параметром")
	}

	// Определяем корректный путь к файлу архива:
	// - если пусто или передан каталог → сгенерируем имя файла внутри каталога
	isDirHint := false
	if options.ArchivePath == "" {
		isDirHint = true
	} else {
		if info, err := os.Stat(options.ArchivePath); err == nil && info.IsDir() {
			isDirHint = true
		} else {
			// признак каталога по завершающему слэшу
			last := options.ArchivePath[len(options.ArchivePath)-1]
			if last == '\\' || last == '/' {
				isDirHint = true
			}
			// корень диска вида "C:" тоже трактуем как каталог
			if len(options.ArchivePath) == 2 && options.ArchivePath[1] == ':' {
				isDirHint = true
			}
		}
	}

	if isDirHint {
		var dirPath string
		if options.ArchivePath == "" {
			dirPath = os.TempDir()
		} else {
			dirPath = filepath.Clean(options.ArchivePath)
		}
		base := filepath.Base(options.RootFolder)
		if base == "." || base == string(filepath.Separator) {
			base = "backup"
		}
		options.ArchivePath = filepath.Join(dirPath,
			fmt.Sprintf("%s_%s.zip", base, time.Now().Format("20060102_150405")))
	}

	files, err := collectFiles(options.RootFolder)
	if err != nil {
		return fmt.Errorf("ошибка получения файлов для архивации: %w", err)
	}

	size, sha256Hex, err := archive.CreateZipArchive(options.ArchivePath, files)
	if err != nil {
		return fmt.Errorf("ошибка архивации: %w", err)
	}

	helloData := protocol.HelloRequest{
		Ver:         1,
		Auth:        options.APIKey,
		JobID:       0,
		Name:        options.Name,
		Size:        size,
		SHA256:      sha256Hex,
		Compression: "zip",
		Encryption:  "none",
	}

	if err := sendWithRetry(options.Addr, options.ArchivePath, helloData, options.MaxRetries); err != nil {
		return fmt.Errorf("ошибка при вызове функции sendWithRetry: %w", err)
	}
	return nil
}

func collectFiles(rootFolder string) ([]string, error) {
	var files []string
	err := filepath.Walk(rootFolder, func(path string, info os.FileInfo, err error) error {
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
	return files, nil
}

func sendOnce(Addr string, archivePath string, request protocol.HelloRequest) error {
	conn, err := net.DialTimeout("tcp", Addr, 3*time.Second)
	if err != nil {
		return fmt.Errorf("ошибка при установке соединения с клиентом: %w", err)
	}

	defer conn.Close()

	if err := transport.SendMessage(conn, request); err != nil {
		return fmt.Errorf("ошибка при отправке HELLO: %w", err)
	}

	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("ошибка при открытии файла: %w", err)
	}

	defer file.Close()

	_, err = io.CopyN(conn, file, request.Size)
	if err != nil {
		return fmt.Errorf("ошибка при передаче файла! %w", err)
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

func sendWithRetry(addr string, archivePath string, request protocol.HelloRequest, maxRetries int) error {
	rand.Seed(time.Now().UnixNano())
	minJitter := 10
	maxJitter := 20
	multiplier := 2
	maxDelay := 300
	delay := 10

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := sendOnce(addr, archivePath, request)
		if err == nil {
			return nil
		}

		totalSeconds := (min(delay*multiplier, maxDelay) + minJitter + rand.Intn(maxJitter-minJitter+1))
		time.Sleep(time.Duration(totalSeconds) * time.Second)

	}
	return fmt.Errorf("все %d попытки на отправку файла были исчерпаны, передача завершилась ошибкой", maxRetries)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
