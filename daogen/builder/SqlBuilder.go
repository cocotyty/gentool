package builder

import (
	"bytes"
)

type sqlBuilder struct {
	buf  bytes.Buffer
	args []interface{}
}
type part struct {
	sql  string
	args []interface{}
}
type selectSqlBuilder sqlBuilder
type fromSqlBuilder sqlBuilder
type whereSqlBuilder sqlBuilder

func Select(names ... string) (*selectSqlBuilder) {
	builder := &selectSqlBuilder{}
	builder.buf.Write([]byte(`SELECT `))
	for i, v := range names {
		builder.buf.WriteByte('`')
		builder.buf.WriteString(v)
		builder.buf.WriteByte('`')
		if i != len(names)-1 {
			builder.buf.WriteByte(',')
		}
	}
	return builder
}
func (builder *selectSqlBuilder) From(table string) (*fromSqlBuilder) {
	builder.buf.Write([]byte(" FROM `"))
	builder.buf.WriteString(table)
	builder.buf.WriteByte('`')
	return (*fromSqlBuilder)(builder)
}

var ALL = part{"", nil}

func (builder *fromSqlBuilder) Where(p part) (*whereSqlBuilder) {
	if p.sql == "" {
		return (*whereSqlBuilder)(builder)
	}
	builder.buf.Write([]byte(" WHERE "))
	builder.buf.WriteString(p.sql)
	builder.args = append(builder.args, p.args...)
	return (*whereSqlBuilder)(builder)
}
func (builder *whereSqlBuilder) OrderBy(orders ... order) (*whereSqlBuilder) {
	builder.buf.WriteString(` order by `)
	for i, v := range orders {
		builder.buf.WriteString(string(v))
		if i != len(orders)-1 {
			builder.buf.WriteByte(',')
		}
	}
	return builder
}
func (builder *whereSqlBuilder) GroupBy(names ... string) (*whereSqlBuilder) {
	builder.buf.WriteString(` group by `)
	for i, v := range names {
		builder.buf.WriteString(string(v))
		if i != len(names)-1 {
			builder.buf.WriteByte(',')
		}
	}
	return builder
}

type query interface {
	Select(interface{}, string, ... interface{})
}

func (builder *whereSqlBuilder) Build() (sql string, args []interface{}) {
	return builder.buf.String(), builder.args
}

func (builder *fromSqlBuilder) Limit(offset int, limit int) {

}

type order string

func Asc(name string) (order) {
	return order(" `" + name + "` asc ")
}
func Desc(name string) (order) {
	return order(" `" + name + "` desc ")
}
func And(parts ... part) (part) {
	sql := ""
	args := []interface{}{}
	for i, v := range parts {
		args = append(args, v.args...)
		if i == len(parts)-1 {
			sql += "(" + v.sql + ")  "
		} else {
			sql += "(" + v.sql + ")  AND "
		}
	}
	return part{sql, args}

}
func Or(parts ... part) (part) {
	sql := ""
	args := []interface{}{}
	for i, v := range parts {
		args = append(args, v.args...)
		if i == len(parts)-1 {
			sql += "(" + v.sql + ")  "
		} else {
			sql += "(" + v.sql + ")  OR "
		}
	}
	return part{sql, args}

}
func In(name string, value ... interface{}) (part) {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte('`')
	buf.WriteString(name)
	buf.WriteString("` in (")
	for i := 0; i < len(value); i++ {
		buf.WriteByte('?')
		if i != len(value)-1 {
			buf.WriteByte(',')
		}
	}
	buf.WriteByte(')')
	return part{buf.String(), value}
}

func Equal(name string, value interface{}) (part) {
	return part{sql: name + "= ?", args: []interface{}{value}}
}
func NotNull(name string, value interface{}) (part) {
	return part{sql: name + " not null ", args: []interface{}{value}}
}
func IsNull(name string, value interface{}) (part) {
	return part{sql: name + " is null ", args: []interface{}{value}}
}
func BiggerThan(name string, value interface{}) (part) {
	return part{sql: name + " > ? ", args: []interface{}{value}}
}
func BiggerThanOrEquals(name string, value interface{}) (part) {
	return part{sql: name + " >= ? ", args: []interface{}{value}}
}
func SmallerThan(name string, value interface{}) (part) {
	return part{sql: name + " < ? ", args: []interface{}{value}}
}
func SmallerThanOrEquals(name string, value interface{}) (part) {
	return part{sql: name + " <= ? ", args: []interface{}{value}}
}
func Like(name string, str string) (part) {
	return part{sql: name + "= ?", args: []interface{}{"%" + str + "%"}}
}

func LikeLeft(name string, str string) (part) {
	return part{sql: name + "= ?", args: []interface{}{str + "%"}}
}
