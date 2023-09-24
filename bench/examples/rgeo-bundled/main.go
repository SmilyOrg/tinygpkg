package main

import (
	"github.com/sams96/rgeo"
)

func main() {
	_, err := rgeo.New(rgeo.Cities10)
	if err != nil {
		panic(err)
	}
}
