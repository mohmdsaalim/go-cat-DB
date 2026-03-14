package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
)

const Version = "1.0.0"

// Driver is the core struct that acts as the database engine.
// It stores JSON files organized by collection/resource inside a directory.
type Driver struct {
	mutex   sync.Mutex             // global mutex to protect the map
	mutexes map[string]*sync.Mutex // percollection mutexes
	dir     string
	log     *zap.Logger
}

// Options holds optional configuration for the Driver.
type Options struct {
	Logger *zap.Logger
}

// New creates or opens a database at the given directory.
// If the directory doesn't exist, it will be created.
func New(dir string, options *Options) (*Driver, error) {

	dir = filepath.Clean(dir)

	opts := Options{}

	if options != nil {
		opts = *options
	}

	if opts.Logger == nil {
		logger, err := zap.NewProduction()
		if err != nil {
			return nil, err
		}
		opts.Logger = logger
	}

	driver := Driver{
		dir:     dir,
		mutexes: make(map[string]*sync.Mutex),
		log:     opts.Logger,
	}

	// If directory already exists, reuse it
	if _, err := os.Stat(dir); err == nil {
		opts.Logger.Debug(
			"Using existing database",
			zap.String("directory", dir),
		)
		return &driver, nil
	}

	// Create the database directory
	opts.Logger.Info(
		"Created new database directory",
		zap.String("directory", dir),
	)

	return &driver, os.MkdirAll(dir, 0755)
}

// Write saves a resource (record) into a collection as a JSON file.
//
//	collection = folder name (e.g. "users")
//	resource   = file name / key (e.g. "messi")
//	v          = any Go value that can be marshalled to JSON
func (d *Driver) Write(collection, resource string, v interface{}) error {

	if collection == "" {
		return fmt.Errorf("missing collection - no place to save record")
	}
	if resource == "" {
		return fmt.Errorf("missing resource - no name for the record")
	}

	mutex := d.getOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()

	dir := filepath.Join(d.dir, collection)
	fnlPath := filepath.Join(dir, resource+".json")
	tmpPath := fnlPath + ".tmp"

	// Create the collection directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal the value to pretty JSON
	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}

	// Write to a temp file first, then rename (atomic write)
	b = append(b, byte('\n'))
	if err := os.WriteFile(tmpPath, b, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, fnlPath)
}

// Update modifies an existing resource in a collection.
// It reads the current record, merges the provided updates on top (partial update),
// and writes the result back atomically using temp file → rename.
//
//	collection = folder name (e.g. "users")
//	resource   = file name / key (e.g. "messi")
//	updates    = any Go value whose JSON fields will be merged on top of the existing record
func (d *Driver) Update(collection, resource string, updates interface{}) error {

	if collection == "" {
		return fmt.Errorf("missing collection - unable to update")
	}
	if resource == "" {
		return fmt.Errorf("missing resource - unable to update record (no name)")
	}

	mutex := d.getOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()

	dir := filepath.Join(d.dir, collection)
	fnlPath := filepath.Join(dir, resource+".json")
	tmpPath := fnlPath + ".tmp"

	// ── Step 1: Read the existing record ─────────────────────
	if _, err := os.Stat(fnlPath); err != nil {
		return fmt.Errorf("record %s/%s does not exist - cannot update", collection, resource)
	}

	existingBytes, err := os.ReadFile(fnlPath)
	if err != nil {
		return fmt.Errorf("failed to read existing record: %w", err)
	}

	// ── Step 2: Unmarshal existing record into a generic map ─
	var existingMap map[string]interface{}
	if err := json.Unmarshal(existingBytes, &existingMap); err != nil {
		return fmt.Errorf("failed to parse existing record: %w", err)
	}

	// ── Step 3: Marshal the updates and unmarshal into a map ─
	updateBytes, err := json.Marshal(updates)
	if err != nil {
		return fmt.Errorf("failed to marshal updates: %w", err)
	}

	var updateMap map[string]interface{}
	if err := json.Unmarshal(updateBytes, &updateMap); err != nil {
		return fmt.Errorf("failed to parse updates: %w", err)
	}

	// ── Step 4: Merge — update fields overwrite existing ones ─
	for key, value := range updateMap {
		existingMap[key] = value
	}

	// ── Step 5: Marshal the merged result to pretty JSON ─────
	mergedBytes, err := json.MarshalIndent(existingMap, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal merged record: %w", err)
	}

	// ── Step 6: Atomic write — temp file → rename ────────────
	mergedBytes = append(mergedBytes, byte('\n'))
	if err := os.WriteFile(tmpPath, mergedBytes, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	return os.Rename(tmpPath, fnlPath)
}

// Read loads a single resource from a collection into the provided pointer v.
//
//	collection = folder name (e.g. "users")
//	resource   = file name / key (e.g. "messi")
//	v          = pointer to a Go value to unmarshal the JSON into
func (d *Driver) Read(collection, resource string, v interface{}) error {

	if collection == "" {
		return fmt.Errorf("missing collection - unable to read")
	}
	if resource == "" {
		return fmt.Errorf("missing resource - unable to read record (no name)")
	}

	record := filepath.Join(d.dir, collection, resource+".json")

	if _, err := os.Stat(record); err != nil {
		return fmt.Errorf("record %s/%s does not exist", collection, resource)
	}

	b, err := os.ReadFile(record)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, v)
}

// ReadAll reads every record in a collection and returns them as a slice of
// raw JSON strings.
func (d *Driver) ReadAll(collection string) ([]string, error) {

	if collection == "" {
		return nil, fmt.Errorf("missing collection - unable to read")
	}

	dir := filepath.Join(d.dir, collection)

	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("collection %s does not exist", collection)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var records []string

	for _, file := range files {
		// skip directories and non-json files
		if file.IsDir() {
			continue
		}

		b, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return nil, err
		}

		records = append(records, string(b))
	}

	return records, nil
}

// Delete removes a resource from a collection, or an entire collection
// if the resource is empty.
func (d *Driver) Delete(collection, resource string) error {

	if collection == "" {
		return fmt.Errorf("missing collection - unable to delete")
	}

	path := filepath.Join(d.dir, collection)

	mutex := d.getOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()

	// If resource is provided, delete that single record
	if resource != "" {
		path = filepath.Join(path, resource+".json")

		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("record %s/%s does not exist", collection, resource)
		}

		return os.Remove(path)
	}

	// If no resource, delete the entire collection directory
	return os.RemoveAll(path)
}

// getOrCreateMutex returns (or creates) a mutex for the given collection
// so that concurrent writes to the same collection are safe.
func (d *Driver) getOrCreateMutex(collection string) *sync.Mutex {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	m, ok := d.mutexes[collection]
	if !ok {
		m = &sync.Mutex{}
		d.mutexes[collection] = m
	}
	return m
}

//  Data Models

// Address represents a user's address.
type Address struct {
	City    string      `json:"city"`
	State   string      `json:"state"`
	Country string      `json:"country"`
	Pincode json.Number `json:"pincode"`
}

// User represents a person stored in the database.
type User struct {
	Name    string      `json:"name"`
	Age     json.Number `json:"age"`
	Contact string      `json:"contact"`
	Club    string      `json:"club"`
	Address Address     `json:"address"`
}

//  Main — demo usage

func main() {

	dir := "./mydb" // database directory

	db, err := New(dir, nil)
	if err != nil {
		fmt.Println("Error", err)
		return
	}

	// ── 1. Write some records ────────────────────────────────
	employees := []User{
		{"messi", "37", "9090", "inter miami", Address{"rosario", "santa fe", "argentina", "678594"}},
		{"ronaldo", "39", "9090", "al nassr", Address{"funchal", "madeira", "portugal", "123345"}},
		{"neymar", "32", "9090", "santos", Address{"mogi das cruzes", "são paulo", "brazil", "6782194"}},
		{"lamine yamal", "17", "9090", "barcelona", Address{"esplugues", "catalonia", "spain", "673294"}},
	}

	for _, value := range employees {
		if err := db.Write("users", value.Name, User{
			Name:    value.Name,
			Age:     value.Age,
			Contact: value.Contact,
			Club:    value.Club,
			Address: value.Address,
		}); err != nil {
			fmt.Println("Error writing", value.Name, ":", err)
		}
	}
	fmt.Println(" All users written successfully!")

	// ── 2. Read all records ──────────────────────────────────
	records, err := db.ReadAll("users")
	if err != nil {
		fmt.Println("Error reading all:", err)
		return
	}

	allUsers := []User{}
	for _, f := range records {
		employeeFound := User{}
		if err := json.Unmarshal([]byte(f), &employeeFound); err != nil {
			fmt.Println("Error unmarshalling:", err)
		}
		allUsers = append(allUsers, employeeFound)
	}

	fmt.Println("\n All Users:")
	for _, u := range allUsers {
		fmt.Printf("   • %s | Age: %s | Club: %s | City: %s, %s\n",
			u.Name, u.Age, u.Club, u.Address.City, u.Address.Country)
	}

	// ── 3. Read a single record ──────────────────────────────
	fmt.Println("\n Reading single user 'messi':")
	var messi User
	if err := db.Read("users", "messi", &messi); err != nil {
		fmt.Println("Error reading messi:", err)
	} else {
		fmt.Printf("   Name: %s, Club: %s, Contact: %s\n", messi.Name, messi.Club, messi.Contact)
	}

	// ── 4. Update a record (partial update) ─────────────────
	fmt.Println("\n Updating messi's club to 'argentina national team'...")
	if err := db.Update("users", "messi", map[string]interface{}{
		"club":    "argentina national team",
		"contact": "10000",
	}); err != nil {
		fmt.Println("Error updating:", err)
	} else {
		fmt.Println("Updated successfully!")
	}

	// Verify the update
	fmt.Println("\n Reading updated 'messi':")
	var updatedMessi User
	if err := db.Read("users", "messi", &updatedMessi); err != nil {
		fmt.Println("Error reading:", err)
	} else {
		fmt.Printf("   Name: %s, Club: %s, Contact: %s\n", updatedMessi.Name, updatedMessi.Club, updatedMessi.Contact)
	}

	// ── 5. Delete a single record ────────────────────────────

	// ── 6. Verify deletion ───────────────────────────────────
	fmt.Println("\n Users after deletion:")
	records, err = db.ReadAll("users")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	for _, f := range records {
		u := User{}
		json.Unmarshal([]byte(f), &u)
		fmt.Printf("   • %s\n", u.Name)
	}

	// fmt.Println("\n  Deleting user 'ronaldo'...") //      deleting is working
	// if err := db.Delete("users", "ronaldo"); err != nil {
	// 	fmt.Println("Error deleting:", err)
	// } else {
	// 	fmt.Println("Deleted successfully!")
	// }

	// ── 7. Delete the entire collection ──────────────────────

	// fmt.Println("\n  Deleting entire 'users' collection...")
	// if err := db.Delete("users", ""); err != nil {
	// 	fmt.Println("Error:", err)
	// } else {
	// 	fmt.Println("   Entire collection deleted!")
	// }

	// fmt.Println("\n cat-DB demo complete!")
}

// imporant concepts in this projetcs

// 1. mutex and per - collection mutexes
// 2. Atomic writes -> no mid-write situation -> updated/previus -> app crash/server crash -> noproblem -> This is a professional technique used in production systems
//    redis, mongo, sqllite even git are using this techniques git commit -> atomic write
// 3. conconrency safety -> write(users) -> getorcreatemutex() -> mutex lock -> write/read -> mutex unlock -> ← (via defer) releases the lock
// 4.

// next 5. Idempotency Key Store
// It's tiny (2 services, ~200 lines total)
// Every payment company uses it
// Most devs have heard of it but never built it
// When interviewer asks "how do you handle duplicate requests?" — you have a built answer not a theoretical one
// The DSA (heap for TTL expiry) is clean and explainable in 2 minutes
