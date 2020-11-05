package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/PumpkinSeed/sqlfuzz/drivers"
	"github.com/brianvoe/gofakeit/v5"
	_ "github.com/go-sql-driver/mysql"
	"github.com/volatiletech/null"
)

var (
	f  flags
	db *sql.DB
)

type flags struct {
	driver drivers.Flags

	table  string
	parsed bool
}

type fieldDescriptor struct {
	Field   string
	Type    string
	Null    string
	Key     string
	Default null.String
	Extra   string
}

func main() {
	gofakeit.Seed(0)
	fields, err := describe()
	if err != nil {
		log.Fatal(err.Error())
	}

	defer db.Close()

	err = fuzz(fields)
	if err != nil {
		log.Fatal(err.Error())
	}
}

func parseFlags() {
	flag.StringVar(&f.driver.Username, "u", "fluidpay", "Username for the database connection")
	flag.StringVar(&f.driver.Password, "p", "fluidpay", "Password for the database connection")
	flag.StringVar(&f.driver.Database, "d", "fluidpay", "Database of the database connection")
	flag.StringVar(&f.driver.Host, "h", "10.0.0.7", "Host for the database connection")
	flag.StringVar(&f.driver.Port, "P", "3306", "Port for the database connection")
	flag.StringVar(&f.driver.Driver, "D", "mysql", "Driver for the database connection (mysql, postgres, etc.)")
	flag.StringVar(&f.table, "t", "transactions", "Table for fuzzing")
	flag.Parse()

	f.parsed = true
}

func connect(d drivers.Driver) {
	var err error
	db, err = sql.Open(d.Driver(), d.Connection())
	if err != nil {
		log.Fatal(err)
	}
}

func describe() ([]fieldDescriptor, error) {
	results, err := connection().Query(fmt.Sprintf("DESCRIBE %s;", flagsOut().table))
	if err != nil {
		return nil, err
	}

	var fields []fieldDescriptor
	for results.Next() {
		var d fieldDescriptor

		err = results.Scan(&d.Field, &d.Type, &d.Null, &d.Key, &d.Default, &d.Extra)
		if err != nil {
			return nil, err
		}

		fields = append(fields, d)
	}

	return fields, nil
}

func fuzz(fields []fieldDescriptor) error {
	return exec(fields)
}

func exec(fields []fieldDescriptor) error {
	driver := drivers.New(flagsOut().driver)

	var f []string
	var values []interface{}
	for _, field := range fields {
		f = append(f, field.Field)

		values = append(values, genField(driver, field.Type))
	}
	driver.Insert(f, flagsOut().table)
	ins, err := connection().Prepare(driver.Insert(f, flagsOut().table))
	if err != nil {
		log.Fatal(err)
	}

	_, err = ins.Exec(values...)
	return err
}

func genField(driver drivers.Driver, t string) interface{} {
	typ, options := driver.MapField(t)
	switch typ {
	case drivers.String:
		return randomString(2)
	case drivers.Uint:
		return gofakeit.Number(1, 200)
	case drivers.Enum:
		return options[gofakeit.Number(0, len(options)-1)]
	case drivers.Bool:
		if gofakeit.Number(1, 200)%2 == 0 {
			return true
		}
		return false
	case drivers.Json:
		return fmt.Sprintf(
			`{"%s": "%s", "%s": "%s"}`,
			gofakeit.Password(true, true, false, false, false, 6),
			gofakeit.Password(true, true, false, false, false, 6),
			gofakeit.Password(true, true, false, false, false, 6),
			gofakeit.Password(true, true, false, false, false, 6),
		)
	case drivers.Time:
		return gofakeit.Date()
	case drivers.Unknown:
		log.Fatalf("Unknown field type: %s", t)
	}

	return nil
}

func flagsOut() flags {
	if !f.parsed {
		parseFlags()
	}

	return f
}

func connection() *sql.DB {
	if db == nil {
		connect(drivers.New(flagsOut().driver))
	}

	return db
}

func randomString(length int) string {
	var charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	var seededRand = rand.New(
		rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b)
}