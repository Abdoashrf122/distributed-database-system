
# Master-Slave Replication System (Go + MySQL)

## 🔁 System Communication Flow

```
+-------------+           POST /slave            +-------------+
|             | <------------------------------> |             |
|   Master    |                                 |    Slave     |
|   Server    | -----> Replication via HTTP ----|   Server(s)  |
| (Go + MySQL)|                                 | (Go + MySQL) |
+-------------+                                 +-------------+
     [client requests]
            |
            v
  /master  (POST, GET, PUT, DELETE)

- Master handles all client requests and replicates changes to slaves.
- Slaves execute commands such as SELECT, INSERT, UPDATE, DELETE, SEARCH.
```

## 📖 Overview

This project implements a simple Master-Slave data replication system in Go.

- The Master Node handles all write operations (create database/table, insert, update, delete).
- Slave nodes are read-only replicas and receive data from the master through HTTP requests.
- Replication is achieved by broadcasting every change from the master to all registered slave devices.

---

## 🏗️ Architecture

- Master Node
  - Hosts HTTP server on port :5000.
  - Handles requests at /master.
  - Broadcasts critical operations (insert/update/create) to all slaves.
  - Stores configuration of slaves in []string{}.

- Slave Node
  - Listens on port :5001.
  - Exposes /slave endpoint.
  - Accepts and applies changes pushed from the master.

---

## 🚀 Getting Started

### 1. Clone the repository
```bash
git clone https://github.com/your-user/replication-system.git
cd replication-system
```

### 2. Setup MySQL

- Ensure MySQL is installed and running on both master and slave machines.
- Set root credentials in the code:
```go
dsn := "root:rootroot@tcp(localhost:3306)/"
```

### 3. Run Master
```bash
go run master.go
```

### 4. Run Slave(s)
```bash
go run slave.go
```

---

## 📦 Features

- ✅ Create & Drop Databases/Tables from Master
- ✅ Insert, Update, Delete (Write) only from Master
- ✅ Master automatically replicates to Slaves
- ✅ Slaves are strictly Read-Only

---

## 📂 Files Structure

```
.
├── master.go        # Master node logic
├── slave.go         # Slave node logic
├── README.md
```

---

## 🧠 Design Choices

- Uses plain HTTP instead of Message Queue (MQ) for simplicity.
- Commands are sent in JSON format over POST requests.
- Slaves are designed to reject direct write operations to enforce consistency.
