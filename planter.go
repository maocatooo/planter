package main

import (
	"bytes"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq" // postgres
	"github.com/pkg/errors"
	"html/template"
	"regexp"
	"sort"
	"strings"
)

type Planter interface {
	OpenDB(connStr string)
	LoadTableDef() []*Table
}

// Queryer database/sql compatible query interface
type Queryer interface {
	Exec(string, ...interface{}) (sql.Result, error)
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) *sql.Row
}

// Column postgres columns
type Column struct {
	FieldOrdinal int
	Name         string
	Comment      sql.NullString
	DataType     string
	DDLType      string
	NotNull      bool
	IsPrimaryKey bool
	IsForeignKey bool
}

// ForeignKey foreign key
type ForeignKey struct {
	ConstraintName        string
	SourceTableName       string
	SourceColName         string
	IsSourceColPrimaryKey bool
	SourceTable           *Table
	SourceColumn          *Column
	TargetTableName       string
	TargetColName         string
	IsTargetColPrimaryKey bool
	TargetTable           *Table
	TargetColumn          *Column
}

// IsOneToOne returns true if one to one relation
// - in case of composite pk
//   - one to one
//   - source table is composite pk && target table is composite pk
//   - source table fks to target table are all pks
//   - other cases are one to many
func (k *ForeignKey) IsOneToOne() bool {
	switch {
	case k.SourceTable.IsCompositePK() && k.TargetTable.IsCompositePK():
		var targetFks []*ForeignKey
		for _, fk := range k.SourceTable.ForeingKeys {
			if fk.TargetTableName == k.TargetTableName {
				targetFks = append(targetFks, fk)
			}
		}
		for _, tfk := range targetFks {
			if !tfk.IsSourceColPrimaryKey || !tfk.IsTargetColPrimaryKey {
				return false
			}
		}
		return true
	case !k.SourceTable.IsCompositePK() && k.SourceColumn.IsPrimaryKey && k.TargetColumn.IsPrimaryKey:
		return true
	default:
		return false
	}
}

// Table postgres table
type Table struct {
	Name        string
	Comment     sql.NullString
	AutoGenPk   bool
	Columns     []*Column
	ForeingKeys []*ForeignKey
}

// IsCompositePK check if table is composite pk
func (t *Table) IsCompositePK() bool {
	cnt := 0
	for _, c := range t.Columns {
		if c.IsPrimaryKey {
			cnt++
		}
		if cnt >= 2 {
			return true
		}
	}
	return false
}

func stripCommentSuffix(s string) string {
	if tok := strings.SplitN(s, "\t", 2); len(tok) == 2 {
		return tok[0]
	}
	return s
}

// FindTableByName find table by name
func FindTableByName(tbls []*Table, name string) (*Table, bool) {
	for _, tbl := range tbls {
		if tbl.Name == name {
			return tbl, true
		}
	}
	return nil, false
}

// FindColumnByName find table by name
func FindColumnByName(tbls []*Table, tableName, colName string) (*Column, bool) {
	for _, tbl := range tbls {
		if tbl.Name == tableName {
			for _, col := range tbl.Columns {
				if col.Name == colName {
					return col, true
				}
			}
		}
	}
	return nil, false
}

func contains(v string, r []*regexp.Regexp) bool {
	for _, e := range r {
		if e != nil && e.MatchString(v) {
			return true
		}
	}
	return false
}

// FilterTables filter tables
func FilterTables(match bool, tbls []*Table, tblNames []string) []*Table {
	sort.Strings(tblNames)

	var tblExps []*regexp.Regexp
	for _, tn := range tblNames {
		str := fmt.Sprintf(`([\\/])?%s([\\/])?`, tn)
		r := regexp.MustCompile(str)
		tblExps = append(tblExps, r)
	}

	var target []*Table
	for _, tbl := range tbls {
		if contains(tbl.Name, tblExps) == match {
			var fks []*ForeignKey
			for _, fk := range tbl.ForeingKeys {
				if contains(fk.TargetTableName, tblExps) == match {
					fks = append(fks, fk)
				}
			}
			tbl.ForeingKeys = fks
			target = append(target, tbl)
		}
	}
	return target
}

// TableToUMLEntry table entry
func TableToUMLEntry(tbls []*Table) ([]byte, error) {
	tpl, err := template.New("entry").Parse(entryTmpl)
	if err != nil {
		return nil, err
	}
	var src []byte
	for _, tbl := range tbls {
		buf := new(bytes.Buffer)
		if err := tpl.Execute(buf, tbl); err != nil {
			return nil, errors.Wrapf(err, "failed to execute template: %s", tbl.Name)
		}
		src = append(src, buf.Bytes()...)
	}
	return src, nil
}

// ForeignKeyToUMLRelation relation
func ForeignKeyToUMLRelation(tbls []*Table) ([]byte, error) {
	tpl, err := template.New("relation").Parse(relationTmpl)
	if err != nil {
		return nil, err
	}
	var src []byte
	for _, tbl := range tbls {
		for _, fk := range tbl.ForeingKeys {
			buf := new(bytes.Buffer)
			if err := tpl.Execute(buf, fk); err != nil {
				return nil, errors.Wrapf(err, "failed to execute template: %s", fk.ConstraintName)
			}
			src = append(src, buf.Bytes()...)
		}
	}
	return src, nil
}

func writePrefix(entry, rel []byte, title string) []byte {
	var src []byte
	src = append([]byte("@startuml\n"))
	if len(title) != 0 {
		src = append(src, []byte("title "+title+"\n")...)
	}
	src = append(src, []byte("hide circle\n"+
		"skinparam linetype ortho\n")...)
	src = append(src, entry...)
	src = append(src, rel...)
	src = append(src, []byte("@enduml\n")...)
	return src
}

func (t *Table) fkEq(colName string) (*Column, bool) {
	for _, col := range t.Columns {
		if col.IsPrimaryKey {
			if strings.Contains(col.Name, t.Name) {
				if colName == col.Name {
					return col, true
				}
			} else {
				if colName == fmt.Sprintf("%s_%s", t.Name, col.Name) {
					return col, true
				}
			}

		}
	}
	return nil, false
}

func (t *Table) analyzeFKRel(otherTab *Table) []*ForeignKey {
	var fks []*ForeignKey
	for _, col := range otherTab.Columns {
		if col.IsPrimaryKey {
			continue
		}
		if source, ok := t.fkEq(col.Name); ok {
			fks = append(fks, &ForeignKey{
				ConstraintName:        col.Name,
				SourceTableName:       t.Name,
				SourceColName:         col.Name,
				IsSourceColPrimaryKey: false,
				SourceTable:           t,
				SourceColumn:          source,
				TargetTableName:       otherTab.Name,
				TargetColName:         col.Name,
				IsTargetColPrimaryKey: false,
				TargetTable:           otherTab,
				TargetColumn:          col,
			})
		}
	}
	return fks
}

// ForeignKeyAnalysis use foreign key analysis if all table not set fk
func ForeignKeyAnalysis(tables []*Table) {
	for _, item := range tables {
		if len(item.ForeingKeys) > 0 {
			return
		}
	}

	for index, cul := range tables {
		if index == len(tables)-1 {
			break
		}
		for _, rel := range tables[index+1:] {
			tabRelAnalysis(cul, rel)
		}
	}
}

func tabRelAnalysis(cur, rel *Table) {
	// cur has pk
	fk1 := cur.analyzeFKRel(rel)
	cur.ForeingKeys = append(cur.ForeingKeys, fk1...)
	fk2 := rel.analyzeFKRel(cur)
	// rel has pk
	rel.ForeingKeys = append(rel.ForeingKeys, fk2...)

}
