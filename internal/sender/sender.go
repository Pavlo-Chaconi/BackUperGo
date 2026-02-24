package sender

import (
	"BackUper/internal/archive"
	"BackUper/internal/protocol"
	"BackUper/internal/transport"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SenderOptions struct {
	RootFolder  string
	ArchivePath string
	TempDir     string
	APIKey      string
	Name        string
	Addr        string
	MaxRetries  int
	EventSink   EventSink
}

type Event struct {
	Time    time.Time         `json:"time"`
	Level   string            `json:"level"`
	Name    string            `json:"name"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type EventSink interface {
	Emit(event Event)
}

func emitEvent(sink EventSink, level, name, message string, fields map[string]string) {
	if sink == nil {
		return
	}
	sink.Emit(Event{
		Time:    time.Now().UTC(),
		Level:   level,
		Name:    name,
		Message: message,
		Fields:  fields,
	})
}

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

	emitEvent(options.EventSink, "info", "archive.prepare", "prepare archive", map[string]string{
		"root": options.RootFolder,
	})

	// Определяем корректный путь к файлу архива:
	// - если пусто или передан каталог → сгенерируем имя файла внутри каталога
	generatedArchive := false
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
		if options.ArchivePath != "" {
			dirPath = filepath.Clean(options.ArchivePath)
		} else if strings.TrimSpace(options.TempDir) != "" {
			dirPath = filepath.Clean(options.TempDir)
		} else {
			dirPath = os.TempDir()
		}
		base := filepath.Base(options.RootFolder)
		if base == "." || base == string(filepath.Separator) {
			base = "backup"
		}
		options.ArchivePath = filepath.Join(dirPath,
			fmt.Sprintf("%s_%s.zip", base, time.Now().Format("20060102_150405")))
		generatedArchive = true
	}

	files, err := collectFiles(options.RootFolder, options.EventSink)
	if err != nil {
		emitEvent(options.EventSink, "error", "archive.collect.fail", err.Error(), nil)
		return fmt.Errorf("ошибка получения файлов для архивации: %w", err)
	}

	emitEvent(options.EventSink, "info", "archive.create.start", "start creating archive", map[string]string{
		"archive": options.ArchivePath,
	})

	size, sha256Hex, err := archive.CreateZipArchive(options.ArchivePath, files)
	if err != nil {
		emitEvent(options.EventSink, "error", "archive.create.fail", err.Error(), map[string]string{
			"archive": options.ArchivePath,
		})
		return fmt.Errorf("ошибка архивации: %w", err)
	}

	emitEvent(options.EventSink, "info", "archive.create.ok", "archive created", map[string]string{
		"archive": options.ArchivePath,
		"size":    fmt.Sprintf("%d", size),
		"sha256":  sha256Hex,
	})

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

	emitEvent(options.EventSink, "info", "send.start", "start sending archive", map[string]string{
		"addr":    options.Addr,
		"archive": options.ArchivePath,
		"size":    fmt.Sprintf("%d", size),
	})
	if err := sendWithRetry(options.Addr, options.ArchivePath, helloData, options.MaxRetries, options.EventSink); err != nil {
		emitEvent(options.EventSink, "error", "send.fail", err.Error(), map[string]string{
			"addr": options.Addr,
		})
		return fmt.Errorf("ошибка при вызове функции sendWithRetry: %w", err)
	}
	emitEvent(options.EventSink, "info", "send.ok", "archive sent", nil)

	if generatedArchive {
		if err := os.Remove(options.ArchivePath); err != nil {
			emitEvent(options.EventSink, "warn", "archive.cleanup.fail", err.Error(), map[string]string{
				"archive": options.ArchivePath,
			})
		} else {
			emitEvent(options.EventSink, "info", "archive.cleanup.ok", "temp archive removed", map[string]string{
				"archive": options.ArchivePath,
			})
		}
	}
	return nil
}
func collectFiles(rootFolder string, sink EventSink) ([]string, error) {
	var files []string
	err := filepath.Walk(rootFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			emitEvent(sink, "error", "file.access.fail", err.Error(), map[string]string{"path": path})
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func sendOnce(Addr string, archivePath string, request protocol.HelloRequest, sink EventSink) error {
	emitEvent(sink, "info", "send.connect.start", "dial tcp", map[string]string{"addr": Addr})
	conn, err := net.DialTimeout("tcp", Addr, 3*time.Second)
	if err != nil {
		emitEvent(sink, "error", "send.connect.fail", err.Error(), map[string]string{"addr": Addr})
		return fmt.Errorf("ошибка при установке соединения с клиентом: %w", err)
	}
	defer conn.Close()
	emitEvent(sink, "info", "send.connect.ok", "connected", map[string]string{"local": conn.LocalAddr().String(), "remote": conn.RemoteAddr().String()})

	emitEvent(sink, "info", "send.hello.start", "send hello", map[string]string{"name": request.Name, "size": fmt.Sprintf("%d", request.Size)})
	if err := transport.SendMessage(conn, request); err != nil {
		emitEvent(sink, "error", "send.hello.fail", err.Error(), nil)
		return fmt.Errorf("ошибка при отправке HELLO: %w", err)
	}
	emitEvent(sink, "info", "send.hello.ok", "hello sent", nil)

	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("ошибка при открытии файла: %w", err)
	}
	defer file.Close()

	emitEvent(sink, "info", "send.payload.start", "send archive", map[string]string{"size": fmt.Sprintf("%d", request.Size)})
	written, err := io.CopyN(conn, file, request.Size)
	if err != nil {
		emitEvent(sink, "error", "send.payload.fail", err.Error(), map[string]string{"written": fmt.Sprintf("%d", written), "size": fmt.Sprintf("%d", request.Size)})
		return fmt.Errorf("ошибка при передаче файла! %w", err)
	}
	emitEvent(sink, "info", "send.payload.ok", "archive sent", map[string]string{"written": fmt.Sprintf("%d", written)})

	emitEvent(sink, "info", "send.final.wait", "wait final", nil)
	var finalRequest protocol.FinalResponse
	if err := transport.ReceiveMessage(conn, &finalRequest); err != nil {
		emitEvent(sink, "error", "send.final.fail", err.Error(), nil)
		return fmt.Errorf("ошибка полученя FINAL: %w", err)
	}
	emitEvent(sink, "info", "send.final.ok", "final received", map[string]string{"status": finalRequest.Status})

	switch finalRequest.Status {
	case "SIZE_FAIL", "HASH_FAIL":
		return fmt.Errorf("получен статус %s: %s", finalRequest.Status, finalRequest.Reason)
	case "OK":
		return nil
	default:
		return fmt.Errorf("неизвестный статус ошибки:%s", finalRequest.Status)

	}
}

func sendWithRetry(addr string, archivePath string, request protocol.HelloRequest, maxRetries int, sink EventSink) error {
	rand.Seed(time.Now().UnixNano())
	minJitter := 10
	maxJitter := 20
	multiplier := 2
	maxDelay := 300
	delay := 10

	for attempt := 0; attempt < maxRetries; attempt++ {
		emitEvent(sink, "info", "send.retry", "attempt", map[string]string{
			"attempt": fmt.Sprintf("%d", attempt+1),
			"max":     fmt.Sprintf("%d", maxRetries),
		})
		err := sendOnce(addr, archivePath, request, sink)
		if err == nil {
			return nil
		}
		emitEvent(sink, "warn", "send.retry.fail", err.Error(), nil)

		totalSeconds := (min(delay*multiplier, maxDelay) + minJitter + rand.Intn(maxJitter-minJitter+1))
		emitEvent(sink, "info", "send.retry.wait", "retry wait", map[string]string{
			"seconds": fmt.Sprintf("%d", totalSeconds),
		})
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
