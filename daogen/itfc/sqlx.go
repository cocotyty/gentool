package itfc

import "github.com/jmoiron/sqlx"

type SqlxHandle interface {
	sqlx.Ext
	Select(dest interface{}, query string, args ...interface{}) error
	Get(dest interface{}, query string, args ...interface{}) error
}
