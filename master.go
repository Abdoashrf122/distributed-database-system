package main

import (
	"bufio"
	"bytes"
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
	MasterDSN  = "root:rootroot@tcp(192.168.43.103:3306)/"
	ListenAddr = ":8080"
)

// Slave endpoints for replication
var slaves = []string{
	"http://192.168.43.104:5001/slave", // replace with real IPs
	"http://192.168.43.105:5001/slave",
}

// Command structure for JSON replication
type Command struct {
	Action string            `json:"action"`
	Table  string            `json:"table,omitempty"`
	Data   map[string]string `json:"data,omitempty"`
	Query  map[string]string `json:"query,omitempty"`
	Attrs  []string          `json:"attrs,omitempty"`
	DBName string            `json:"dbname,omitempty"`
}

var (
	db      *sql.DB
	clients []net.Conn
)

func main() {
	var err error
	db, err = sql.Open("mysql", MasterDSN)
	if err != nil {
		log.Fatal("DB open error:", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("DB ping error:", err)
	}
	log.Println("[Server] Connected to MySQL")

	// Start HTTP server for replication
	go startReplicationServer()

	ln, err := net.Listen("tcp", ListenAddr)
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	defer ln.Close()
	log.Println("Server listening on", ListenAddr)

	go handleServerInput()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}
		clients = append(clients, conn)
		log.Println("Client connected:", conn.RemoteAddr())
		go handleClient(conn)
	}
}

func startReplicationServer() {
	http.HandleFunc("/master", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	log.Println("Replication HTTP server running on :5000")
	http.ListenAndServe(":5000", nil)
}

func handleServerInput() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("Enter query for clients: ")
		if !scanner.Scan() {
			return
		}
		dispatch(scanner.Text(), nil)
	}
}

func handleClient(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		q := scanner.Text()
		log.Println("Client sent:", q)
		dispatch(q, conn)
	}
	if err := scanner.Err(); err != nil {
		log.Println("Client read error:", err)
	}
}

// dispatch runs the query locally then broadcasts and replicates
func dispatch(query string, replyConn net.Conn) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return
	}
	parts := strings.Fields(trimmed)
	cmdStr := strings.ToUpper(parts[0])

	execLocal := func(q string) error {
		_, err := db.Exec(q)
		return err
	}
	broadcast := func(msg string) {
		for _, cl := range clients {
			if cl != nil {
				fmt.Fprintf(cl, "%s\n", msg)
			}
		}
	}

	switch cmdStr {
	case "SHOW":
		// handle SHOW DATABASES/TABLES (omitted for brevity)
		// ...
		return
	case "USE":
		execLocal(trimmed)
		broadcast(trimmed)
		broadcast("OK changed")
		replicateToSlaves(Command{Action: "use", DBName: parts[1]})
		return
	case "SELECT", "DESCRIBE", "DESC":
		// read-only, no replication
		// ...
		return
	case "CREATE", "DROP":
		if len(parts) > 1 && (parts[1] == "TABLE" || parts[1] == "DATABASE") {
			execLocal(trimmed)
			broadcast(trimmed)
			broadcast("OK changed")
			action := strings.ToLower(parts[0])
			obj := strings.ToLower(parts[1])
			replicateToSlaves(Command{Action: action + "_" + obj, DBName: parts[2], Table: parts[2], Attrs: nil})
			return
		}
	}

	// default DML
	if err := execLocal(trimmed); err != nil {
		log.Println("Exec error:", err)
		return
	}
	broadcast(trimmed)
	broadcast("OK changed")
	// build Command for DML
	dml := strings.ToLower(cmdStr)
	cmd := Command{Action: dml}
	// parse cmd.Data or cmd.Query for insert/update/delete
	// omitted: parsing logic to fill cmd.Data, cmd.Query, cmd.Table, cmd.DBName
	replicateToSlaves(cmd)
	if replyConn == nil {
		fmt.Println("OK changed")
	}
}

func replicateToSlaves(cmd Command) {
	jsonData, err := json.Marshal(cmd)
	if err != nil {
		log.Println("Replication marshal error:", err)
		return
	}
	for _, url := range slaves {
		go func(slaveURL string) {
			resp, err := http.Post(slaveURL, "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				log.Println("Replication error to", slaveURL, ":", err)
				return
			}
			resp.Body.Close()
		}(url)
	}
}
