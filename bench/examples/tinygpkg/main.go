package main

import (
	"github.com/smilyorg/tinygpkg/gpkg"
)

func main() {
	g, err := gpkg.Open("dummy", "dummy", "dummy")
	if err != nil {
		panic(err)
	}
	defer g.Close()
}
