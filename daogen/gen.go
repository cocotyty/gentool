package daogen

import (
	"reflect"
	"strings"
	"os"
	"github.com/jmoiron/sqlx"
	"database/sql"
	"os/exec"
	"regexp"
	"log"
	"text/template"
)

type Gen struct {
	db *sqlx.DB
}

func NewGen() *Gen {
	return &Gen{}
}

type Table struct {
	Name string `db:"name"`
}
type Column struct {
	Field   string         `db:"Field"`
	Type    string         `db:"Type"`
	Null    sql.NullString `db:"Null"`
	Key     []byte         `db:"Key"`
	Default []byte         `db:"Default"`
	Extra   []byte         `db:"Extra"`
}
type GoColumn struct {
	Name       string
	Type       string
	Annotation string
	CanNull    bool
}

func (g *Gen) G(pkg, address string) {
	paths := strings.Split(os.Getenv("GOPATH"), string(os.PathListSeparator))
	GOPATH := paths[0]
	os.MkdirAll(GOPATH + string(os.PathSeparator) + "src" + string(os.PathSeparator) + strings.Replace(pkg, "/", string(os.PathSeparator), -1), 0777)
	log.Println(GOPATH + string(os.PathSeparator) + "src" + string(os.PathSeparator) + strings.Replace(pkg, "/", string(os.PathSeparator), -1))
	basePkg := pkg[strings.LastIndex(pkg, "/") + 1:]
	g.db = sqlx.MustOpen("mysql", address)
	defer g.db.Close()
	t, err := template.New("default").Funcs(fns).Parse(tpl)
	if err != nil {
		log.Println(err)
		return
	}
	tables := []*Table{}
	err = g.db.Select(&tables, `SELECT
 table_name AS name
FROM
 information_schema.tables
WHERE
 table_schema = DATABASE()`)
	if err != nil {
		panic(err)
	}

	for _, v := range tables {
		log.Println(v.Name)
		cols := []*Column{}
		err := g.db.Select(&cols, "SHOW COLUMNS FROM " + v.Name)
		if err != nil {
			panic(err)
		}

		cs := []*GoColumn{}
		needTimeImport := false
		idElemField := ""
		idElemCol := ""
		for _, c := range cols {
			log.Println(c)
			gc := &GoColumn{}
			if c.Null.String != "NO" {
				gc.CanNull = true
			}
			colType := strings.ToLower(c.Type)
			gc.Name = convertToCamel(c.Field)
			gc.Annotation = "`db:\"" + c.Field + "\" json:\"" + convertToLowerCamel(c.Field) + "\"`"
			if strings.Contains(colType, "char") || strings.Contains(colType, "text") {
				gc.Type = "string"
			} else if strings.Contains(colType, "tinyint") || strings.Contains(colType, "bool") {
				gc.Type = "int"
			} else if strings.Contains(colType, "int") {
				if strings.ToLower(c.Field) == "id" {
					idElemField = c.Field
					idElemCol = gc.Name
				}
				gc.Type = "int"
			} else if strings.Contains(colType, "float") {
				gc.Type = "float32"
			} else if strings.Contains(colType, "double") {
				gc.Type = "float64"
			} else if strings.Contains(colType, "time") || strings.Contains(colType, "date") || strings.Contains(colType, "datetime") || strings.Contains(colType, "timestamp") {
				gc.Type = "time.Time"
				needTimeImport = true
			} else if strings.Contains(colType, "blob") {
				gc.Type = "[]byte"
			}
			cs = append(cs, gc)
		}
		f, _ := os.Create(GOPATH + string(os.PathSeparator) + "src" + string(os.PathSeparator) + strings.Replace(pkg, "/", string(os.PathSeparator), -1) + string(os.PathSeparator) + v.Name + ".go")
		err = t.Execute(f, map[string]interface{}{"idElemCol": idElemCol, "idElemField": idElemField, "name": convertToCamel(v.Name), "needTimeImport": needTimeImport, "pkg": basePkg, "cols": cs, "fields": cols, "table": v.Name})
		if err != nil {
			log.Println(err)
		}
		f.Close()
	}
	cmd := exec.Command("go", "fmt", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func convertToCamel(name string) string {
	name = strings.ToLower(name)
	nName := strings.ToUpper(name[:1]) + name[1:]
	nName = regexp.MustCompile("\\_[a-zA-Z]").ReplaceAllStringFunc(nName, func(from string) string {
		return strings.ToUpper(from[1:])
	})
	return nName
}
func convertToLowerCamel(name string) string {
	name = strings.ToLower(name)
	nName := name
	nName = regexp.MustCompile("\\_[a-zA-Z]").ReplaceAllStringFunc(nName, func(from string) string {
		return strings.ToUpper(from[1:])
	})
	return nName
}

var tpl = `
package {{.pkg}}

import (
	"github.com/jmoiron/sqlx"
	"database/sql"
	"bytes"
	{{if .needTimeImport}}"time"{{end}}
)
type {{.name}} struct{
	{{range .cols}}
	{{.Name}} {{if .CanNull}}*{{end}}{{.Type}} {{.Annotation}}
	{{end}}
}
type {{.name}}Dao struct{
	DB *sqlx.DB ` + "`sm:\"@.(.)\"`" + `
}
func New{{.name}}Dao(db *sqlx.DB)(*{{.name}}Dao){
	return &{{.name}}Dao{db}
}
func (this *{{.name}}Dao)Fields()string{
	return "{{range $index,$el := .fields }}` + "`" + `{{$el.Field}}` + "`" + ` {{if last $index $.fields}}{{else}},{{end}}{{end}}"
}
func (this *{{.name}}Dao)FindByID(ID int)(one *{{.name}},err error){
	one=&{{.name}}{}
	err=this.DB.Get(one,"select "+this.Fields()+" from ` + "`" + `{{.table}}` + "`" + ` where ` + "`" + `{{.idElemField}}` + "`" + `=? limit 1",ID)
	if err== sql.ErrNoRows{
		return nil,nil
	}
	if err!=nil {
		return nil,err
	}
	return one,nil
}
func (this *{{.name}}Dao)Page(page ,pageSize int)(list []*{{.name}},err error){
	list=[]*{{.name}}{}
	err=this.DB.Select(&list,"select * from ` + "`" + `{{.table}}` + "`" + ` limit ?,?",page*(pageSize-1),pageSize)
	if err!=nil && err!= sql.ErrNoRows {
		return nil,err
	}
	return list,nil
}
func (this *{{.name}}Dao)WherePage(whereSql string,page ,pageSize int,args ... interface{})(list []*{{.name}},err error){
	list=[]*{{.name}}{}
	args = append(args,page*(pageSize-1),pageSize)
	err=this.DB.Select(&list,"select * from ` + "`" + `{{.table}}` + "`" + ` where "+whereSql+" limit ?,?",args...)
	if err!=nil && err!= sql.ErrNoRows {
		return nil,err
	}
	return list,nil
}
func (this *{{.name}}Dao)All()(list []*{{.name}},err error){
	list=[]*{{.name}}{}
	err=this.DB.Select(&list,"select "+this.Fields()+" from ` + "`" + `{{.table}}` + "`" + ` ")
	if err!=nil && err!= sql.ErrNoRows {
		return nil,err
	}
	return list,nil
}
func (this *{{.name}}Dao)Where(whereSql string,args ... interface{})(list []*{{.name}},err error){
	list=[]*{{.name}}{}
	err=this.DB.Select(&list,"select "+this.Fields()+" from ` + "`" + `{{.table}}` + "`" + ` where "+whereSql,args...)
	if err!=nil && err!= sql.ErrNoRows {
		return nil,err
	}
	return list,nil
}
func (this *{{.name}}Dao)Find(whereSql string,args ... interface{})(one *{{.name}},err error){
	one=&{{.name}}{}
	err=this.DB.Get(one,"select "+this.Fields()+" from ` + "`" + `{{.table}}` + "`" + ` where "+whereSql,args...)
	if err== sql.ErrNoRows{
		return nil,nil
	}
	if err!=nil {
		return nil,err
	}
	return one,nil
}
func (this *{{.name}}Dao)Insert(one *{{.name}})(res sql.Result,err error){
	return this.DB.Exec("insert into ` + "`" + `{{ .table }}` + "`" + ` ( {{range $index,$el := .fields }}{{if ne $el.Field $.idElemField}}` + "`" + `{{$el.Field}}` + "`" + ` {{if last $index $.fields}}{{else}},{{end}}{{end}}{{end}})values( {{range $index,$el := $.fields }} {{if ne $el.Field $.idElemField}}?{{if last $index $.fields}}{{else}},{{end}}{{end}}{{end}})",{{range $index,$el := .cols }}{{if ne $el.Name $.idElemCol}}one.{{$el.Name}} {{if last $index $.cols}} {{else}},{{end}}{{end}}{{end}})
}
func (this *{{.name}}Dao)UpdateAll(one *{{.name}})(res sql.Result,err error){
	return this.DB.Exec("update  ` + "`" + `{{ .table }}` + "`" + ` set {{range $index,$el := .fields }}{{if ne $el.Field $.idElemField}}` + "`" + `{{$el.Field}}` + "`" + `=? {{if last $index $.fields}}{{else}},{{end}}{{end}}{{end}} where id = ? ",{{range $index,$el := .cols }}{{if ne $el.Name $.idElemCol}}one.{{$el.Name}} {{if last $index $.cols}} {{else}},{{end}}{{end}}{{end}},one.Id)
}
func (this *{{.name}}Dao)InsertX(one *{{.name}})(err error){
	res,err:=this.Insert(one)
	if err!=nil{
		return err
	}
	id,err:=res.LastInsertId()
	if err!=nil{
		return err
	}
	one.Id = int(id)
	return nil
}
func (this *{{.name}}Dao)UpdateWhere(sqlStr string, kv map[string]interface{},args ... interface{}) (res sql.Result, err error) {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("update  ` + "`" + `{{ .table }}` + "`" + ` set ")
	i := 0
	length := len(kv)
	vs := []interface{}{}
	for k, v := range kv {
		i++
		buf.WriteString("` + "`" + `")
		buf.WriteString(k)
		buf.WriteString("` + "`" + `")
		buf.WriteString("=?")

		if i < length {
			buf.WriteString(",")
		}
		vs = append(vs, v)
	}
	vs = append(vs, args...)
	buf.WriteString(" where "+sqlStr)
	return this.DB.Exec(buf.String(), vs...)
}
func (this *{{.name}}Dao)Update(ID int, kv map[string]interface{}) (res sql.Result, err error) {
	buf := bytes.NewBuffer([]byte{})
	buf.WriteString("update  ` + "`" + `{{ .table }}` + "`" + ` set ")
	i := 0
	length := len(kv)
	vs := []interface{}{}
	for k, v := range kv {
		i++
		buf.WriteString("` + "`" + `")
		buf.WriteString(k)
		buf.WriteString("` + "`" + `")
		buf.WriteString("=?")

		if i < length {
			buf.WriteString(",")
		}
		vs = append(vs, v)
	}
	vs = append(vs, ID)
	buf.WriteString(" where id = ?")
	return this.DB.Exec(buf.String(), vs...)
}
`
var fns = template.FuncMap{
	"last": func(x int, a interface{}) bool {
		log.Println("[last]", x, a)
		return x == reflect.ValueOf(a).Len() - 1
	},
}

