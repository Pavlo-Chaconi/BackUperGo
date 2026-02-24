package main

import (
	"log"

	"BackUper/internal/logging"
	"BackUper/internal/receiver"
)

func main() {
	logPath, err := logging.Init("BackUperServer", 14)
	if err != nil {
		log.Printf("failed to init file logging: %v", err)
	} else {
		defer logging.Close()
		log.Printf("log file: %s", logPath)
	}

	receiver.Run()
}
