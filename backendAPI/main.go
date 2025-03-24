package main

import (
	"backendAPI/handler"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"time"
)

func main() {
	// Setup enhanced logging with timestamps
	logFile, err := os.OpenFile("backendAPI.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
	} else {
		defer logFile.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.Println("BackendAPI starting...")

	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			errorMsg := fmt.Sprintf("PANIC RECOVERED: %v\n%s", r, debug.Stack())
			log.Println(errorMsg)

			// Write to emergency log file in case main log is not working
			errLogFile := fmt.Sprintf("crash_%s.log", time.Now().Format("20060102_150405"))
			os.WriteFile(errLogFile, []byte(errorMsg), 0644)

			// Terminate with non-zero exit code
			os.Exit(1)
		}
	}()

	// Start the server
	handler.RequestHandler()

	// If we reach here, log that the server exited normally
	log.Println("BackendAPI exited normally")
}
