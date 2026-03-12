package main

import (
	"os"
	// "database/sql/driver"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
)

const Version = "1.0.0"

type(
	Logger interface{
	Fatal(string, ...interface{})
	Error(string, ...interface{})
	Warn(string, ...interface{})
	Info(string, ...interface{})
	Debug(string, ...interface{})
	Trace(string, ...interface{})
	}

	Driver struct{
		mutex sync.Mutex
		mutexes map[string]*sync.Mutex
		dir string
		log Logger
	}
)

type Options struct {
	Logger *zap.Logger
}


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
		mutexes: map[string]*sync.Mutex{},
		log:     opts.Logger,
	}

	if _, err := os.Stat(dir); err == nil {

		opts.Logger.Debug(
			"Using existing database",
			zap.String("directory", dir),
		)

		return &driver, nil
	}

	// create directory if not exists
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	opts.Logger.Info(
		"Created new database directory",
		zap.String("directory", dir),
	)

	return &driver, nil
}

func (d *Driver ) Write()error {
	
}

func (d *Driver ) Read() error {
	
}

func (d *Driver ) ReadAll()() {
	
}

func (d *Driver ) Delete() error {
	
}

func (d *Driver ) getOrCreateMutex() *sync.Mutex {
	
}

type Address struct {
	City string
	State string
	Country string
	Pincode json.Number
}

type User struct {
	Name string
	Age json.Number
	Contact string
	Club string
	Address Address
}
func main() {

	dir := "./"

	db, err := New(dir, nil)

	if err != nil {
		fmt.Println("Error", err)
	}

	employees := []User{
		{"messi", "23", "9090", "barcalona", Address{"rosario", "rosario", "argentina", "678594"}},
		{"ronaldo", "23", "9090", "realmadrid", Address{"kakkadampoyil", "coorg", "portugal", "123345"}},
		{"neymar", "23", "9090", "santos", Address{"kurukanmola", "kerala", "india", "6782194"}},
		{"lamine yamal", "23", "9090", "barcalona", Address{"chokkanampara", "kerala", "india", "673294"}},
	}

	for _, value := range employees{
		db.Write("users", value.Name, User{
			Name: value.Name,
			Age: value.Age,
			Contact: value.Contact,
			Club: value.Club,
			Address: value.Address,
		})
	}

	records, err := db.ReadAll("users")
	if err != nil{
		fmt.Println("Error", err)
	}
	fmt.Println(records)


	allusers := []User{}

	for _, f := range records{
		employeeFound := User{}
		if err := json.Unmarshal([]byte(f), &employeeFound); err != nil{
			fmt.Println("Error", err)
		}
		allusers = append(allusers, employeeFound)
	}
	fmt.Println((allusers))

	// if err := db.Delete("users", "john"); err != nil{
	// 	fmt.Println("Errors", err)
	// }

	// if err := db.Delete("user",""); err != nil{
	// 	fmt.Println("Error", err)
	// }
}