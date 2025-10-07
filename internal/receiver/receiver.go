package receiver

import (
	"BackUper/internal/protocol"
	"BackUper/internal/transport"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net"
	"os"
	"time"
)

func receiverMain() {
	listner, err := net.Listen("tcp", "localhost:9000")
	if err != nil {
		log.Fatal(err)
	}
	defer listner.Close()

	for {
		conn, err := listner.Accept()
		if err != nil {
			log.Println("Ошибка во время принятия сообщения: ", err)
			continue
		}

		go handleConnectionFromServer(conn)
	}
}

func handleConnectionFromServer(conn net.Conn) {

	defer conn.Close()

	// var length uint32

	var helloRequest protocol.HelloRequest

	// err := binary.Read(conn, binary.BigEndian, &length)
	// if err != nil {
	// 	log.Fatalf("Ошибка чтения длинны сообщения: %v", err)
	// }

	// buf := make([]byte, length)

	// _, err = io.ReadFull(conn, buf)
	// if err != nil {
	// 	log.Fatalf("Ошибка чтения JSON: %v", err)
	// }

	// err = json.Unmarshal(buf, &helloRequest)
	// if err != nil {
	// 	log.Fatalf("Ошибка парсинга JSON: %v", err)
	// }

	if err := transport.ReceiveMessage(conn, &helloRequest); err != nil {
		log.Fatalf("Ошибка приема HELLO: %v", err)
	}

	file, err := os.Create("received_" + helloRequest.Name)
	if err != nil {
		log.Fatalf("Ошибка создания файла: %v", err)
	}

	_, err = io.Copy(file, conn)
	if err != nil {
		log.Fatalf("Ошибка записи файла: %v", err)
	}

	file.Close()

	receivedHash, receivedHashSize, err := calc256Hex("received_" + helloRequest.Name)
	if err != nil {
		log.Fatalf("Ошибка подсчета SHA256: %v", err)
	}

	status := "OK"
	reason := ""

	if receivedHash != helloRequest.SHA256 {
		status = "FAIL"
		reason = "Несовпадение хеша SHA256"
	}

	responseData := protocol.FinalResponse{
		JobID:      1,
		Status:     status,
		Reason:     reason,
		Size:       receivedHashSize,
		SHA256:     receivedHash,
		ReceivedAt: time.Now(),
		StoredPath: "received_" + helloRequest.Name,
	}

	if err := transport.SendMessage(conn, responseData); err != nil {
		log.Fatalf("Ошибка отправки FINAL: %v", err)
	}

}

func calc256Hex(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", 0, err
	}

	info, err := os.Stat(path)
	if err != nil {
		log.Fatalf("Ошибка при подсчете финального размера архива на стороне клиента!: %v", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), info.Size(), nil
}
