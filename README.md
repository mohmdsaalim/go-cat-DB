# go-cat-DB

A lightweight, file-based JSON database engine built from scratch in Go. No external database dependencies — the filesystem itself acts as the database.

---

## Table of Contents

- [Overview](#overview)
- [Tech Stack](#tech-stack)
- [How It Works](#how-it-works)
  - [Database Structure](#database-structure)
  - [JSON Record Format](#json-record-format)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Running the Project](#running-the-project)
- [API Reference](#api-reference)
  - [New — Initialize Database](#new--initialize-database)
  - [Write — Create or Update a Record](#write--create-or-update-a-record)
  - [Read — Read a Single Record](#read--read-a-single-record)
  - [ReadAll — Read All Records in a Collection](#readall--read-all-records-in-a-collection)
  - [Delete — Delete a Record or Collection](#delete--delete-a-record-or-collection)
- [Internal Architecture](#internal-architecture)
  - [Atomic Writes](#atomic-writes)
  - [Per-Collection Mutex Locking](#per-collection-mutex-locking)
  - [Thread-Safe Mutex Management](#thread-safe-mutex-management)
- [Code Walkthrough](#code-walkthrough)
  - [Driver Struct](#driver-struct)
  - [Data Models](#data-models)
  - [Complete Flow](#complete-flow)
- [Project Structure](#project-structure)
- [License](#license)

---

## Overview

cat-DB is a mini database engine that stores structured data as JSON files on disk. It is inspired by how NoSQL databases like MongoDB organize data into collections and documents, but uses the local filesystem as the storage backend.

The project demonstrates core database concepts including:

- CRUD operations (Create, Read, Update, Delete)
- Atomic writes for data integrity
- Concurrency safety with mutex-based locking
- Structured logging for observability

This is not meant to replace production databases. It is a learning project that shows how database fundamentals work under the hood.

---

## Tech Stack

| Technology | Purpose |
|---|---|
| **Go (Golang)** | Core language — chosen for its strong concurrency primitives, fast compilation, and excellent standard library |
| **JSON** | Storage format — human-readable, structured, easy to debug by opening files directly |
| **Uber Zap** | High-performance structured logger for tracking database operations |
| **sync.Mutex** | Go's built-in mutual exclusion lock for thread-safe concurrent access |
| **os / filepath** | Go standard library packages for file system operations |
| **encoding/json** | Go standard library package for JSON serialization and deserialization |

---

## How It Works

### Database Structure

Data is organized into **collections** (directories) and **records** (JSON files). This is similar to how MongoDB organizes data into collections and documents.

```
mydb/                              <-- Database root directory
  |
  └── users/                       <-- Collection (a folder)
        |
        ├── messi.json             <-- Record (a JSON file)
        ├── ronaldo.json
        ├── neymar.json
        └── lamine yamal.json
```

- The **database root** (`mydb/`) is a directory on disk. All collections live inside it.
- A **collection** is a subdirectory (e.g., `users/`). You can have as many collections as you want (e.g., `users/`, `products/`, `orders/`).
- A **record** is a single `.json` file inside a collection. The filename is the record's key/identifier.

### JSON Record Format

Each record is stored as a pretty-printed JSON file. For example, `mydb/users/messi.json` contains:

```json
{
    "name": "messi",
    "age": "37",
    "contact": "9090",
    "club": "inter miami",
    "address": {
        "city": "rosario",
        "state": "santa fe",
        "country": "argentina",
        "pincode": "678594"
    }
}
```

Because records are plain JSON files, you can inspect, edit, or back them up using any text editor or standard file tools.

---

## Getting Started

### Prerequisites

- **Go 1.21 or later** installed on your machine
- Git (to clone the repository)

### Installation

```bash
# Clone the repository
git clone https://github.com/mohmdsaalim/go-cat-DB.git

# Navigate into the project
cd go-cat-DB

# Download dependencies
go mod tidy
```

### Running the Project

```bash
# Run directly
go run main.go

# Or build and run the binary
go build -o go-cat-db .
./go-cat-db
```

When you run the project, it will:

1. Create a `mydb/` directory (the database)
2. Write 4 user records into the `users` collection
3. Read all records and display them
4. Read a single record (`messi`) and display it
5. Show the full list of users

You should see output like:

```
 All users written successfully!

 All Users:
   - lamine yamal | Age: 17 | Club: barcelona | City: esplugues, spain
   - messi | Age: 37 | Club: inter miami | City: rosario, argentina
   - neymar | Age: 32 | Club: santos | City: mogi das cruzes, brazil
   - ronaldo | Age: 39 | Club: al nassr | City: funchal, portugal

 Reading single user 'messi':
   Name: messi, Club: inter miami, Contact: 9090

 Users after deletion:
   - lamine yamal
   - messi
   - neymar
   - ronaldo
```

---

## API Reference

### New — Initialize Database

```go
func New(dir string, options *Options) (*Driver, error)
```

Creates a new database instance. If the directory does not exist, it will be created automatically.

**Parameters:**
- `dir` — Path to the database root directory (e.g., `"./mydb"`)
- `options` — Optional configuration. Pass `nil` to use defaults.

**Returns:**
- A pointer to a `Driver` instance
- An error if the directory could not be created

**Example:**

```go
db, err := New("./mydb", nil)
if err != nil {
    fmt.Println("Error:", err)
    return
}
```

**What happens internally:**
1. The directory path is cleaned using `filepath.Clean()`
2. If no logger is provided, a production Zap logger is created automatically
3. If the directory already exists, it reuses it. Otherwise, it creates it with `os.MkdirAll()`
4. Returns a `Driver` struct with an initialized mutex map

---

### Write — Create or Update a Record

```go
func (d *Driver) Write(collection, resource string, v interface{}) error
```

Saves any Go value as a JSON file inside a collection.

**Parameters:**
- `collection` — The collection name (becomes a folder, e.g., `"users"`)
- `resource` — The record name/key (becomes the filename, e.g., `"messi"`)
- `v` — Any Go value that can be serialized to JSON (struct, map, slice, etc.)

**Returns:**
- An error if the write fails, `nil` on success

**Example:**

```go
user := User{
    Name:    "messi",
    Age:     "37",
    Contact: "9090",
    Club:    "inter miami",
    Address: Address{
        City:    "rosario",
        State:   "santa fe",
        Country: "argentina",
        Pincode: "678594",
    },
}

err := db.Write("users", "messi", user)
```

**What happens internally:**
1. Validates that both `collection` and `resource` are provided
2. Acquires the mutex lock for this collection (thread safety)
3. Creates the collection directory if it does not exist
4. Marshals the Go value to pretty-printed JSON using `json.MarshalIndent()`
5. Writes the JSON to a temporary `.tmp` file
6. Atomically renames the `.tmp` file to the final `.json` file
7. Releases the mutex lock

The atomic write (step 5-6) ensures that if the process crashes mid-write, you never end up with a corrupted or partially written file.

---

### Read — Read a Single Record

```go
func (d *Driver) Read(collection, resource string, v interface{}) error
```

Reads a single JSON record from a collection and deserializes it into the provided Go pointer.

**Parameters:**
- `collection` — The collection name (e.g., `"users"`)
- `resource` — The record name/key (e.g., `"messi"`)
- `v` — A pointer to a Go value where the JSON will be unmarshalled into

**Returns:**
- An error if the record does not exist or cannot be read, `nil` on success

**Example:**

```go
var messi User
err := db.Read("users", "messi", &messi)
if err != nil {
    fmt.Println("Error:", err)
} else {
    fmt.Println(messi.Name, messi.Club) // messi inter miami
}
```

**What happens internally:**
1. Validates that both `collection` and `resource` are provided
2. Constructs the file path: `<db_dir>/<collection>/<resource>.json`
3. Checks if the file exists using `os.Stat()`
4. Reads the entire file content using `os.ReadFile()`
5. Unmarshals the JSON bytes into the provided Go pointer using `json.Unmarshal()`

---

### ReadAll — Read All Records in a Collection

```go
func (d *Driver) ReadAll(collection string) ([]string, error)
```

Reads every record in a collection and returns them as a slice of raw JSON strings.

**Parameters:**
- `collection` — The collection name (e.g., `"users"`)

**Returns:**
- A slice of strings, where each string is the raw JSON content of one record
- An error if the collection does not exist or cannot be read

**Example:**

```go
records, err := db.ReadAll("users")
if err != nil {
    fmt.Println("Error:", err)
    return
}

// Parse each raw JSON string into a struct
for _, record := range records {
    var user User
    json.Unmarshal([]byte(record), &user)
    fmt.Println(user.Name, user.Club)
}
```

**What happens internally:**
1. Validates that `collection` is provided
2. Constructs the directory path: `<db_dir>/<collection>`
3. Checks if the directory exists using `os.Stat()`
4. Lists all entries in the directory using `os.ReadDir()`
5. Iterates over each file (skipping subdirectories), reads its content, and appends it to the result slice
6. Returns the slice of raw JSON strings

---

### Delete — Delete a Record or Collection

```go
func (d *Driver) Delete(collection, resource string) error
```

Deletes a single record or an entire collection.

**Parameters:**
- `collection` — The collection name (e.g., `"users"`)
- `resource` — The record name to delete (e.g., `"ronaldo"`). Pass an empty string `""` to delete the entire collection.

**Returns:**
- An error if the record/collection does not exist, `nil` on success

**Example — Delete a single record:**

```go
err := db.Delete("users", "ronaldo")
// This removes mydb/users/ronaldo.json
```

**Example — Delete an entire collection:**

```go
err := db.Delete("users", "")
// This removes the entire mydb/users/ directory and all records inside it
```

**What happens internally:**
1. Validates that `collection` is provided
2. Acquires the mutex lock for this collection
3. If `resource` is provided:
   - Constructs the file path: `<db_dir>/<collection>/<resource>.json`
   - Checks if the file exists, then removes it with `os.Remove()`
4. If `resource` is empty:
   - Removes the entire collection directory and all its contents with `os.RemoveAll()`
5. Releases the mutex lock

---

## Internal Architecture

### Atomic Writes

One of the most important concepts in database design is ensuring that writes are **atomic** — they either fully succeed or fully fail. There should never be a half-written file.

go-cat-DB achieves this with a two-step process:

```
Step 1: Write data to a temporary file    →  messi.json.tmp
Step 2: Rename temp file to final name    →  messi.json
```

The `os.Rename()` operation is atomic on most filesystems. This means:
- If the process crashes during Step 1, only the `.tmp` file is affected. The original `.json` file (if it existed) remains intact.
- If the process crashes during Step 2, the rename either happened or it did not. There is no in-between state.

This is the same principle used by many production databases and applications (e.g., SQLite, Redis RDB snapshots).

### Per-Collection Mutex Locking

Instead of using a single global lock (which would serialize all database operations), go-cat-DB uses a **separate mutex for each collection**.

```
Global Lock (single mutex):
  Write "users/messi"    →  BLOCKS  →  Write "products/phone"

Per-Collection Lock (go-cat-DB approach):
  Write "users/messi"    →  runs concurrently with  →  Write "products/phone"
  Write "users/messi"    →  BLOCKS                  →  Write "users/ronaldo"
```

This means:
- Two writes to **different collections** can happen at the same time (no blocking)
- Two writes to the **same collection** are serialized (one waits for the other) to prevent race conditions

This gives much better throughput compared to a single global lock.

### Thread-Safe Mutex Management

The mutexes themselves are stored in a map (`map[string]*sync.Mutex`). Creating a new mutex for a new collection must also be thread-safe — if two goroutines try to create a mutex for the same collection at the same time, they could create duplicate mutexes.

The `getOrCreateMutex()` function solves this by using a **global mutex** to protect the mutex map:

```go
func (d *Driver) getOrCreateMutex(collection string) *sync.Mutex {
    d.mutex.Lock()         // Lock the global mutex
    defer d.mutex.Unlock() // Unlock when done

    m, ok := d.mutexes[collection]
    if !ok {
        m = &sync.Mutex{}
        d.mutexes[collection] = m
    }
    return m
}
```

This is a common pattern in Go called **lazy initialization with synchronization**.

---

## Code Walkthrough

### Driver Struct

The `Driver` is the core struct that powers the database:

```go
type Driver struct {
    mutex   sync.Mutex            // Global mutex to protect the mutexes map
    mutexes map[string]*sync.Mutex // One mutex per collection
    dir     string                // Root directory of the database
    log     *zap.Logger           // Structured logger
}
```

| Field | Type | Purpose |
|---|---|---|
| `mutex` | `sync.Mutex` | Protects the `mutexes` map from concurrent access |
| `mutexes` | `map[string]*sync.Mutex` | Stores one mutex per collection for fine-grained locking |
| `dir` | `string` | The root directory where all collections and records are stored |
| `log` | `*zap.Logger` | Uber Zap logger for structured, high-performance logging |

### Data Models

```go
type Address struct {
    City    string      `json:"city"`
    State   string      `json:"state"`
    Country string      `json:"country"`
    Pincode json.Number `json:"pincode"`
}

type User struct {
    Name    string      `json:"name"`
    Age     json.Number `json:"age"`
    Contact string      `json:"contact"`
    Club    string      `json:"club"`
    Address Address     `json:"address"`
}
```

- JSON struct tags (`json:"name"`) ensure that the JSON keys are lowercase, following standard JSON conventions.
- `json.Number` is used instead of `int` or `string` to preserve the original number format from JSON without type conversion issues.

### Complete Flow

Here is the full lifecycle of data in go-cat-DB:

```
1. Initialize
   New("./mydb", nil)
   └── Creates the "mydb" directory if it does not exist
   └── Returns a Driver instance with an empty mutex map

2. Write
   db.Write("users", "messi", user)
   └── Acquires mutex for "users" collection
   └── Creates "mydb/users/" directory if needed
   └── Marshals the user struct to JSON
   └── Writes to "mydb/users/messi.json.tmp"
   └── Renames to "mydb/users/messi.json" (atomic)
   └── Releases mutex

3. Read Single
   db.Read("users", "messi", &result)
   └── Reads "mydb/users/messi.json"
   └── Unmarshals JSON into the result pointer

4. Read All
   db.ReadAll("users")
   └── Lists all files in "mydb/users/"
   └── Reads each .json file
   └── Returns all records as raw JSON strings

5. Delete Record
   db.Delete("users", "messi")
   └── Acquires mutex for "users" collection
   └── Removes "mydb/users/messi.json"
   └── Releases mutex

6. Delete Collection
   db.Delete("users", "")
   └── Acquires mutex for "users" collection
   └── Removes entire "mydb/users/" directory
   └── Releases mutex
```

---

## Project Structure

```
go-cat-DB/
  |-- main.go          Main source file containing the database engine and demo
  |-- go.mod           Go module definition
  |-- go.sum           Dependency checksums
  |-- README.md        This file
  |-- mydb/            Database directory (created at runtime)
       └── users/      Example collection
            |-- messi.json
            |-- ronaldo.json
            |-- neymar.json
            └── lamine yamal.json
```

---

## License

This project is open source and available under the [MIT License](LICENSE).

---

**Built by [mohmdsaalim](https://github.com/mohmdsaalim)**