package main

import (
	"encoding/csv"
	"os"
	"strconv"
	"time"
)

type PersonRecord struct {
	FullName    string
	DateOfBirth time.Time
	PostalCode  string
	HouseNumber int
}

// Parse person from CSV file with name filename
func ReadPersons(filename string) ([]PersonRecord, error) {
	persons := []PersonRecord{}

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';'
	reader.Comment = '#'
	reader.FieldsPerRecord = 4
	reader.TrimLeadingSpace = true
	reader.ReuseRecord = false

	for {
		record, err := reader.Read()
		if err != nil {
			break
		}

		dateOfBirth, err := time.Parse("2006-01-02", record[1])
		if err != nil {
			return nil, err
		}
		houseNumber, err := strconv.Atoi(record[3])
		if err != nil {
			return nil, err
		}

		person := PersonRecord{
			FullName:    record[0],
			DateOfBirth: dateOfBirth,
			PostalCode:  record[2],
			HouseNumber: houseNumber,
		}
		persons = append(persons, person)
	}

	return persons, nil
}

func main() {
	persons, err := ReadPersons("persons.csv")
	if err != nil {
		panic(err)
	}

	for _, person := range persons {
		println(person.FullName)
	}
}
