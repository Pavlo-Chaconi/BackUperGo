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

	if err := transport.ReceiveMessage(conn, &helloRequest); err != nil {
		log.Fatalf("Ошибка приема HELLO: %v", err)
	}

	file, err := os.Create("received_" + helloRequest.Name)
	if err != nil {
		log.Fatalf("Ошибка создания файла: %v", err)
	}
	defer file.Close()

	//Необходимо читать определенное количество байт, сколько передаст HELLO реквест изначально, потому используем io.CopyN

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
		log.Printf("Ошибка записи файла: %v", err)
		return
	}

	receivedHash, err := calc256Hex("received_" + helloRequest.Name)
	if err != nil {

		log.Fatalf("Ошибка подсчета SHA256: %v", err)
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

	if err := transport.SendMessage(conn, responseData); err != nil {
		log.Fatalf("Ошибка отправки FINAL: %v", err)
	}

}

func calc256Hex(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	// info, err := os.Stat(path)
	// if err != nil {
	// 	log.Fatalf("Ошибка при подсчете финального размера архива на стороне клиента!: %v", err)
	// }

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
