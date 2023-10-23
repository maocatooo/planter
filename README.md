planter

🥺 Reference from https://github.com/achiku/planter


## 🤪 Help
```shell
planter --help

usage: planter [<flags>] <conn>

Flags:
      --help                 Show context-sensitive help (also try --help-long
                             and --help-man).
  -d, --driver="mysql"       driver mysql/postgres, Default mysql
  -s, --schema="public"      PostgreSQL schema name
  -o, --output=OUTPUT        output file path
  -t, --table=TABLE ...      target tables
  -x, --exclude=EXCLUDE ...  target tables
  -T, --title=TITLE          Diagram title
      --svg                  gen svg
```

## feature

✌️ add MySQL generation for corresponding PlantUML (MySQL is the default driver)
```shell
planter --driver mysql  root:123456@tcp(127.0.0.1:3306)/test -o test.uml
# use PostgreSQL
planter --driver postgres  ...
```

✌️ add foreign key analysis (if your database doesn't have foreign keys).

✌️ add SVG generation

## 🤪 Installation
```shell
go install -u github.com/maocatooo/planter
```



## 🤪 Generate PlantUML 
```shell
planter root:123456@tcp(127.0.0.1:3306)/test -o test.uml
```

## 🤪 Generate SVG 
```shell
planter root:123456@tcp(127.0.0.1:3306)/test -o test.svg --svg
```