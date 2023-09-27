<!-- HEADER -->
<br />
<p align="center">
  <a href="https://github.com/SmilyOrg/tinygpkg">
    <img src="assets/logo.png" alt="Logo" width="80" height="80">
  </a>

  <h3 align="center">tinygpkg</h3>

  <p align="center">
    Go library for local, small, fast reverse geocoding with <a href="https://github.com/TWKB/Specification/blob/master/twkb.md">TWKB</a> & <a href="http://www.geopackage.org/">GeoPackage</a>.
    <br />
    <br />
    <a href="https://github.com/SmilyOrg/tinygpkg-data">📥 Get Datasets</a>
    ·
    <a href="https://github.com/SmilyOrg/tinygpkg/issues">🐛 Report Bug</a>
    ·
    <a href="https://github.com/SmilyOrg/tinygpkg/issues">💡 Request Feature</a>
  </p>
</p>



<!-- TABLE OF CONTENTS -->
<details open="open">
  <summary>Table of Contents</summary>
  <ol>
    <li>
      <a href="#about">About</a>
      <ul>
        <li><a href="#features">Features</a></li>
        <li><a href="#limitations">Limitations</a></li>
        <li><a href="#built-with">Built With</a></li>
      </ul>
    </li>
    <li><a href="#benchmarks">Benchmarks</a></li>
    <li><a href="#usage">Usage</a></li>
    <li><a href="#contributing">Contributing</a></li>
    <li><a href="#license">License</a></li>
    <li><a href="#acknowledgements">Acknowledgements</a></li>
  </ol>
</details>



## About

tinygpkg is a Go library for fast, local, and small-scale geospatial processing.
Currently the main use-case is local reverse geocoding by using [GeoPackage]
files that have been simplified and compressed into [Tiny Well-known Binary
(TWKB)] format.

The library has been heavily inspired by [sams96/rgeo], a Go library for local
reverse geocoding. The main difference is that `rgeo` uses embedded compressed
GeoJSON, which it uses to build a [s2.ShapeIndex] at initialization time, while
`tinygpkg` uses the [GeoPackage] format (based on SQLite), which it queries and
deserializes at query time.

This means that for comparable datasets `tinygpkg` has almost **no startup
cost** (12ms vs 14s) and **drastically lower runtime memory usage** (27MB vs
1.5GB) at the expense of **slower reverse geocoding queries** (63µs vs 500ns)
compared to `rgeo`. `tinygpkg` can also work with much larger datasets (like
[geoBoundaries CGAZ]), as it doesn't need to index the entire dataset in memory.

### Features

* **Local** - no network requests needed
* **Small** - supports [Tiny Well-known Binary (TWKB)] in [GeoPackage] for smaller dataset sizes
* **Fast** - fast startup time (12ms) and reverse geocoding queries (<1ms)
* **Low memory usage** - [GeoPackage] files are queried on-the-fly at runtime
* **Large datasets** - can work with datasets that don't fit in memory
* **GeoPackage** - uses the [GeoPackage] format reading geospatial data
* **TWKB** - supports [Tiny Well-known Binary (TWKB)] in GeoPackage for compressed datasets

### Limitations

* **Slower queries** - each query needs to do a database lookup, geometry deserialization, and point-in-polygon check - it's still plenty fast (microseconds), but not as fast as [sams96/rgeo] that uses [s2.ShapeIndex]
* **No GeoJSON** - only supports GeoPackage files for now

### Built With

* [Go](https://golang.org/)
* [GeoPackage](http://www.geopackage.org/) - SQLite-based format for geospatial data
* [Tiny Well-known Binary (TWKB)] - compressed geometry format
* [peterstace/simplefeatures](https://github.com/peterstace/simplefeatures) - Go geometry processing library
* [zombiezen.com/go/sqlite](https://github.com/zombiezen/go-sqlite) - pure Go SQLite library

## Benchmarks

See a more detailed comparison below using two [Natural Earth] datasets.

[s2.ShapeIndex]: https://pkg.go.dev/github.com/golang/geo/s2#ShapeIndex
[Natural Earth]: https://www.naturalearthdata.com/
[geoBoundaries CGAZ]: https://www.geoboundaries.org/downloadCGAZ.html

| Benchmark - [110m countries dataset] | rgeo       | tinygpkg   | % of rgeo |
| ------------------------------------ | ---------- | ---------- | --------- |
| Compiled code size                   | **3.2 MB** | 7.8 MB     | 243%      |
| Bundle size (code + data)            | 32 MB      | **8.2 MB** | 26%       |
| Startup time                         | 93 ms      | **14 ms**  | 15%       |
| Startup allocated bytes              | 22 MB      | **12 KB**  | 0.05%     |
| Runtime memory usage                 | **25 MB**  | 27 MB      | 108%      |
| Reverse geocode time                 | **1.2 µs** | 87 µs      | 7250%     |

| Benchmark - [10m cities dataset] | rgeo       | tinygpkg    | % of rgeo |
| -------------------------------- | ---------- | ----------- | --------- |
| Compiled code size               | **3.2 MB** | 7.8 MB      | 243%      |
| Bundle size (code + data)        | 32 MB      | **11.5 MB** | 35%       |
| Bundle size (7z compressed)      | 30 MB      | **5 MB**    | 17%       |
| Startup time                     | 14000 ms   | **12 ms**   | 0.08%     |
| Startup allocated bytes          | 6 GB       | **13 KB**   | 0.0002%   |
| Runtime memory usage             | 1.5 GB     | **27 MB**   | 1.8%      |
| Reverse geocode time             | **0.5 µs** | 63 µs       | 12600%    |

See also detailed [benchmark results](/bench/results/).

[110m countries dataset]: https://www.naturalearthdata.com/downloads/110m-cultural-vectors/110m-admin-0-countries/
[10m cities dataset]: https://www.naturalearthdata.com/downloads/10m-cultural-vectors/10m-urban-area/


## Usage

```sh
go get github.com/smilyorg/tinygpkg
```

See [example](/example), shortened below.
```go

// Open GeoPackage with dataset and column for reverse geocoding
g, _ := gpkg.Open(
  "../testdata/ne_110m_admin_0_countries_s4_twkb_p3.gpkg",
  "ne_110m_admin_0_countries",
  "NAME",
)
defer g.Close()

// Reverse geocode a point
p := s2.LatLngFromDegrees(48.8566, 2.3522)
name, _ := g.ReverseGeocode(context.Background(), p)
println(name)

// Output: France
```


## Contributing

Pull requests are welcome. For major changes, please open an issue first to
discuss what you would like to change.

## License

Distributed under the MIT License. See [LICENSE](LICENSE) for more information.

## Acknowledgements
* [sams96/rgeo] - big inspiration for this library
* [Best-README-Template](https://github.com/othneildrew/Best-README-Template)
* [readme.so](https://readme.so/)

[Tiny Well-known Binary (TWKB)]: https://github.com/TWKB/Specification/blob/master/twkb.md
[GeoPackage]: http://www.geopackage.org/
[sams96/rgeo]: https://github.com/sams96/rgeo