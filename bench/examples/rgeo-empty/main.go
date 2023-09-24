package main

import (
	"github.com/sams96/rgeo"
)

func main() {
	_, err := rgeo.New()
	if err != nil {
		panic(err)
	}
}
