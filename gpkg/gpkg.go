package gpkg

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/golang/geo/s2"
	"github.com/peterstace/simplefeatures/geom"
	"github.com/smilyorg/tinygpkg/binary"
	"zombiezen.com/go/sqlite/sqlitex"
)

var ErrNotFound = errors.New("not found")
var poolSize = 10

var skipValidationOpts = []geom.ConstructorOption{
	geom.DisableAllValidations,
}

type Direction string

const (
	Asc  Direction = "ASC"
	Desc Direction = "DESC"
)

type Order struct {
	Column    string
	Direction Direction
}

type GeoPackage struct {
	pool     *sqlitex.Pool
	table    string
	nameCol  string
	Order    Order
	Validate bool
}

func Open(path, table, nameCol string) (*GeoPackage, error) {
	g := &GeoPackage{}
	var err error
	g.table = table
	g.nameCol = nameCol
	g.pool, err = sqlitex.Open(path, 0, poolSize)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (g *GeoPackage) Close() error {
	return g.pool.Close()
}

func (g *GeoPackage) ReverseGeocode(ctx context.Context, l s2.LatLng) (string, error) {
	conn := g.pool.Get(ctx)
	defer g.pool.Put(conn)

	sql := `
		SELECT ` + g.nameCol + `, geom
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
			return "", errors.New("invalid order direction")
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
			return "", err
		} else if !exists {
			break
		}

		name := stmt.ColumnText(0)
		r := stmt.ColumnReader(1)
		g, err := readGeometry(r, opts)
		if err != nil {
			return "", err
		}
		p, err := geom.NewPoint(geom.Coordinates{
			XY: geom.XY{
				X: l.Lng.Degrees(),
				Y: l.Lat.Degrees(),
			},
		})
		if err != nil {
			return "", err
		}
		contains := geom.Intersects(g, p.AsGeometry())
		if contains {
			return name, nil
		}

	}

	return "", ErrNotFound
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
