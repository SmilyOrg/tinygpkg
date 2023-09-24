package gpkg

import (
	"context"
	"testing"

	"github.com/golang/geo/s2"
)

type testCase struct {
	name     string
	l        s2.LatLng
	want     string
	notFound bool
}

var countriesFull = []testCase{
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

var countriesThreeLetterCode = []testCase{
	{
		name: "Slovenia",
		l:    s2.LatLngFromDegrees(46.1512, 14.9955),
		want: "SVN",
	},
	{
		name: "Germany",
		l:    s2.LatLngFromDegrees(52.5200, 13.4050),
		want: "DEU",
	},
	{
		name: "France",
		l:    s2.LatLngFromDegrees(48.8566, 2.3522),
		want: "FRA",
	},
	{
		name: "United Kingdom",
		l:    s2.LatLngFromDegrees(51.5074, 0.1278),
		want: "GBR",
	},
	{
		name: "United States of America",
		l:    s2.LatLngFromDegrees(40.7128, -74.0060),
		want: "USA",
	},
	{
		name: "Australia",
		l:    s2.LatLngFromDegrees(-33.8688, 151.2093),
		want: "AUS",
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
		name: "London",
		l:    s2.LatLngFromDegrees(51.514039, -0.092758),
		want: "London2",
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

var citiesAdm2 = []testCase{
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
		name: "London",
		l:    s2.LatLngFromDegrees(51.514039, -0.092758),
		want: "City of London",
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

var geopackages = []struct {
	name      string
	path      string
	table     string
	nameCol   string
	testCases []testCase
}{
	{
		name:      "ne countries makevalid",
		path:      "../testdata/ne_110m_admin_0_countries_makevalid.gpkg",
		table:     "ne_110m_admin_0_countries",
		nameCol:   "NAME",
		testCases: countriesFull,
	},
	{
		name:      "ne countries s4",
		path:      "../testdata/ne_110m_admin_0_countries_s4.gpkg",
		table:     "ne_110m_admin_0_countries",
		nameCol:   "NAME",
		testCases: countriesFull,
	},
	{
		name:      "ne countries s4_twkb_p3",
		path:      "../testdata/ne_110m_admin_0_countries_s4_twkb_p3.gpkg",
		table:     "ne_110m_admin_0_countries",
		nameCol:   "NAME",
		testCases: countriesFull,
	},
	{
		name:      "ne cities makevalid",
		path:      "../testdata/ne_10m_urban_areas_landscan_makevalid.gpkg",
		table:     "ne_10m_urban_areas_landscan",
		nameCol:   "name_conve",
		testCases: cities,
	},
	{
		name:      "ne cities s4",
		path:      "../testdata/ne_10m_urban_areas_landscan_s4.gpkg",
		table:     "ne_10m_urban_areas_landscan",
		nameCol:   "name_conve",
		testCases: cities,
	},
	{
		name:      "ne cities s4_twkb_p3",
		path:      "../testdata/ne_10m_urban_areas_landscan_s4_twkb_p3.gpkg",
		table:     "ne_10m_urban_areas_landscan",
		nameCol:   "name_conve",
		testCases: cities,
	},
	// {
	// 	name:      "geoboundaries makevalid",
	// 	path:      "../testdata/geoBoundariesCGAZ_ADM0_makevalid.gpkg",
	// 	table:     "globalADM0",
	// 	nameCol:   "shapeGroup",
	// 	testCases: countriesThreeLetterCode,
	// },
	// {
	// 	name:      "geoboundaries s4",
	// 	path:      "../testdata/geoBoundariesCGAZ_ADM0_s4.gpkg",
	// 	table:     "globalADM0",
	// 	nameCol:   "shapeGroup",
	// 	testCases: countriesThreeLetterCode,
	// },
	// {
	// 	name:      "geoboundaries s4_twkb_p3",
	// 	path:      "../testdata/geoBoundariesCGAZ_ADM0_s4_twkb_p3.gpkg",
	// 	table:     "globalADM0",
	// 	nameCol:   "shapeGroup",
	// 	testCases: countriesThreeLetterCode,
	// },
	{
		name:      "geoboundaries adm2 makevalid",
		path:      "../testdata/geoBoundariesCGAZ_ADM2_makevalid.gpkg",
		table:     "globalADM2",
		nameCol:   "shapeName",
		testCases: citiesAdm2,
	},
	{
		name:      "geoboundaries adm2 s4",
		path:      "../testdata/geoBoundariesCGAZ_ADM2_s4.gpkg",
		table:     "globalADM2",
		nameCol:   "shapeName",
		testCases: citiesAdm2,
	},
	{
		name:      "geoboundaries adm2 s4_twkb_p3",
		path:      "../testdata/geoBoundariesCGAZ_ADM2_s4_twkb_p3.gpkg",
		table:     "globalADM2",
		nameCol:   "shapeName",
		testCases: citiesAdm2,
	},
}

func TestReverseGeocode(t *testing.T) {

	for _, db := range geopackages {
		t.Run(db.name, func(t *testing.T) {
			g, err := Open(db.path, db.table, db.nameCol)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()

			for _, tc := range db.testCases {
				t.Run(tc.name, func(t *testing.T) {
					got, err := g.ReverseGeocode(context.Background(), tc.l)
					if tc.notFound && err != ErrNotFound {
						t.Fatalf("got %v, want ErrNotFound", got)
					} else if !tc.notFound && err != nil {
						t.Fatal(err)
					}
					if got != tc.want {
						t.Errorf("got %q, want %q", got, tc.want)
					}
				})
			}
		})
	}
}

func BenchmarkReverseGeocode(b *testing.B) {
	for _, opts := range []struct {
		Validate bool
	}{
		{false},
		{true},
	} {
		name := "none"
		if opts.Validate {
			name = "validate"
		}
		b.Run("opts="+name, func(b *testing.B) {
			for _, db := range geopackages {
				b.Run("dataset="+db.name, func(b *testing.B) {
					g, err := Open(db.path, db.table, db.nameCol)
					if err != nil {
						b.Fatal(err)
					}
					g.Validate = opts.Validate
					defer g.Close()

					latlng := s2.LatLngFromDegrees(40.7128, -74.0060)

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_, err := g.ReverseGeocode(context.Background(), latlng)
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
