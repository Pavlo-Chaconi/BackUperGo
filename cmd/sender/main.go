package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"BackUper/internal/receiver"
	"BackUper/internal/sender"
)

func main() {

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("BackUper")
		fmt.Println("1) Запустить сервер (receiver)")
		fmt.Println("2) Отправить архив (client -> send)")
		fmt.Println("3) Выход")
		fmt.Print("Выберите пункт: ")

		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			fmt.Println("Сервер запускается на localhost:9000. Для остановки закройте окно или нажмите Ctrl+C.")
			receiver.Run() // блокирует до завершения
			return

		case "2":
			opts := sender.SenderOptions{
				RootFolder:  promptDefault(reader, "Каталог для архивации", "C:\\Users\\...\\test"),
				ArchivePath: promptDefault(reader, "Путь к архиву", "C:\\Users\\...\\backup.zip"),
				Addr:        promptDefault(reader, "Адрес сервера (host:port)", "localhost:9000"),
				APIKey:      promptDefault(reader, "API ключ", "test_key"),
				Name:        promptDefault(reader, "Имя задания", "backup"),
				MaxRetries:  mustAtoi(promptDefault(reader, "Число попыток", "5")),
			}

			if err := sender.BuildAndSendArchive(opts); err != nil {
				log.Printf("Ошибка отправки: %v\n", err)
			} else {
				fmt.Println("Отправлено успешно.")
			}
			fmt.Println("Нажмите Enter, чтобы вернуться в меню...")
			reader.ReadString('\n')

		case "3":
			fmt.Println("Выход.")
			return

		default:
			fmt.Println("Неверный выбор. Повторите ввод.")
		}
	}
}

func promptDefault(r *bufio.Reader, label, def string) string {
	fmt.Printf("%s [%s]: ", label, def)
	text, _ := r.ReadString('\n')
	text = strings.TrimSpace(text)
	if text == "" {
		return def
	}
	return text
}

func mustAtoi(s string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 5
	}
	return v
}
