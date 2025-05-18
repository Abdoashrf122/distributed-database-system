package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

const (
	LocalDSN   = "ammar:rootroot@tcp(192.168.43.103:3306)/"
	MasterAddr = "192.168.43.103:8080"
)

var db *sql.DB

// Command for replication
type Command struct {
	Action string            `json:"action"`
	Table  string            `json:"table,omitempty"`
	Data   map[string]string `json:"data,omitempty"`
	Query  map[string]string `json:"query,omitempty"`
	Attrs  []string          `json:"attrs,omitempty"`
	DBName string            `json:"dbname,omitempty"`
}

func main() {
	var err error
	db, err = sql.Open("mysql", LocalDSN)
	if err != nil {
		log.Fatalf("DB open error: %v", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatalf("DB ping error: %v", err)
	}
	log.Printf("[Slave] Connected to local MySQL (%s)", LocalDSN)

	// start HTTP server for replication commands
	http.HandleFunc("/slave", handleCommand)
	go func() {
		log.Println("Replication HTTP server running on :5001")
		http.ListenAndServe(":5001", nil)
	}()

	// start TCP connection to master for client queries
	conn, err := net.Dial("tcp", MasterAddr)
	if err != nil {
		log.Fatalf("Failed to connect to master (%s): %v", MasterAddr, err)
	}
	defer conn.Close()
	log.Printf("[Slave] Connected to master at %s", MasterAddr)

	go receiveMessages(conn)
	go readUserInput()
	select {}
}

func handleCommand(w http.ResponseWriter, r *http.Request) {
	var cmd Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid command", http.StatusBadRequest)
		return
	}
	// when master down, this HTTP handler applies local replication commands
	executeCmd(cmd)
	w.Write([]byte("OK"))
}

func receiveMessages(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)

		// prevent DB/TABLE create/drop
		if strings.HasPrefix(upper, "CREATE DATABASE") ||
			strings.HasPrefix(upper, "CREATE TABLE") ||
			strings.HasPrefix(upper, "DROP DATABASE") ||
			strings.HasPrefix(upper, "DROP TABLE") {
			fmt.Printf("[Slave] ERROR: Command not allowed: %s\n", line)
			continue
		}

		// exec or show
		execLocal(line, upper)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[Slave] Receive error: %v", err)
	}
}

func readUserInput() {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter query: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("[Slave] Input error: %v", err)
			continue
		}
