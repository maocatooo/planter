package main

import (
	"io"
	"log"
	"os"

	"github.com/alecthomas/kingpin"
)

var (
	connStr = kingpin.Arg(
		"conn", "MySQL/PostgreSQL connection string in URL format").Required().String()
	driver         = kingpin.Flag("driver", "driver mysql/postgres, Default mysql").Default("mysql").Short('d').String()
	postgresSchema = kingpin.Flag(
		"schema", "PostgreSQL schema name").Default("public").Short('s').String()
	outFile     = kingpin.Flag("output", "output file path").Short('o').String()
	targetTbls  = kingpin.Flag("table", "target tables").Short('t').Strings()
	xTargetTbls = kingpin.Flag("exclude", "target tables").Short('x').Strings()
	title       = kingpin.Flag("title", "Diagram title").Short('T').String()
	svg         = kingpin.Flag("svg", "gen svg").Bool()
)

func main() {
	kingpin.Parse()

	var planter Planter

	switch *driver {
	case "mysql":
		planter = NewMysql()
	case "postgres":
		planter = NewPostgres(*postgresSchema)
	default:
		log.Fatal("unknown driver")
	}

	planter.OpenDB(*connStr)

	ts := planter.LoadTableDef()

	// use foreign key analysis if all table not set fk
	ForeignKeyAnalysis(ts)

	var tbls []*Table
	if len(*targetTbls) != 0 {
		tbls = FilterTables(true, ts, *targetTbls)
	} else {
		tbls = ts
	}
	if len(*xTargetTbls) != 0 {
		tbls = FilterTables(false, tbls, *xTargetTbls)
	}
	entry, err := TableToUMLEntry(tbls)
	if err != nil {
		log.Fatal(err)
	}
	rel, err := ForeignKeyToUMLRelation(tbls)
	if err != nil {
		log.Fatal(err)
	}
	src := writePrefix(entry, rel, *title)

	// save as svg
	if svg != nil && *svg {
		src = genSVG(string(src))
	}

	var out io.Writer
	if *outFile != "" {
		out, err = os.Create(*outFile)
		if err != nil {
			log.Fatalf("failed to create output file %s: %s", *outFile, err)
		}
	} else {
		out = os.Stdout
	}
	if _, err := out.Write(src); err != nil {
		log.Fatal(err)
	}
}
