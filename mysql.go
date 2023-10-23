package main

import (
	"database/sql"
	"github.com/pkg/errors"
	"log"
)

const _MySQLCurrentDataBaseSQL = `
SELECT DATABASE();
`

const _MySQLTableDefSQL = `
SELECT
    TABLE_NAME,TABLE_COMMENT
FROM
    information_schema.TABLES a
WHERE
    a.table_schema = ?

`

const _MySQLColumDefSQL = `
SELECT
    b.ORDINAL_POSITION, b.COLUMN_NAME, b.COLUMN_COMMENT, b.COLUMN_TYPE, b.COLUMN_KEY,b.IS_NULLABLE
FROM
    information_schema.TABLES a
        LEFT JOIN information_schema.COLUMNS b ON a.table_name = b.TABLE_NAME and a.TABLE_SCHEMA = b.TABLE_SCHEMA
WHERE
        a.TABLE_SCHEMA = ? AND a.table_name = ? ORDER BY b.ORDINAL_POSITION
`

const _MySQLFKDefSQL = `
SELECT REFERENCED_COLUMN_NAME, TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME
FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
WHERE CONSTRAINT_SCHEMA = ?  AND REFERENCED_TABLE_NAME = ? 
`

type mysql struct {
	db               Queryer
	_currentDataBase string
}

func NewMysql() Planter {
	return &mysql{}
}

func (m *mysql) OpenDB(connStr string) {
	conn, err := sql.Open("mysql", connStr)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to connect to database"))
	}
	m.db = conn
}

func (m *mysql) loadCurrentDataBase() string {
	if m._currentDataBase != `` {
		return m._currentDataBase
	}
	dbName, err := m.db.Query(_MySQLCurrentDataBaseSQL)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to load dbName def"))
	}
	for dbName.Next() {
		err := dbName.Scan(
			&m._currentDataBase,
		)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to scan"))
		}
		return m._currentDataBase
	}
	log.Fatal(errors.New("failed to load current database"))
	return ""
}

// LoadColumnDef load Postgres column definition
func (m *mysql) loadColumnDef(table string) []*Column {
	dbName := m.loadCurrentDataBase()

	colDefs, err := m.db.Query(_MySQLColumDefSQL, dbName, table)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to load column def"))
	}
	var cols []*Column
	for colDefs.Next() {
		var c mySQLColumn
		err := colDefs.Scan(
			&c.FieldOrdinal,
			&c.Name,
			&c.Comment,
			&c.DataType,
			&c.KeyType,
			&c.Nullable,
		)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to scan"))
		}
		c.format()
		c.Comment.String = stripCommentSuffix(c.Comment.String)
		cols = append(cols, c.toColumn())
	}
	return cols
}

// LoadForeignKeyDef load Postgres fk definition
func (m *mysql) loadForeignKeyDef(tbls []*Table, tbl *Table) []*ForeignKey {
	dbName := m.loadCurrentDataBase()
	fkDefs, err := m.db.Query(_MySQLFKDefSQL, dbName, tbl.Name)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to load fk def"))
	}
	var fks []*ForeignKey
	for fkDefs.Next() {
		fk := ForeignKey{
			TargetTableName: tbl.Name,
			TargetTable:     tbl,
		}
		err := fkDefs.Scan(
			&fk.TargetColName,
			&fk.SourceTableName,
			&fk.SourceColName,
			&fk.ConstraintName,
		)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to scan"))
		}
		fks = append(fks, &fk)
	}
	for _, fk := range fks {
		targetTbl, found := FindTableByName(tbls, fk.TargetTableName)
		if !found {
			log.Fatal(errors.Errorf("%s not found", fk.TargetTableName))
		}
		fk.TargetTable = targetTbl
		targetCol, found := FindColumnByName(tbls, fk.TargetTableName, fk.TargetColName)
		if !found {
			log.Fatal(errors.Errorf("%s.%s not found", fk.TargetTableName, fk.TargetColName))
		}
		fk.TargetColumn = targetCol
		targetCol.IsForeignKey = true
		fk.TargetColumn.IsPrimaryKey = targetCol.IsPrimaryKey

		sourceTbl, found := FindTableByName(tbls, fk.SourceTableName)
		if !found {
			log.Fatal(errors.Errorf("%s not found", fk.TargetTableName))
		}
		fk.SourceTable = sourceTbl
		sourceCol, found := FindColumnByName(tbls, fk.SourceTableName, fk.SourceColName)
		if !found {
			log.Fatal(errors.Errorf("%s.%s not found", fk.SourceTableName, fk.SourceColName))
		}
		fk.SourceColumn = sourceCol
		fk.SourceColumn.IsPrimaryKey = sourceCol.IsPrimaryKey
	}
	return fks
}

// LoadTableDef load Mysql table definition
func (m *mysql) LoadTableDef() []*Table {
	dbName := m.loadCurrentDataBase()
	tbDefs, err := m.db.Query(_MySQLTableDefSQL, dbName)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to load table def"))
	}
	var tbls []*Table
	for tbDefs.Next() {
		t := &Table{}
		err := tbDefs.Scan(
			&t.Name,
			&t.Comment,
		)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to scan"))
		}
		cols := m.loadColumnDef(t.Name)
		t.Columns = cols
		tbls = append(tbls, t)
	}
	for _, tbl := range tbls {
		fks := m.loadForeignKeyDef(tbls, tbl)
		tbl.ForeingKeys = fks
	}
	return tbls
}

type mySQLColumn struct {
	FieldOrdinal int
	Name         string
	Comment      sql.NullString
	DataType     string

	KeyType      string
	Nullable     string
	NotNull      bool
	IsPrimaryKey bool
	IsForeignKey bool
}

func (m *mySQLColumn) toColumn() *Column {
	return &Column{
		FieldOrdinal: m.FieldOrdinal,
		Name:         m.Name,
		Comment:      m.Comment,
		DataType:     m.DataType,
		DDLType:      m.DataType,
		IsPrimaryKey: m.IsPrimaryKey,
		IsForeignKey: m.IsForeignKey,
	}
}

func (m *mySQLColumn) format() {
	if m.Nullable == "YES" {
		m.NotNull = false
	}
	if m.KeyType == "PRI" {
		m.IsPrimaryKey = true
	}
}
