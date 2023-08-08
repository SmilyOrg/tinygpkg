package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/bitfield/script"
	"github.com/peterstace/simplefeatures/geom"
	"github.com/smilyorg/gpkg"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

const OUTPUT_DIR = "data/"
const NATURAL_EARTH_URL = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/117488dc884bad03366ff727eca013e434615127/geojson/"
const MIN_PRECXY = -8
const MAX_PRECXY = +7

func geojsonUrl(name string) string {
	return NATURAL_EARTH_URL + name + ".geojson"
}

func download(url string, path string) (err error) {
	if script.IfExists(path).Error() == nil {
		fmt.Printf("download %s exists\n", path)
		return
	}
	fmt.Printf("download %s\n", path)
	_, err = script.
		Get(url).
		WriteFile(path)
	fmt.Printf("download %s done\n", path)
	return
}

func gdal(mount string, cmd string) error {
	abs, err := filepath.Abs(mount)
	if err != nil {
		return fmt.Errorf("unable to get absolute path: %s", err.Error())
	}

	dcmd := fmt.Sprintf(`docker run --rm -v "%s:/data/" -w /data/ ghcr.io/osgeo/gdal:alpine-small-latest %s`, abs, cmd)
	fmt.Printf("gdal %s\n", cmd)
	code, err := script.Exec(dcmd).Stdout()
	if code != 0 || err != nil {
		return fmt.Errorf("gdal failed: %d %s", code, err)
	}
	return nil
}

func rasterize(srcPath string, table string, dstPath string, w, h int, xmin, ymin, xmax, ymax float64) error {
	sdir, sfile := filepath.Split(srcPath)
	ddir, dfile := filepath.Split(dstPath)
	if sdir != ddir {
		return fmt.Errorf("files need to share the same directory: %s %s", srcPath, dstPath)
	}

	// gdal_rasterize -init 255 -burn 90 -ot Byte -ts 2073 1000 -te 0 30 33 56 -l ne_110m_admin_0_countries ne_110m_admin_0_countries_roundtrip_twkb_p3_s1.gpkg output.tiff
	cmd := fmt.Sprintf(
		"gdal_rasterize -init 255 -burn 90 -ot Byte -ts %d %d -te %f %f %f %f -l %s %s %s",
		w, h, xmin, ymin, xmax, ymax, table, sfile, dfile,
	)
	return gdal(sdir, cmd)
}

func render(gpkgPath string, table string, name string, w, h int, cx, cy, zoom float64) error {
	tifffile := name + ".tiff"
	pngfile := name + ".png"
	tiff := OUTPUT_DIR + tifffile
	png := OUTPUT_DIR + pngfile

	multisample := 4
	mw, mh := w*multisample, h*multisample

	// s :=

	ar := float64(h) / float64(w) * 360. / 180.

	p := math.Pow(2, zoom)

	xs := 360 / p * 0.5
	xmin, xmax := cx-xs, cx+xs

	ys := 180 / p * 0.5
	ymin, ymax := cy-ys*ar, cy+ys*ar

	// don't check error code as gdal returns 61 for some reason,
	// but it works anyway ¯\_(ツ)_/¯
	rasterize(gpkgPath, table, tiff, mw, mh, xmin, ymin, xmax, ymax)
	if _, err := os.Stat(tiff); err != nil {
		return fmt.Errorf("tiff does not exist after rasterize: %s", err.Error())
	}

	// it's error code 89 here for another reason ¯\_(ツ)_/¯
	gdal(OUTPUT_DIR, fmt.Sprintf(`gdal_translate -of PNG -r lanczos -outsize %d %d %s %s`, w, h, tifffile, pngfile))
	if _, err := os.Stat(png); err != nil {
		return fmt.Errorf("png does not exist after resize: %s", err.Error())
	}

	os.Remove(tiff)
	return nil
}

func convertToGeopackage(srcPath string, gpkgPath string) (err error) {
	sdir, sfile := filepath.Split(srcPath)
	gdir, gfile := filepath.Split(gpkgPath)
	if sdir != gdir {
		return fmt.Errorf("files need to share the same directory: %s %s", srcPath, gpkgPath)
	}

	abs, err := filepath.Abs(sdir)
	if err != nil {
		return
	}
	return gdal(abs, fmt.Sprintf(`ogr2ogr -makevalid -f "GPKG" "%s" "%s"`, gfile, sfile))
}

type compressOpts struct {
	name              string
	simplify          []float64
	simplifyMinPoints int
	minPrecXY         int
}

type compressJob struct {
	fid int64
	wkb []byte
	h   gpkg.Header
}

type compressInfo struct {
	debug            string
	usedSimplify     float64
	usedPrecXY       int
	originalSize     int
	originalPoints   int
	simplifiedPoints int
}

type writeJob struct {
	name string
	fid  int64
	twkb []byte
	h    gpkg.Header
	info compressInfo
}

func compressor(wg *sync.WaitGroup, in <-chan compressJob, out chan<- writeJob, opts compressOpts) {
	for job := range in {
		twkb, info, err := compressGeometry(job.wkb, opts)
		if err != nil {
			panic(err)
		}
		if twkb == nil {
			continue
		}
		out <- writeJob{opts.name, job.fid, twkb, job.h, info}
	}
	wg.Done()
}

func writer(wg *sync.WaitGroup, writec *sqlite.Conn, table string, in <-chan writeJob) {
	writes := writec.Prep(fmt.Sprintf(`
		UPDATE %s
		SET geom = ?
		WHERE fid = ?`,
		table,
	))
	for job := range in {
		fid := job.fid
		g := job.h
		twkb := job.twkb
		info := job.info

		g.SetType(gpkg.ExtendedType)
		g.SetEnvelopeContentsIndicatorCode(gpkg.NoEnvelope)

		// Write TWKB
		w := new(bytes.Buffer)
		g.Write(w)
		w.Write([]byte{'T', 'W', 'K', 'B'})
		io.Copy(w, bytes.NewReader(twkb))

		buf := w.Bytes()
		writes.BindBytes(1, buf)
		writes.BindInt64(2, fid)
		_, err := writes.Step()
		if err != nil {
			panic(fmt.Errorf("unable to write twkb: %s", err.Error()))
		}
		err = writes.Reset()
		if err != nil {
			panic(fmt.Errorf("unable to reset write statement: %s", err.Error()))
		}

		fmt.Printf(
			"compress %10s fid %6d %.5f simplify %6d to %6d points %7d wkb bytes %7d twkb bytes %4.0f%% at %d precXY %6d bytes written %s\n",
			job.name, fid, info.usedSimplify, info.originalPoints, info.simplifiedPoints, info.originalSize, len(twkb), 100.*float32(len(twkb))/float32(info.originalSize), info.usedPrecXY, len(buf), info.debug,
		)
	}
	wg.Done()
}

func compressGeopackage(gpkgPath, tgpkgPath, table string, opts compressOpts) error {
	_, err := script.Exec(fmt.Sprintf(`cp "%s" "%s"`, gpkgPath, tgpkgPath)).Stdout()
	if err != nil {
		return err
	}
	pool, err := sqlitex.Open(tgpkgPath, 0, 2)
	if err != nil {
		return err
	}
	defer pool.Close()

	readc := pool.Get(context.Background())
	defer pool.Put(readc)

	reads := readc.Prep(fmt.Sprintf(`
		SELECT fid, geom
		FROM %s`,
		table,
	))
	defer reads.Reset()

	writec := pool.Get(context.Background())
	defer pool.Put(writec)

	err = sqlitex.ExecuteScript(writec, fmt.Sprintf(`
		INSERT OR IGNORE INTO gpkg_extensions VALUES ('%[1]s', 'geom', 'mlunar_twkb', 'https://github.com/SmilyOrg/tinygpkg', 'read-write');
		DROP TRIGGER IF EXISTS rtree_%[1]s_geom_update1;
		DROP TRIGGER IF EXISTS rtree_%[1]s_geom_update2;
		DROP TRIGGER IF EXISTS rtree_%[1]s_geom_update3;
		DROP TRIGGER IF EXISTS rtree_%[1]s_geom_update4;
	`, table), nil)
	if err != nil {
		return err
	}

	sqlitex.Execute(writec, "BEGIN TRANSACTION;", nil)

	compressChan := make(chan compressJob, 10)
	writeChan := make(chan writeJob, 10)
	cwg := &sync.WaitGroup{}
	for i := 0; i < runtime.NumCPU(); i++ {
		cwg.Add(1)
		go compressor(cwg, compressChan, writeChan, opts)
	}
	wwg := &sync.WaitGroup{}
	wwg.Add(1)
	go writer(wwg, writec, table, writeChan)

	for {
		if exists, err := reads.Step(); err != nil {
			return fmt.Errorf("error listing geometry: %s", err.Error())
		} else if !exists {
			break
		}

		fid := reads.ColumnInt64(0)
		r := reads.ColumnReader(1)
		g, err := gpkg.Read(r)
		if err != nil {
			return fmt.Errorf("error reading gpkg: %s", err.Error())
		}

		if g.Empty() {
			fmt.Printf("compress fid %6d empty, skipping\n", fid)
			continue
		}

		if g.Type() != gpkg.StandardType {
			fmt.Printf("compress fid %6d non-standard geometry, skipping\n", fid)
			continue
		}

		wkb, err := io.ReadAll(r)
		if err != nil {
			return fmt.Errorf("error reading geometry: %s", err.Error())
		}

		compressChan <- compressJob{
			fid: fid,
			wkb: wkb,
			h:   *g,
		}

		// twkb, info, err := compressGeometry(wkb, opts)
		// if err != nil {
		// 	return fmt.Errorf("error compressing geometry: %s", err.Error())
		// }
	}

	close(compressChan)
	cwg.Wait()
	close(writeChan)
	wwg.Wait()

	err = sqlitex.Execute(writec, "COMMIT;", nil)
	if err != nil {
		return fmt.Errorf("unable to commit: %s", err.Error())
	}

	err = sqlitex.Execute(writec, "VACUUM;", nil)
	if err != nil {
		return fmt.Errorf("unable to vacuum: %s", err.Error())
	}

	return nil
}

func compressGeometry(wkb []byte, opts compressOpts) (twkb []byte, info compressInfo, err error) {
	info.originalSize = len(wkb)

	gm, err := geom.UnmarshalWKB(wkb)
	if err != nil {
		return nil, info, fmt.Errorf("error unmarshalling wkb: %s", err.Error())
	}

	var extra []string

	info.originalPoints = gm.DumpCoordinates().Length()
	info.simplifiedPoints = info.originalPoints
	info.usedSimplify = 0.0
	if info.originalPoints < opts.simplifyMinPoints {
		extra = append(extra, "not simplified min points")
	} else {
		si := 0
		for ; si < len(opts.simplify); si++ {
			info.usedSimplify = opts.simplify[si]
			gms, err := gm.Simplify(info.usedSimplify)
			if err == nil {
				points := gms.DumpCoordinates().Length()
				if points >= 3 {
					gm = gms
					info.simplifiedPoints = points
					break
				}
				extra = append(extra, "not simplified collapse")
				break
			}
		}
		if si == len(opts.simplify) {
			extra = append(extra, "not simplified max level")
		} else if si > 0 {
			extra = append(extra, "simplify fallback")
		}
	}

	info.usedPrecXY = opts.minPrecXY
	for ; info.usedPrecXY <= MAX_PRECXY; info.usedPrecXY++ {
		twkb, err = geom.MarshalTWKB(gm, info.usedPrecXY)
		if err != nil {
			return nil, info, fmt.Errorf("error marshalling twkb: %s", err.Error())
		}

		_, err = geom.UnmarshalTWKB(twkb)
		if err == nil {
			break
		}
		// if info.usedPrecXY >= MAX_PRECXY {
		// 	extra = append(extra, "unable to compress, skipping")
		// 	twkb = nil
		// 	// return nil, info, fmt.Errorf("error marshalling twkb: unable to roundtrip at any precision: %s", err.Error())
		// 	break
		// }
		// fmt.Printf("compress fid %6d roundtrip error, falling back to higher precision, precxy %d: %s\n", fid, prec, err.Error())
	}

	if info.usedPrecXY >= MAX_PRECXY {
		extra = append(extra, "unable to compress, skipping")
		twkb = nil
	} else if info.usedPrecXY != opts.minPrecXY {
		extra = append(extra, "roundtrip fallback")
	}

	info.debug = strings.Join(extra, " ")
	return twkb, info, nil
}

func decompressGeopackage(name, tgpkgPath, gpkgPath, table string) error {
	_, err := script.Exec(fmt.Sprintf(`cp "%s" "%s"`, tgpkgPath, gpkgPath)).Stdout()
	if err != nil {
		return err
	}
	pool, err := sqlitex.Open(gpkgPath, 0, 2)
	if err != nil {
		return err
	}
	defer pool.Close()

	readc := pool.Get(context.Background())
	defer pool.Put(readc)

	reads := readc.Prep(fmt.Sprintf(`
		SELECT fid, geom
		FROM %s`,
		table,
	))
	defer reads.Reset()

	writec := pool.Get(context.Background())
	defer pool.Put(writec)

	writes := writec.Prep(fmt.Sprintf(`
		UPDATE %s
		SET geom = ?
		WHERE fid = ?`,
		table,
	))

	sqlitex.Execute(writec, "BEGIN TRANSACTION;", nil)

	for {
		if exists, err := reads.Step(); err != nil {
			return fmt.Errorf("error listing geometry: %s", err.Error())
		} else if !exists {
			break
		}

		fid := reads.ColumnInt64(0)
		r := reads.ColumnReader(1)
		g, err := gpkg.Read(r)
		if err != nil {
			return fmt.Errorf("unable to read gpkg: %w", err)
		}

		if g.Empty() {
			fmt.Printf("decompress fid %6d empty, skipping\n", fid)
			continue
		}

		if g.Type() != gpkg.ExtendedType {
			fmt.Printf("decompress fid %6d standard geometry, skipping\n", fid)
			continue
		}

		var magic [4]byte
		n, err := r.Read(magic[:])
		if err != nil {
			return fmt.Errorf("unable to read magic: %w", err)
		}
		if n != 4 {
			return fmt.Errorf("unable to read magic, short read: %d", n)
		}
		if magic != [4]byte{'T', 'W', 'K', 'B'} {
			return fmt.Errorf("invalid magic: %s", string(magic[:]))
		}

		twkb, err := io.ReadAll(r)
		if err != nil {
			return fmt.Errorf("unable to read twkb: %w", err)
		}

		gm, err := geom.UnmarshalTWKB(twkb)
		if err != nil {
			return fmt.Errorf("unable to unmarshal twkb: %w", err)
		}

		wkb := gm.AsBinary()

		g.SetType(gpkg.StandardType)
		g.SetEnvelopeContentsIndicatorCode(gpkg.NoEnvelope)

		// Write WKB
		w := new(bytes.Buffer)
		g.Write(w)
		io.Copy(w, bytes.NewReader(wkb))

		buf := w.Bytes()
		writes.BindBytes(1, buf)
		writes.BindInt64(2, fid)
		_, err = writes.Step()
		if err != nil {
			return fmt.Errorf("unable to write wkb: %s", err.Error())
		}
		err = writes.Reset()
		if err != nil {
			return fmt.Errorf("unable to reset write statement: %w", err)
		}

		fmt.Printf(
			"decompress %10s fid %6d %7d twkb bytes to %7d wkb bytes %4.0f%% %7d bytes written\n",
			name, fid, len(twkb), len(wkb), 100.*float32(len(wkb))/float32(len(twkb)), len(buf),
		)
	}

	sqlitex.Execute(writec, "COMMIT;", nil)

	err = sqlitex.ExecuteScript(writec, fmt.Sprintf(`
		DELETE FROM gpkg_extensions WHERE table_name = '%[1]s' AND column_name = 'geom' AND extension_name = 'mlunar_twkb';
	`, table), nil)
	if err != nil {
		return err
	}

	err = sqlitex.Execute(writec, "VACUUM;", nil)
	if err != nil {
		return err
	}

	return nil
}

// func move(from string, to string) {
// 	_, err := script.Exec(fmt.Sprintf(`mv "%s" "%s"`, from, to)).Stdout()
// 	if err != nil {
// 		panic(err)
// 	}
// }

// func moveDir(path string, old string, new string) {
// 	after, found := strings.CutPrefix(path, old)
// 	if !found {
// 		panic("old dir not found")
// 	}
// 	move(path, filepath.Join(new, after))
// }

func compressWithOpts(gpkg string, name string, table string, optss []compressOpts) {
	fmt.Printf("compress %s\n", name)

	var err error
	optch := make(chan compressOpts, 1)
	wg := &sync.WaitGroup{}
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for opts := range optch {
				tgpkg := OUTPUT_DIR + name + "_" + opts.name + ".gpkg"
				rtgpkg := OUTPUT_DIR + name + "_roundtrip_" + opts.name + ".gpkg"

				err = compressGeopackage(gpkg, tgpkg, table, opts)
				if err != nil {
					panic(err)
				}

				err = decompressGeopackage(opts.name, tgpkg, rtgpkg, table)
				if err != nil {
					panic(err)
				}
			}
		}()
	}

	for _, opts := range optss {
		optch <- opts
	}
	close(optch)
	wg.Wait()
}

type point struct {
	x float64
	y float64
}

type zoomPoint struct {
	zoom float64
	point
}

type renderOpts struct {
	name   string
	w      int
	h      int
	points []point
	zooms  []float64
}

type renderJob struct {
	gpkg  string
	table string
	name  string
	w     int
	h     int
	cx    float64
	cy    float64
	zoom  float64
}

func renderJobs(name string, table string, ch <-chan renderJob) {
	fmt.Printf("render %s\n", name)

	wg := &sync.WaitGroup{}
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range ch {
				err := render(job.gpkg, job.table, job.name, job.w, job.h, job.cx, job.cy, job.zoom)
				// rtgpkg := OUTPUT_DIR + name + "_roundtrip_" + opts.name + ".gpkg"
				// err := render(rtgpkg, name+"_roundtrip_"+opts.name, table, 512, 512, 14.75, 46.08, 5)
				if err != nil {
					panic(err)
				}
			}
		}()
	}
	wg.Wait()
}

func renderRoundtrip(name string, table string, optss []compressOpts) {
	jobs := make(chan renderJob)

	w, h := 512, 512
	// cx, cy := 14.75, 46.08
	// points := []point{{14.75, 46.08}}
	points := []zoomPoint{
		{0, point{0, 0}},
		{3, point{9.6, 49}},
		{2, point{19, 6}},
		{3, point{-101, 40}},
		{5, point{130, 35}},
	}
	// zooms := []int{0, 1, 2}

	go func() {
		for _, opts := range optss {
			for pi, zp := range points {
				jobs <- renderJob{
					gpkg:  OUTPUT_DIR + name + "_roundtrip_" + opts.name + ".gpkg",
					table: table,
					name:  fmt.Sprintf("%s_roundtrip_%s_p%d", name, opts.name, pi),
					w:     w,
					h:     h,
					cx:    zp.x,
					cy:    zp.y,
					zoom:  float64(zp.zoom),
				}
			}
		}
		close(jobs)
	}()
	renderJobs(name, table, jobs)
}

func generate(name string, optss []compressOpts) {
	fmt.Printf("generate %s\n", name)

	geojson := OUTPUT_DIR + name + ".geojson"
	gpkg := OUTPUT_DIR + name + ".gpkg"
	var err error
	err = download(geojsonUrl(name), geojson)
	if err != nil {
		panic(err)
	}

	err = convertToGeopackage(geojson, gpkg)
	if err != nil {
		panic(err)
	}

	compressWithOpts(gpkg, name, name, optss)

	fmt.Printf("generate %s done\n", name)
}

func optrange(fromPrec int, toPrec int, fromSimp int, toSimp int) []compressOpts {
	optss := []compressOpts{}
	// fromPrec, toPrec := 3, 3
	// fromSimp, toSimp := 0, 5
	// fromSimp, toSimp := 1, 4
	for minPrec := fromPrec; minPrec <= toPrec; minPrec++ {
		for i := fromSimp; i <= toSimp; i++ {
			simplify := []float64{}
			jrange := toSimp - fromSimp
			if jrange < 3 {
				jrange = 3
			}
			for j := i; j <= fromSimp+jrange; j++ {
				simplify = append(simplify, math.Pow(10, float64(-1-j)))
			}
			optss = append(optss, compressOpts{
				name:              fmt.Sprintf("twkb_p%d_s%d", minPrec, i),
				minPrecXY:         minPrec,
				simplify:          simplify,
				simplifyMinPoints: 20,
			})
		}
	}
	return optss
}

func main() {

	// optss := []compressOpts{}
	// for i := MIN_PRECXY; i <= MAX_PRECXY; i++ {
	// 	optss = append(optss, compressOpts{
	// 		name:              fmt.Sprintf("precxy_%d", i),
	// 		minPrecXY:         i,
	// 		simplify:          []float64{0.01, 0.001},
	// 		simplifyMinPoints: 20,
	// 	})
	// }

	// optss := []compressOpts{}
	// maxPower := 2
	// minPower := -6
	// for i := maxPower; i >= minPower; i-- {
	// 	simplify := []float64{}
	// 	for j := i; j >= minPower; j-- {
	// 		simplify = append(simplify, math.Pow(10, float64(j)))
	// 	}
	// 	optss = append(optss, compressOpts{
	// 		name:              fmt.Sprintf("twkb_%d", len(optss)),
	// 		minPrecXY:         3,
	// 		simplify:          simplify,
	// 		simplifyMinPoints: 20,
	// 	})
	// }

	// generate("ne_110m_admin_0_countries", optss)
	// generate("ne_10m_admin_0_countries", optrange(3, 3, 0, 3))
	// generate("ne_10m_urban_areas_landscan", optrange(3, 3, 0, 2))

	// compressWithOpts("gadm_410", OUTPUT_DIR+"gadm_410.gpkg", optrange(3, 3, 2, 2))

	// err := convertToGeopackage(OUTPUT_DIR+"geoBoundariesCGAZ_ADM2.gpkg", OUTPUT_DIR+"geoBoundariesCGAZ_ADM2_valid.gpkg")
	// if err != nil {
	// 	panic(err)
	// }
	// compressWithOpts(OUTPUT_DIR+"geoBoundariesCGAZ_ADM2_valid.gpkg", "geoBoundariesCGAZ_ADM2", "globalADM2", optrange(2, 2, 3, 3))

	// panic(render(OUTPUT_DIR+"ne_110m_admin_0_countries.gpkg", "ne_110m_admin_0_countries", "ne_110m_admin_0_countries_basic", 512, 512, 46.08, 14.75, 1))
	// panic(render(OUTPUT_DIR+"ne_110m_admin_0_countries.gpkg", "ne_110m_admin_0_countries", "ne_110m_admin_0_countries_basic", 512, 512, 14.75, 46.08, 5))
	// panic(render(OUTPUT_DIR+"ne_110m_admin_0_countries.gpkg", "ne_110m_admin_0_countries", "ne_110m_admin_0_countries_basic", 1024, 512, 46.08, 14.75, 1))
	// panic(render(OUTPUT_DIR+"ne_110m_admin_0_countries.gpkg", "ne_110m_admin_0_countries", "ne_110m_admin_0_countries_basic", 512, 1024, 46.08, 14.75, 1))

	// renderRoundtrip("ne_110m_admin_0_countries", "ne_110m_admin_0_countries", optrange(2, 3, 0, 5))
	// renderRoundtrip("ne_110m_admin_0_countries", "ne_110m_admin_0_countries", optrange(2, 2, 0, 0))
	// renderRoundtrip("ne_10m_admin_0_countries", "ne_10m_admin_0_countries", optrange(3, 3, 0, 0))
	// renderRoundtrip("ne_10m_urban_areas_landscan", "ne_10m_urban_areas_landscan", optrange(3, 3, 0, 0))
	renderRoundtrip("geoBoundariesCGAZ_ADM2", "globalADM2", optrange(2, 2, 3, 3))

	// generate("ne_10m_admin_0_countries", 0, +7)
	// generate("ne_10m_admin_0_countries", 0, +5)
	// generate("ne_10m_admin_0_countries", 0, +2)
	// generate("ne_10m_urban_areas_landscan", compressOpts{
	// 	minPrecXY:         +2,
	// 	simplify:          []float64{0.01, 0.001, 0.0001},
	// 	simplifyMinPoints: 20,
	// })
}
