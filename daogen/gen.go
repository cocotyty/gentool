package daogen

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"log"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"text/template"
)

type Gen struct {
	db             *sqlx.DB
	disableBoolean bool
	enableCache    bool
	specialTables  []string
}

func (g *Gen) SpecialTables(tables ...string) *Gen {
	g.specialTables = tables
	return g
}
func (g *Gen) EnableBoolean() *Gen {
	g.disableBoolean = true
	return g
}

func (g *Gen) EnableCache() *Gen {
	g.enableCache = true
	return g
}
func NewGen() *Gen {
	return &Gen{}
}

type Table struct {
	Name    string `db:"name"`
	Comment string `db:"table_comment"`
}
type Column struct {
	Field      string         `db:"Field"`
	Type       string         `db:"Type"`
	Null       sql.NullString `db:"Null"`
	Key        []byte         `db:"Key"`
	Default    []byte         `db:"Default"`
	Comment    string         `db:"Comment"`
	Extra      []byte         `db:"Extra"`
	Privileges []byte         `db:"Privileges"`
	Collation  []byte         `db:"Collation"`
}
type GoColumn struct {
	Name       string
	Type       string
	Annotation string
	DBName     string
	CanNull    bool
	Comment    string
}

var GOPATH string

func init() {
	GOPATH = strings.Split(os.Getenv("GOPATH"), string(os.PathListSeparator))[0]
}
func (g *Gen) G(pkg, address string) {
	g.g(pkg, address, tpl)
}
func (g *Gen) ProductTable(pkg, address string, table string) {
}
func (g *Gen) Models(pkg, address string) {
	g.g(pkg, address, modelOnlyTpl)
}
func (g *Gen) g(pkg, address, tpl string) {
	os.MkdirAll(GOPATH+string(os.PathSeparator)+"src"+string(os.PathSeparator)+strings.Replace(pkg, "/", string(os.PathSeparator), -1), 0777)
	basePkg := pkg[strings.LastIndex(pkg, "/")+1:]
	g.db = sqlx.MustOpen("mysql", address)
	defer g.db.Close()
	t, err := template.New("default").Funcs(fns).Parse(tpl)
	if err != nil {
		log.Println(err)
		return
	}
	tables := []*Table{}
	err = g.db.Select(&tables, `SELECT
 table_name AS name,table_comment
FROM
 information_schema.tables
WHERE
 table_schema = DATABASE()`)
	if err != nil {
		panic(err)
	}
	if g.specialTables != nil {
		StringSet := map[string]bool{}
		for _, v := range g.specialTables {
			StringSet[v] = true
		}
		for _, v := range tables {
			if StringSet[v.Name] {
				g.genTable(v, t, pkg, basePkg)
			}
		}
	} else {
		for _, v := range tables {
			g.genTable(v, t, pkg, basePkg)
		}
	}

	cmd := exec.Command("go", "fmt", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}
func (g *Gen) genTable(v *Table, t *template.Template, pkg string, basePkg string) {
	cols := []*Column{}
	err := g.db.Select(&cols, "SHOW FULL COLUMNS FROM `"+v.Name+"`")
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
		gc.Comment = c.Comment
		gc.Name = convertToCamel(c.Field)
		if gc.Comment == "" {
			gc.Comment = c.Field
		}
		gc.Annotation = "`db:\"" + c.Field + "\" json:\"" + convertToLowerCamel(c.Field) + "\"`"
		gc.DBName = c.Field
		if strings.Contains(colType, "char") || strings.Contains(colType, "text") {
			gc.Type = "string"
		} else if strings.Contains(colType, "tinyint") || strings.Contains(colType, "bool") {
			if g.disableBoolean {
				gc.Type = "bool"
			} else {
				gc.Type = "int"
			}
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
	err = t.Execute(f, map[string]interface{}{"idElemCol": idElemCol, "idElemField": idElemField, "name": convertToCamel(v.Name), "comment": strings.Replace(v.Comment, "\n", "\n //", -1), "needCache": g.enableCache, "needTimeImport": needTimeImport, "pkg": basePkg, "cols": cs, "fields": cols, "table": v.Name})
	if err != nil {
		log.Println(err)
	}
	f.Close()
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

var modelOnlyTpl = `
package {{.pkg}}

import (
	{{if .needTimeImport}}"time"{{end}}
)
// {{.comment}}
type {{.name}} struct{
	{{range .cols}}
	// {{.Comment}}
	{{.Name}} {{if .CanNull}}*{{end}}{{.Type}} {{.Annotation}}
	{{end}}
}`

var tpl = `
package {{.pkg}}

import (
	"github.com/jmoiron/sqlx"
	"database/sql"
{{if .needCache}}
	"github.com/cocotyty/gentool/gziptool"
	"gopkg.in/redis.v4"
	"strconv"
{{end}}"bytes"
	{{if or .needTimeImport .needCache}}"time"{{end}}
)

{{if .needCache}}
type IDCache{{.name}}Dao struct{
	*{{.name}}Dao ` + "`sm:\"(.Ref)\"`" + `
	Client *redis.Client ` + "`sm:\"(.Client)\"`" + `
	Prefix string ` + "`sm:\"(.Prefix)\"`" + `
	Exp time.Duration
}
func (d *IDCache{{.name}}Dao)Update(ID int, kv map[string]interface{}) (res sql.Result, err error) {
	key:=d.Prefix+strconv.Itoa(ID)
	res,err=d.{{.name}}Dao.Update(ID,kv)
	if err == nil {
		d.Client.Del(key).Result()
	}
	return
}
func (d *IDCache{{.name}}Dao) FindByID(ID int)(one *{{.name}},err error){
	key:=d.Prefix+strconv.Itoa(ID)
	data,err:=d.Client.Get(key).Bytes()
	if err==nil{
		one=&{{.name}}{}
		err=gziptool.GUnzipJSON(data,one)
		if err == nil{
			return one,nil
		}
	}
	one,err=d.{{.name}}Dao.FindByID(ID)
	if err == nil {
		data,_:=gziptool.JSONGzip(one)
		d.Client.Set(key,data,d.Exp).Result()
	}
	return
}

func (d *IDCache{{.name}}Dao) InsertX(one *{{.name}})(err error){
	if err:=d.{{.name}}Dao.InsertX(one);err!=nil{
		return err
	}
	another, err := d.FindByID(one.Id)
	if err != nil {
		return nil
	}
	*one = *another
	return nil
}
{{end}}
// {{.comment}}
type {{.name}} struct{
	{{range .cols}}
	// {{.Comment}}
	{{.Name}} {{if .CanNull}}*{{end}}{{.Type}} {{.Annotation}}
	{{end}}
}
type {{.name}}Dao struct{
	{{range .cols}}
	Col{{.Name}} string ` + "`sm:\"-\"`" + `
	{{end}}
	Columns []string ` + "`sm:\"-\"`" + `
	Table string  ` + "`sm:\"-\"`" + `
	DB *sqlx.DB ` + "`sm:\"@.(.)\"`" + `
}
func New{{.name}}Dao(db *sqlx.DB)(*{{.name}}Dao){
	 dao:=&{{.name}}Dao{DB:db}
	 dao.Init()
	 return dao
}
func (dao *{{.name}}Dao)Init(){
	dao.Columns=[]string{ {{range .cols}}
	 "{{.DBName}}",
	{{end}} }
	{{range .cols}}
	dao.Col{{.Name}} = "{{.DBName}}"
	{{end}}
	dao.Table = "{{.table}}"
}
func (dao *{{.name}}Dao)Fields()string{
	return "{{range $index,$el := .fields }}` + "`" + `{{$el.Field}}` + "`" + ` {{if last $index $.fields}}{{else}},{{end}}{{end}}"
}

func (dao *{{.name}}Dao)FindByID(ID int)(one *{{.name}},err error){
	one=&{{.name}}{}
	err=dao.DB.Get(one,"select "+dao.Fields()+" from ` + "`" + `{{.table}}` + "`" + ` where ` + "`" + `{{.idElemField}}` + "`" + `=? limit 1",ID)
	if err== sql.ErrNoRows{
		return nil,nil
	}
	if err!=nil {
		return nil,err
	}
	return one,nil
}
func (dao *{{.name}}Dao)Page(page ,pageSize int)(list []*{{.name}},err error){
	list=[]*{{.name}}{}
	err=dao.DB.Select(&list,"select "+dao.Fields()+" from ` + "`" + `{{.table}}` + "`" + ` limit ?,?",pageSize*(page-1),pageSize)
	if err!=nil && err!= sql.ErrNoRows {
		return nil,err
	}
	return list,nil
}
func (dao *{{.name}}Dao)WherePage(whereSql string,page ,pageSize int,args ... interface{})(list []*{{.name}},err error){
	list=[]*{{.name}}{}
	args = append(args,pageSize*(page-1),pageSize)
	err=dao.DB.Select(&list,"select "+dao.Fields()+" from ` + "`" + `{{.table}}` + "`" + ` where "+whereSql+" limit ?,?",args...)
	if err!=nil && err!= sql.ErrNoRows {
		return nil,err
	}
	return list,nil
}

func (dao *{{.name}}Dao)RawSelect(str string,args []interface{})(list []*{{.name}},err error){
	list=[]*{{.name}}{}
	err=dao.DB.Select(&list,str,args...)
	if err!=nil && err!= sql.ErrNoRows {
		return nil,err
	}
	return list,nil
}

func (dao *{{.name}}Dao)All()(list []*{{.name}},err error){
	list=[]*{{.name}}{}
	err=dao.DB.Select(&list,"select "+dao.Fields()+" from ` + "`" + `{{.table}}` + "`" + ` ")
	if err!=nil && err!= sql.ErrNoRows {
		return nil,err
	}
	return list,nil
}
func (dao *{{.name}}Dao)Where(whereSql string,args ... interface{})(list []*{{.name}},err error){
	list=[]*{{.name}}{}
	err=dao.DB.Select(&list,"select "+dao.Fields()+" from ` + "`" + `{{.table}}` + "`" + ` where "+whereSql,args...)
	if err!=nil && err!= sql.ErrNoRows {
		return nil,err
	}
	return list,nil
}
func (dao *{{.name}}Dao)Find(whereSql string,args ... interface{})(one *{{.name}},err error){
	one=&{{.name}}{}
	err=dao.DB.Get(one,"select "+dao.Fields()+" from ` + "`" + `{{.table}}` + "`" + ` where "+whereSql,args...)
	if err== sql.ErrNoRows{
		return nil,nil
	}
	if err!=nil {
		return nil,err
	}
	return one,nil
}
func (dao *{{.name}}Dao)Insert(one *{{.name}})(res sql.Result,err error){
	return dao.DB.Exec("insert into ` + "`" + `{{ .table }}` + "`" + ` ( {{range $index,$el := .fields }}{{if ne $el.Field $.idElemField}}` + "`" + `{{$el.Field}}` + "`" + ` {{if last $index $.fields}}{{else}},{{end}}{{end}}{{end}})values( {{range $index,$el := $.fields }} {{if ne $el.Field $.idElemField}}?{{if last $index $.fields}}{{else}},{{end}}{{end}}{{end}})",{{range $index,$el := .cols }}{{if ne $el.Name $.idElemCol}}one.{{$el.Name}} {{if last $index $.cols}} {{else}},{{end}}{{end}}{{end}})
}
func (dao *{{.name}}Dao)UpdateAll(one *{{.name}})(res sql.Result,err error){
	return dao.DB.Exec("update  ` + "`" + `{{ .table }}` + "`" + ` set {{range $index,$el := .fields }}{{if ne $el.Field $.idElemField}}` + "`" + `{{$el.Field}}` + "`" + `=? {{if last $index $.fields}}{{else}},{{end}}{{end}}{{end}} where id = ? ",{{range $index,$el := .cols }}{{if ne $el.Name $.idElemCol}}one.{{$el.Name}} {{if last $index $.cols}} {{else}},{{end}}{{end}}{{end}},one.Id)
}
func (dao *{{.name}}Dao)InsertX(one *{{.name}})(err error){
	res,err:=dao.Insert(one)
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
func (dao *{{.name}}Dao)UpdateWhere(sqlStr string, kv map[string]interface{},args ... interface{}) (res sql.Result, err error) {
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
	return dao.DB.Exec(buf.String(), vs...)
}
func (dao *{{.name}}Dao)Update(ID int, kv map[string]interface{}) (res sql.Result, err error) {
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
	return dao.DB.Exec(buf.String(), vs...)
}
`
var fns = template.FuncMap{
	"last": func(x int, a interface{}) bool {
		log.Println("[last]", x, a)
		return x == reflect.ValueOf(a).Len()-1
	},
}
