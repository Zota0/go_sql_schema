package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/fatih/color"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Column struct {
	Type    string
	Name    string
	Length  string
	Null    string
	Key     string
	AutoInc bool
	GoType  string
	JsonTag string
}

type Table struct {
	Name    string
	Columns []Column
}

type Database struct {
	Name   string
	Tables []Table
}

func main() {
	var databases []Database
	databaseMap := make(map[string]bool)
	tableMap := make(map[string]map[string]bool)

	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)
	magenta := color.New(color.FgMagenta)

	for {
		var databaseName string
		fmt.Println()
		cyan.Print("Enter database ('.exit' to finish): ")
		reader := bufio.NewReader(os.Stdin)
		databaseName, _ = reader.ReadString('\n')
		databaseName = strings.TrimSpace(databaseName)

		if databaseName == ".exit" {
			break
		}

		if _, ok := databaseMap[databaseName]; ok {
			red.Println("Database with this name already exists. Choose a different name.")
			continue
		}
		databaseMap[databaseName] = true

		db := Database{Name: databaseName, Tables: []Table{}}

		tableMap[databaseName] = make(map[string]bool)

		fmt.Println()

		for {
			var tableName string
			green.Printf("Enter table for '%s' ('.exit' to finish database): ", databaseName)
			reader := bufio.NewReader(os.Stdin)
			tableName, _ = reader.ReadString('\n')
			tableName = strings.TrimSpace(tableName)

			if tableName == ".exit" {
				break
			}

			if _, ok := tableMap[databaseName][tableName]; ok {
				red.Println("Table with this name already exists in this database. Choose a different name.")
				continue
			}
			tableMap[databaseName][tableName] = true

			tableColumns := []Column{}
			columnMap := make(map[string]bool)

			i := 0
			for {
				var colName, colType, colLength, colNull, colKey, autoInc string
				var line string
				var promptColor *color.Color

				if i%2 == 0 {
					promptColor = cyan
				} else {
					promptColor = magenta
				}
				i++

				promptColor.Println("(Column: Name, Type, len|null, null|not, (PRIMARY|UNIQUE|FOREIGN|empty), autoIncrement(true|false|blank)) | ('.exit'):")
				reader := bufio.NewReader(os.Stdin)
				line, _ = reader.ReadString('\n')
				line = strings.TrimSpace(line)
				if line == ".exit" {
					break
				}

				parts := strings.FieldsFunc(line, func(r rune) bool {
					return unicode.IsSpace(r) || r == ','
				})

				if len(parts) < 4 {
					red.Println("Invalid input: Insufficient parameters. At least column name, type, length, and null constraint are required.")
					continue
				}

				colName = strings.TrimSpace(parts[0])
				colType = strings.TrimSpace(parts[1])

				if _, ok := columnMap[colName]; ok {
					red.Println("Column with this name already exists in this table. Choose a different name.")
					continue
				}
				columnMap[colName] = true

				colLength = strings.TrimSpace(parts[2])
				if colLength == "null" {
					colLength = ""
				}
				colNull = strings.TrimSpace(parts[3])

				if len(parts) >= 5 {
					colKey = strings.TrimSpace(parts[4])
				}
				if len(parts) >= 6 {
					autoInc = strings.TrimSpace(parts[5])
				} else {
					autoInc = "false"
				}

				autoIncrement, err := strconv.ParseBool(autoInc)
				if err != nil && autoInc != "" {
					red.Println("Invalid autoIncrement value. Use 'true', 'false', or leave blank.")
					continue
				}

				var goType string
				switch colType {
				case "INT", "INTEGER", "SMALLINT", "BIGINT":
					goType = "int"
				case "VARCHAR", "TEXT", "CHAR", "LONGTEXT":
					goType = "string"
				case "DATE", "DATETIME", "TIMESTAMP", "TIME":
					goType = "time.Time"
				case "BOOLEAN", "BOOL":
					goType = "bool"
				case "FLOAT", "DOUBLE", "REAL":
					goType = "float64"
				default:
					goType = "interface{}"
				}

				tableColumns = append(tableColumns, Column{
					Type:    colType,
					Name:    colName,
					Length:  colLength,
					Null:    colNull,
					Key:     colKey,
					AutoInc: autoIncrement,
					GoType:  goType,
					JsonTag: colName,
				})
			}

			db.Tables = append(db.Tables, Table{Name: tableName, Columns: tableColumns})
		}
		databases = append(databases, db)
	}

	generateFiles(databases, green, red, yellow, bold)
}

func generateFiles(databases []Database, green, red, yellow, bold *color.Color) {
	green.Println("\nGenerating files...")
	err := os.MkdirAll("./out", os.ModePerm)
	if err != nil {
		red.Printf("Error creating out directory: %v\n", err)
		return
	}

	goStructFile, err := os.Create("./out/structs.go")
	if err != nil {
		red.Printf("Error creating structs.go: %v\n", err)
		return
	}
	defer goStructFile.Close()

	goStructFile.WriteString("package structs\n\nimport (\n\t\"time\"\n)\n\n")

	caser := cases.Title(language.Und)
	for _, db := range databases {
		for _, table := range db.Tables {
			goStructFile.WriteString(fmt.Sprintf("type %s struct {\n", caser.String(table.Name)))
			for _, col := range table.Columns {
				goStructFile.WriteString(fmt.Sprintf("\t%s\t%s `json:\"%s\"`\n", caser.String(col.Name), col.GoType, col.JsonTag))
			}
			goStructFile.WriteString("}\n\n")
		}
	}

	sqlFile, err := os.Create("./out/tables.sql")
	if err != nil {
		red.Printf("Error creating tables.sql: %v\n", err)
		return
	}
	defer sqlFile.Close()

	for _, db := range databases {
		yellow.Printf("-----%s-----\n", db.Name)
		sqlFile.WriteString(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;\n", db.Name))
		sqlFile.WriteString(fmt.Sprintf("USE %s;\n\n", db.Name))
		for _, table := range db.Tables {
			green.Printf("%s,\n", table.Name)
			sqlFile.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", table.Name))
			for j, col := range table.Columns {
				nullConstraint := " NOT NULL"
				if col.Null == "null" {
					nullConstraint = " NULL"
				}
				keyConstraint := ""
				switch col.Key {
				case "PRIMARY":
					keyConstraint = " PRIMARY KEY"
					if col.AutoInc {
						keyConstraint += " AUTO_INCREMENT"
					}
				case "UNIQUE":
					keyConstraint = " UNIQUE"
				case "FOREIGN":
					keyConstraint = " FOREIGN KEY"
				}
				length := ""
				if col.Length != "" {
					length = fmt.Sprintf("(%s)", col.Length)
				}

				sqlFile.WriteString(fmt.Sprintf("\t%s %s%s%s%s",
					col.Name,
					strings.ToUpper(col.Type),
					length,
					nullConstraint,
					keyConstraint,
				))
				if j < len(table.Columns)-1 {
					sqlFile.WriteString(",")
				}
				sqlFile.WriteString("\n")
			}
			sqlFile.WriteString(");\n\n")
		}
	}

	fmt.Println()
	green.Println(bold.Sprintf("Go structs generated in ./out/structs.go"))
	green.Println(bold.Sprintf("SQL table definitions generated in ./out/tables.sql"))
}
