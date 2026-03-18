package receiver

import (
	"BackUper/internal/hash"
	"BackUper/internal/protocol"
	"BackUper/internal/transport"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	baseDir string
	apiKey  string
)

func init() {
	// Путь к хранилищу из переменной окружения или по умолчанию
	baseDir = os.Getenv("BACKUPER_STORAGE_DIR")
	if baseDir == "" {
		baseDir = "C:\\BackUper\\backups"
	}

	// APIKey для проверки агентов (опционально)
	apiKey = strings.TrimSpace(os.Getenv("BACKUPER_API_KEY"))
}

func Run() {
	receiverMain(nil)
}

// RunWithShutdown запускает клиент в фоне. Возвращает функцию stop для остановки.
func RunWithShutdown() (stop func()) {
	done := make(chan struct{})
	var listener net.Listener
	listener, err := net.Listen("tcp", ":9000") // слушаем на всех интерфейсах
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		defer close(done)
		receiverMain(listener)
	}()
	return func() {
		if listener != nil {
			listener.Close()
		}
		<-done
	}
}

func receiverMain(existingListener net.Listener) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		log.Fatalf("Ошибка создания директории: %v", err)
	}

	var listener net.Listener
	var err error
	if existingListener != nil {
		listener = existingListener
	} else {
		listener, err = net.Listen("tcp", ":9000")
		if err != nil {
			log.Fatal(err)
		}
	}
	defer listener.Close()

	addr := listener.Addr()
	log.Printf("[CLIENT] Слушаем на %s (порт 9000)", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("[CLIENT] Остановлен или ошибка Accept:", err)
			return
		}
		log.Printf("[CLIENT] Входящее соединение от %s", conn.RemoteAddr())
		go handleConnectionFromServer(conn)
	}
}

func handleConnectionFromServer(conn net.Conn) {
	remote := conn.RemoteAddr().String()
	defer conn.Close()
	log.Printf("[CLIENT] Обработка соединения от %s", remote)

	var helloRequest protocol.HelloRequest

	log.Printf("[CLIENT] Ожидание HELLO...")
	if err := transport.ReceiveMessage(conn, &helloRequest); err != nil {
		log.Printf("[CLIENT] ОШИБКА приема HELLO от %s: %v", remote, err)
		return
	}
	log.Printf("[CLIENT] HELLO получен: name=%s, size=%d", helloRequest.Name, helloRequest.Size)

	// Проверка APIKey (если задан)
	if apiKey != "" && helloRequest.Auth != apiKey {
		log.Printf("[CLIENT] AUTH FAIL: неверный APIKey от %s", remote)
		responseData := protocol.FinalResponse{
			JobID:      helloRequest.JobID,
			Status:     "AUTH_FAIL",
			Reason:     "Неверный APIKey",
			Size:       0,
			SHA256:     "",
			ReceivedAt: time.Now(),
			StoredPath: "",
		}
		_ = transport.SendMessage(conn, responseData)
		return
	}

	safeFileName := filepath.Base(helloRequest.Name)
	filePath := filepath.Join(baseDir, "received_"+safeFileName)

	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("Ошибка создания файла: %v", err)
		return
	}
	defer file.Close()

	log.Printf("[CLIENT] Приём архива (%d байт)...", helloRequest.Size)
	writenBytes, err := io.CopyN(file, conn, helloRequest.Size)
	if err != nil || writenBytes != helloRequest.Size {
		responseData := protocol.FinalResponse{
			JobID:      helloRequest.JobID,
			Status:     "SIZE_FAIL",
			Reason:     "Размер файла не совпадает с ожимдаемым!",
			Size:       writenBytes,
			SHA256:     "Не посчитано",
			ReceivedAt: time.Now(),
			StoredPath: "NULL",
		}

		_ = transport.SendMessage(conn, responseData)
		file.Close()
		os.Remove(file.Name())
		log.Printf("[CLIENT] ОШИБКА записи: %v (получено %d/%d)", err, writenBytes, helloRequest.Size)
		return
	}
	log.Printf("[CLIENT] Архив принят (%d байт)", writenBytes)

	// Сбрасываем буфер на диск перед чтением для хеша
	if err := file.Sync(); err != nil {
		log.Printf("Ошибка Sync файла: %v", err)
		return
	}

	log.Printf("[CLIENT] Подсчёт SHA256...")
	receivedHash, err := hash.SHA256File(filePath)
	if err != nil {
		log.Printf("[CLIENT] ОШИБКА подсчета SHA256: %v", err)
		return
	}

	status := "OK"
	reason := ""

	if receivedHash != helloRequest.SHA256 {
		status = "HASH_FAIL"
		reason = "Несовпадение хеша SHA256"
	}

	responseData := protocol.FinalResponse{
		JobID:      helloRequest.JobID,
		Status:     status,
		Reason:     reason,
		Size:       writenBytes,
		SHA256:     receivedHash,
		ReceivedAt: time.Now(),
		StoredPath: file.Name(),
	}

	log.Printf("[CLIENT] Отправка FINAL (status=%s)...", status)
	if err := transport.SendMessage(conn, responseData); err != nil {
		log.Printf("[CLIENT] ОШИБКА отправки FINAL: %v", err)
		return
	}
	log.Printf("[CLIENT] FINAL отправлен. Архив сохранён: %s", filePath)
}
