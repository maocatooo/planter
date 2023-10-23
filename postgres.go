package main

import (
	"database/sql"
	_ "github.com/lib/pq" // postgres
	"github.com/pkg/errors"
	"log"
)

const _PGSQLcolumDefSQL = `
SELECT
    a.attnum AS field_ordinal,
    a.attname AS column_name,
    pd.description AS description,
    format_type(a.atttypid, a.atttypmod) AS data_type,
    a.attnotnull AS not_null,
    COALESCE(ct.contype = 'p', false) AS  is_primary_key,
    CASE WHEN a.atttypid = ANY ('{int,int8,int2}'::regtype[])
      AND EXISTS (
         SELECT 1 FROM pg_attrdef ad
         WHERE  ad.adrelid = a.attrelid
         AND    ad.adnum   = a.attnum
         AND    pg_get_expr(ad.adbin, ad.adrelid) = 'nextval('''
            || (pg_get_serial_sequence (a.attrelid::regclass::text
                                      , a.attname))::regclass
            || '''::regclass)'
         )
    THEN CASE a.atttypid
            WHEN 'int'::regtype  THEN 'serial'
            WHEN 'int8'::regtype THEN 'bigserial'
            WHEN 'int2'::regtype THEN 'smallserial'
         END
    ELSE format_type(a.atttypid, a.atttypmod)
    END AS data_type
FROM pg_attribute a
JOIN ONLY pg_class c ON c.oid = a.attrelid
JOIN ONLY pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_constraint ct ON ct.conrelid = c.oid
AND a.attnum = ANY(ct.conkey) AND ct.contype IN ('p', 'u')
LEFT JOIN pg_attrdef ad ON ad.adrelid = c.oid AND ad.adnum = a.attnum
LEFT JOIN pg_description pd ON pd.objoid = a.attrelid AND pd.objsubid = a.attnum
WHERE a.attisdropped = false
AND n.nspname = $1
AND c.relname = $2
AND a.attnum > 0
ORDER BY a.attnum
`

const _PGSQLTableDefSQL = `
SELECT
  c.relname AS table_name,
  pd.description AS description
FROM pg_class c
JOIN ONLY pg_namespace n
ON n.oid = c.relnamespace
LEFT JOIN pg_description pd ON pd.objoid = c.oid AND pd.objsubid = 0
WHERE n.nspname = $1
AND c.relkind in ('r','p') AND NOT COALESCE((row_to_json(c)->>'relispartition')::boolean,false)
ORDER BY c.relname
`

const _PGSQLFKDefSQL = `
select
  att2.attname as "child_column"
  , cl.relname as "parent_table"
  , att.attname as "parent_column"
  , con.conname
  , case 
      when pi.indisprimary is null then false
      else pi.indisprimary
    end as "is_parent_pk"
  , case 
      when ci.indisprimary is null then false
      else ci.indisprimary
    end as "is_child_pk"
from (
  select 
    unnest(con1.conkey) as "parent"
    , unnest(con1.confkey) as "child"
    , con1.confrelid
    , con1.conrelid
    , con1.conname
  from pg_class cl
  join pg_namespace ns on cl.relnamespace = ns.oid
  join pg_constraint con1 on con1.conrelid = cl.oid
  where ns.nspname = $1
  and cl.relname = $2
  and con1.contype = 'f'
  and (coalesce((row_to_json(con1)->>'conparentid'),'0')::oid) = 0
) con
join pg_attribute att
on att.attrelid = con.confrelid and att.attnum = con.child
left outer join pg_index pi
on att.attrelid = pi.indrelid and att.attnum = any(pi.indkey)
join pg_class cl
on cl.oid = con.confrelid
join pg_attribute att2
on att2.attrelid = con.conrelid and att2.attnum = con.parent
left outer join pg_index ci
on att2.attrelid = ci.indrelid and att2.attnum = any(ci.indkey)
order by con.conname
`

type postgres struct {
	db               Queryer
	schema           string
	_currentDataBase string
}

func NewPostgres(schema string) Planter {
	return &postgres{
		schema: schema,
	}
}

func (m *postgres) OpenDB(connStr string) {
	conn, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to connect to database"))
	}
	m.db = conn
}

// loadColumnDef load Postgres column definition
func (m *postgres) loadColumnDef(table string) []*Column {
	colDefs, err := m.db.Query(_PGSQLcolumDefSQL, m.schema, table)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to load table def"))
	}
	var cols []*Column
	for colDefs.Next() {
		var c Column
		err := colDefs.Scan(
			&c.FieldOrdinal,
			&c.Name,
			&c.Comment,
			&c.DataType,
			&c.NotNull,
			&c.IsPrimaryKey,
			&c.DDLType,
		)
		c.Comment.String = stripCommentSuffix(c.Comment.String)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to scan"))
		}
		cols = append(cols, &c)
	}
	return cols
}

// loadForeignKeyDef load Postgres fk definition
func (m *postgres) loadForeignKeyDef(tbls []*Table, tbl *Table) []*ForeignKey {
	fkDefs, err := m.db.Query(_PGSQLFKDefSQL, m.schema, tbl.Name)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to load fk def"))
	}
	var fks []*ForeignKey
	for fkDefs.Next() {
		fk := ForeignKey{
			SourceTableName: tbl.Name,
			SourceTable:     tbl,
		}
		err := fkDefs.Scan(
			&fk.SourceColName,
			&fk.TargetTableName,
			&fk.TargetColName,
			&fk.ConstraintName,
			&fk.IsTargetColPrimaryKey,
			&fk.IsSourceColPrimaryKey,
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
		sourceCol, found := FindColumnByName(tbls, fk.SourceTableName, fk.SourceColName)
		if !found {
			log.Fatal(errors.Errorf("%s.%s not found", fk.SourceTableName, fk.SourceColName))
		}
		sourceCol.IsForeignKey = true
		fk.SourceColumn = sourceCol
	}
	return fks
}

// LoadTableDef load Postgres table definition
func (m *postgres) LoadTableDef() []*Table {
	tbDefs, err := m.db.Query(_PGSQLTableDefSQL, m.schema)
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
