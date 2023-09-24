package main

import (
	"context"

	"github.com/golang/geo/s2"
	"github.com/smilyorg/tinygpkg/gpkg"
)

func main() {
	// Open GeoPackage file
	g, err := gpkg.Open(
		// GeoPackage file path
		"../testdata/ne_110m_admin_0_countries_s4_twkb_p3.gpkg",
		// Dataset (table) name
		"ne_110m_admin_0_countries",
		// Column name to use for reverse geocoding
		"NAME",
	)
	if err != nil {
		panic(err)
	}
	defer g.Close()

	// Reverse geocode a point
	p := s2.LatLngFromDegrees(48.8566, 2.3522)
	name, err := g.ReverseGeocode(context.Background(), p)
	if err != nil {
		panic(err)
	}

	// Output: France
	println(name)
}
