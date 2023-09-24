package main

import (
	"context"
	"testing"

	"github.com/golang/geo/s2"
	"github.com/sams96/rgeo"
	"github.com/smilyorg/tinygpkg/gpkg"
	"github.com/twpayne/go-geom"
)

type testCase struct {
	name     string
	l        s2.LatLng
	want     string
	notFound bool
}

var countries = []testCase{
	{
		name: "Slovenia",
		l:    s2.LatLngFromDegrees(46.1512, 14.9955),
		want: "Slovenia",
	},
	{
		name: "Croatia/Slovenia concave check",
		l:    s2.LatLngFromDegrees(45.5377, 14.5958),
		want: "Croatia",
	},
	{
		name: "Germany",
		l:    s2.LatLngFromDegrees(52.5200, 13.4050),
		want: "Germany",
	},
	{
		name: "France/Germany concave check",
		l:    s2.LatLngFromDegrees(48.902, 7.701),
		want: "France",
	},
	{
		name:     "Italy concave not found check",
		l:        s2.LatLngFromDegrees(45.5482, 13.1011),
		notFound: true,
	},
	{
		name: "France",
		l:    s2.LatLngFromDegrees(48.8566, 2.3522),
		want: "France",
	},
	{
		name: "United Kingdom",
		l:    s2.LatLngFromDegrees(51.5074, 0.1278),
		want: "United Kingdom",
	},
	{
		name: "United States of America",
		l:    s2.LatLngFromDegrees(40.7128, -74.0060),
		want: "United States of America",
	},
	{
		name: "Australia",
		l:    s2.LatLngFromDegrees(-33.8688, 151.2093),
		want: "Australia",
	},
	{
		name:     "NotFound",
		l:        s2.LatLngFromDegrees(0, 0),
		notFound: true,
	},
	{
		name:     "NotFoundPacific",
		l:        s2.LatLngFromDegrees(0, 180),
		notFound: true,
	},
	{
		name:     "NotFoundAtlantic",
		l:        s2.LatLngFromDegrees(0, -180),
		notFound: true,
	},
}

var cities = []testCase{
	{
		name: "New York",
		l:    s2.LatLngFromDegrees(40.7128, -74.0060),
		want: "New York",
	},
	{
		name: "Ljubljana",
		l:    s2.LatLngFromDegrees(46.0569, 14.5058),
		want: "Ljubljana",
	},
	{
		name: "Paris",
		l:    s2.LatLngFromDegrees(48.8566, 2.3522),
		want: "Paris",
	},
	{
		name: "Berlin",
		l:    s2.LatLngFromDegrees(52.5200, 13.4050),
		want: "Berlin",
	},
	{
		name: "Tokyo",
		l:    s2.LatLngFromDegrees(35.6895, 139.6917),
		want: "Tokyo",
	},
	{
		name:     "NotFound",
		l:        s2.LatLngFromDegrees(0, 0),
		notFound: true,
	},
	{
		name:     "NotFoundPacific",
		l:        s2.LatLngFromDegrees(0, 180),
		notFound: true,
	},
	{
		name:     "NotFoundAtlantic",
		l:        s2.LatLngFromDegrees(0, -180),
		notFound: true,
	},
}

var datasets = []struct {
	name      string
	gpkg      string
	table     string
	nameCol   string
	rgeo      func() []byte
	rgeoLoc   func(rgeo.Location) string
	testCases []testCase
}{
	{
		name:      "countries",
		gpkg:      "../testdata/ne_110m_admin_0_countries_s4_twkb_p3.gpkg",
		table:     "ne_110m_admin_0_countries",
		nameCol:   "NAME",
		rgeo:      rgeo.Countries110,
		rgeoLoc:   func(l rgeo.Location) string { return l.Country },
		testCases: countries,
	},
	{
		name:      "cities",
		gpkg:      "../testdata/ne_10m_urban_areas_landscan_s4_twkb_p3.gpkg",
		table:     "ne_10m_urban_areas_landscan",
		nameCol:   "name_conve",
		rgeo:      rgeo.Cities10,
		rgeoLoc:   func(l rgeo.Location) string { return l.City },
		testCases: cities,
	},
}

func TestReverseGeocode(t *testing.T) {
	for _, ds := range datasets {
		t.Run(ds.name, func(t *testing.T) {
			g, err := gpkg.Open(ds.gpkg, ds.table, ds.nameCol)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()

			rg, err := rgeo.New(ds.rgeo)
			if err != nil {
				t.Fatal(err)
			}

			for _, tc := range ds.testCases {
				t.Run(tc.name, func(t *testing.T) {
					c := geom.Coord{tc.l.Lng.Degrees(), tc.l.Lat.Degrees()}

					t.Run("tinygpkg", func(t *testing.T) {
						got, err := g.ReverseGeocode(context.Background(), tc.l)
						if tc.notFound && err != gpkg.ErrNotFound {
							t.Fatalf("got %v, want ErrNotFound", got)
						} else if !tc.notFound && err != nil {
							t.Fatal(err)
						}
						if got != tc.want {
							t.Errorf("got %q, want %q", got, tc.want)
						}
					})

					t.Run("rgeo", func(t *testing.T) {
						loc, err := rg.ReverseGeocode(c)
						if tc.notFound && err != rgeo.ErrLocationNotFound {
							t.Fatalf("got %v, want ErrLocationNotFound", loc)
						} else if !tc.notFound && err != nil {
							t.Fatal(err)
						}
						got := ds.rgeoLoc(loc)
						if got != tc.want {
							t.Errorf("got %q, want %q", got, tc.want)
						}
					})
				})
			}
		})
	}
}

func BenchmarkSetup(b *testing.B) {
	for _, ds := range datasets {
		b.Run("dataset="+ds.name, func(b *testing.B) {
			b.Run("lib=rgeo", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, err := rgeo.New(ds.rgeo)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("lib=tinygpkg", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					g, err := gpkg.Open(ds.gpkg, ds.table, ds.nameCol)
					if err != nil {
						b.Fatal(err)
					}
					g.Close()
				}
			})
		})
	}
}

func BenchmarkReverseGeocode(b *testing.B) {
	latlng := s2.LatLngFromDegrees(40.7128, -74.0060)
	coord := geom.Coord{latlng.Lng.Degrees(), latlng.Lat.Degrees()}

	for _, ds := range datasets {
		b.Run("dataset="+ds.name, func(b *testing.B) {

			b.Run("lib=rgeo", func(b *testing.B) {
				rg, err := rgeo.New(ds.rgeo)
				if err != nil {
					b.Fatal(err)
				}

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err := rg.ReverseGeocode(coord)
					if err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run("lib=tinygpkg", func(b *testing.B) {
				g, err := gpkg.Open(ds.gpkg, ds.table, ds.nameCol)
				if err != nil {
					b.Fatal(err)
				}
				defer g.Close()

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, err := g.ReverseGeocode(context.Background(), latlng)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
