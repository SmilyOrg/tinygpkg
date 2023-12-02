package gpkg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/golang/geo/s2"
	"github.com/peterstace/simplefeatures/geom"
	"github.com/smilyorg/tinygpkg/binary"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

var ErrNotFound = errors.New("not found")
var poolSize = 10

var skipValidationOpts = []geom.ConstructorOption{
	geom.DisableAllValidations,
}

type Direction string
type FeatureId int64

const (
	Asc  Direction = "ASC"
	Desc Direction = "DESC"
)

type Order struct {
	Column    string
	Direction Direction
}

type GeometryCache interface {
	Get(fid FeatureId) (geom.Geometry, error)
	Set(fid FeatureId, g geom.Geometry) error
}

type GeoPackage struct {
	pool      *sqlitex.Pool
	table     string
	cols      []string
	colSelect string
	Order     Order
	Validate  bool
	Cache     GeometryCache
}

// Open opens a GeoPackage file at the specified path
//
// The table parameter specifies the name of the table to query,
// and the cols parameter specifies the columns to select.
// If table is an empty string, Open will attempt to auto-configure
// using the first table listed in "gpkg_contents".
//
// cols defines the columns of the table to return from ReverseGeocode.
//
// You can set the Cache field to a GeometryCache implementation to
// cache geometries in memory. This can be useful if you are querying
// similar locations multiple times.

// Warning: the table and columns are not sanitized, so they are prone
// to SQL injection attacks if provided by user input.
func Open(path, table string, cols []string) (*GeoPackage, error) {
	g := &GeoPackage{}
	var err error
	g.table = table
	g.cols = cols
	g.pool, err = sqlitex.Open(
		path,
		sqlite.OpenReadOnly|sqlite.OpenURI,
		poolSize,
	)
	if err != nil {
		return nil, err
	}
	if g.table == "" {
		if err := g.autoconfTable(); err != nil {
			g.Close()
			return nil, err
		}
	}
	if len(g.cols) == 0 {
		g.Close()
		return nil, errors.New("no columns specified")
	}
	g.colSelect = g.cols[0]
	for i := 1; i < len(g.cols); i++ {
		g.colSelect += ", " + g.cols[i]
	}
	return g, nil
}

func (g *GeoPackage) autoconfTable() error {
	conn := g.pool.Get(context.Background())
	defer g.pool.Put(conn)

	sql := `
		SELECT table_name
		FROM gpkg_contents
		LIMIT 1`

	stmt := conn.Prep(sql)
	defer stmt.Reset()

	if exists, err := stmt.Step(); err != nil {
		return fmt.Errorf("error auto-configuring table: %w", err)
	} else if !exists {
		return errors.New("error auto-configuring table: no table found")
	}

	g.table = stmt.ColumnText(0)
	if g.table == "" {
		return errors.New("error auto-configuring table: table name is empty")
	}
	return nil
}

func (g *GeoPackage) Close() error {
	if g == nil || g.pool == nil {
		return nil
	}
	err := g.pool.Close()
	if err != nil {
		return fmt.Errorf("error closing geopackage: %w", err)
	}
	g.pool = nil
	return nil
}

func (g *GeoPackage) ReverseGeocode(ctx context.Context, l s2.LatLng) ([]string, error) {
	conn := g.pool.Get(ctx)
	defer g.pool.Put(conn)

	sql := `
		SELECT fid, geom, ` + g.colSelect + `
		FROM ` + g.table + `
		WHERE fid IN (
			SELECT id
			FROM rtree_` + g.table + `_geom
			WHERE
				:x >= minx AND :x <= maxx AND
				:y >= miny AND :y <= maxy
		)`

	if g.Order.Column != "" {
		if g.Order.Direction != Asc && g.Order.Direction != Desc {
			return nil, errors.New("invalid order direction")
		}
		sql += ` ORDER BY ` + g.Order.Column + ` ` + string(g.Order.Direction)
	}

	stmt := conn.Prep(sql)
	defer stmt.Reset()

	stmt.BindFloat(1, l.Lng.Degrees())
	stmt.BindFloat(2, l.Lat.Degrees())

	var opts []geom.ConstructorOption
	if !g.Validate {
		opts = skipValidationOpts
	}

	for {
		if exists, err := stmt.Step(); err != nil {
			return nil, err
		} else if !exists {
			break
		}

		fid := FeatureId(stmt.ColumnInt64(0))

		var gm geom.Geometry
		read := true
		if g.Cache != nil {
			var err error
			gm, err = g.Cache.Get(fid)
			if err == nil {
				read = false
			}
		}
		if read {
			var err error
			r := stmt.ColumnReader(1)
			gm, err = readGeometry(r, opts)
			if err != nil {
				return nil, err
			}
			if g.Cache != nil {
				g.Cache.Set(fid, gm)
			}
		}

		p, err := geom.NewPoint(geom.Coordinates{
			XY: geom.XY{
				X: l.Lng.Degrees(),
				Y: l.Lat.Degrees(),
			},
		})
		if err != nil {
			return nil, err
		}
		contains := geom.Intersects(gm, p.AsGeometry())
		if contains {
			cols := make([]string, len(g.cols))
			for i := 0; i < len(g.cols); i++ {
				cols[i] = stmt.ColumnText(2 + i)
			}
			return cols, nil
		}
	}

	return nil, ErrNotFound
}

func readGeometry(r io.Reader, opts []geom.ConstructorOption) (geom.Geometry, error) {
	var g geom.Geometry

	h, err := binary.Read(r)
	if err != nil {
		return g, err
	}

	if h.Empty() {
		return g, nil
	}

	b, err := io.ReadAll(r)
	if err != nil {
		return g, err
	}

	switch {
	case h.Type() == binary.StandardType:
		g, err = geom.UnmarshalWKB(b, opts...)
	case h.Type() == binary.ExtendedType && bytes.Equal(h.ExtensionCode, binary.ExtensionTWKB):
		g, err = geom.UnmarshalTWKB(b, opts...)
	default:
		return g, errors.New("unsupported geometry type")
	}
	if err != nil {
		return g, err
	}

	return g, nil
}
