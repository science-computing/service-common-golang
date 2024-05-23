// Package dbutil basically provides DbContext to simplify DB interaction
package dbutil

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/science-computing/service-common-golang/apputil"

	"github.com/apex/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	// initializes postgres driver

	//_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/pkg/errors"
)

type DatasetFlag uint64

var SKIP_ERROR error = fmt.Errorf("Skipping due to previous error")

const (
	Committed DatasetFlag = 1 << iota
	CheckedOut
)

var (
	logger         = apputil.InitLogging()
	activeContexts = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "active_db_contexts",
		Help: "The total number active db contexts",
	})
)

type DbConnectionHelper struct {
	DbConnectionURL string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifeTime int
	dbConnection    *sql.DB
	lock            sync.Mutex
}

type RowsAccessor interface {
	Next() bool
	Scan(dest ...interface{}) error
}

type DbAccessor interface {
	RegisterErrorHandler(errorHandler func(err error))
	QueryRow(query string, args ...interface{}) (*sql.Row, error)
	ScanQueryRow(supressErrNoRows bool, query Query, destination []interface{}) error
	Query(query string, args ...interface{}) (RowsAccessor, error)
	Execute(query string, args ...interface{}) error
	Commit(restartTx bool) error
	Rollback(restartTx bool) error
	Close() error
	LastError() error
	SetLastError(err error)
	ResetError()
}

// DbContext simplifies db interaction by providing a context to execute
// queries with or without transactional and/or cancellation context

type DbContext struct {
	// Err is deprecated - dont use anymore directly
	err          error
	db           *sql.DB
	ctx          *context.Context
	tx           *sql.Tx
	errorHandler func(error)
}

// Query allows to pass parametrized query an single function parameter
type Query struct {
	Query string
	Args  []interface{}
}

// GetDbContext returns a context in which queries (including inserts, deletes) can be executed.
// ctx allows optional context cancellation if not nil
// DbContext.Err and any transaction are resetted
func (helper *DbConnectionHelper) GetDbContext(ctx *context.Context, useTransaction bool) (dbContext *DbContext) {
	helper.lock.Lock()
	dbContext = &DbContext{ctx: ctx}
	func() {
		defer helper.lock.Unlock()

		dbConnectionURL := helper.DbConnectionURL

		log.Debugf("Get DbContext for URL [%v]", dbConnectionURL)

		dbContext.db = helper.dbConnection
		if dbContext.db == nil {
			if dbContext.db, dbContext.err = getDBConnection(dbConnectionURL); dbContext.err != nil {
				return
			}
			helper.dbConnection = dbContext.db
			dbContext.db.SetMaxOpenConns(helper.MaxOpenConns)
			dbContext.db.SetMaxIdleConns(helper.MaxIdleConns)
			dbContext.db.SetConnMaxLifetime(time.Duration(helper.ConnMaxLifeTime) * time.Second)
		}
	}()

	if useTransaction {
		//open transaction with/without cancellation context
		if ctx != nil {
			dbContext.tx, dbContext.err = dbContext.db.BeginTx(*ctx, nil)
		} else {
			dbContext.tx, dbContext.err = dbContext.db.Begin()
		}
	} else {
		dbContext.tx = nil
	}

	activeContexts.Inc()

	return dbContext
}

// CloseContexts closes all open db connections
func (helper *DbConnectionHelper) CloseContexts() {
	helper.lock.Lock()
	defer helper.lock.Unlock()

	if helper.dbConnection != nil {
		helper.dbConnection.Close()
		helper.dbConnection = nil
	}
}

// RegisterErrorHandler registers function as error handler to call in case
// DbContext.Err is set. Any previous handler is overwritten.
func (dbContext *DbContext) RegisterErrorHandler(errorHandler func(err error)) {
	dbContext.errorHandler = errorHandler
}

func (dbContext *DbContext) handleError() {
	if dbContext.err != nil && dbContext.errorHandler != nil {
		dbContext.errorHandler(dbContext.err)
	}
}

// QueryRow returns at most one row for given query with given substituion paramaters.
// The operation becomes a no-op if there is a previous error in DbContext.err.
func (dbContext *DbContext) QueryRow(query string, args ...interface{}) (*sql.Row, error) {
	if dbContext.err != nil {
		log.Errorf("Skipping QueryRow due to previous error [%v]", dbContext.err)
		return nil, SKIP_ERROR
	}

	log.Debugf("Executing SQL [%v] with args %v", query, args)
	row := dbContext.db.QueryRow(query, args...)

	dbContext.handleError()
	return row, dbContext.err
}

// ScanQueryRow executes the given query with optional args and writes colums of
// the first row (if present) to given destination parameters
// The operation becomes a no-op if there is a previous error in DbContext.err.
// If supressErrNoRows and error occurrs, destination value are reset to ""
func (dbContext *DbContext) ScanQueryRow(supressErrNoRows bool, query Query, destination []interface{}) error {
	if dbContext.err != nil {
		log.Errorf("Skipping QueryRow [%v] due to previous error [%v]", query, dbContext.err)
		return SKIP_ERROR
	}

	log.Debugf("Executing SQL [%v] with args %v", query, query.Args)
	var row *sql.Row
	if query.Args != nil {
		row = dbContext.db.QueryRow(query.Query, query.Args...)

	} else {
		row = dbContext.db.QueryRow(query.Query)
	}

	// copy column values
	dbContext.err = row.Scan(destination...)

	// supress sql.ErrNoRows
	if dbContext.err != nil && dbContext.err == sql.ErrNoRows && supressErrNoRows {
		dbContext.err = nil
		// unset all destination parameters
		for index := range destination {
			// TODO find better solution than casting to string as *interface{} cannot be dereferenced
			valRef := destination[index].(*string)
			*valRef = ""
		}
	}

	dbContext.handleError()
	return dbContext.err
}

// Query returns all rows for given query with given substituion paramaters.
// The operation becomes a no-op if there is a previous error in DbContext.err.
func (dbContext *DbContext) Query(query string, args ...interface{}) (RowsAccessor, error) {
	if dbContext.err != nil {
		log.Errorf("Skipping Query [%v] due to previous error [%v]", query, dbContext.err)
		return nil, SKIP_ERROR
	}

	log.Debugf("Executing SQL [%v] with args %v", query, args)

	dbContext.handleError()
	var rows *sql.Rows
	rows, dbContext.err = dbContext.db.Query(query, args...)
	return rows, dbContext.err
}

// Execute runs given query with given substitution parameters as for $1 etc.
// The operation becomes a no-op if there is a previous error in DbContext.err
func (dbContext *DbContext) Execute(query string, args ...interface{}) error {
	if dbContext.err != nil {
		log.Errorf("Skipping Execute [%v] due to previous error [%v]", query, dbContext.err)
		return SKIP_ERROR
	}

	log.Debugf("Executing SQL [%v] with args %v", query, args)

	// execute in transaction if present
	if dbContext.tx != nil {
		// execute with context cancellation
		if dbContext.ctx != nil {
			_, dbContext.err = dbContext.tx.ExecContext(*dbContext.ctx, query, args...)
		} else {
			// execute without context cancellation
			_, dbContext.err = dbContext.tx.Exec(query, args...)
		}
		if dbContext.err != nil {
			dbContext.err = errors.Wrap(dbContext.err, "Insert failed. Transaction rolled back")
			dbContext.tx.Rollback()
			dbContext.handleError()
			return dbContext.err
		}
	} else {
		// otherwise execute without tx
		if dbContext.ctx != nil {
			_, dbContext.err = dbContext.db.ExecContext(*dbContext.ctx, query, args...)
		} else {
			// execute without context cancellation
			_, dbContext.err = dbContext.db.Exec(query, args...)
		}
		if dbContext.err != nil {
			dbContext.err = errors.Wrap(dbContext.err, "Insert failed")
			dbContext.handleError()
			return dbContext.err
		}
	}

	dbContext.handleError()
	return dbContext.err
}

// Commit commits any open transaction if there is one
func (dbContext *DbContext) Commit(restartTx bool) error {
	if dbContext.tx != nil {
		dbContext.tx.Commit()
		if restartTx {
			//reopen transaction with/without cancellation context
			if dbContext.ctx != nil {
				dbContext.tx, dbContext.err = dbContext.db.BeginTx(*dbContext.ctx, nil)
			} else {
				dbContext.tx, dbContext.err = dbContext.db.Begin()
			}
		}
	}
	return dbContext.err
}

// Rollback rolls any open transaction back if there is one
func (dbContext *DbContext) Rollback(restartTx bool) error {
	if dbContext.tx != nil {
		dbContext.tx.Rollback()
		if restartTx {
			//reopen transaction with/without cancellation context
			if dbContext.ctx != nil {
				dbContext.tx, dbContext.err = dbContext.db.BeginTx(*dbContext.ctx, nil)
			} else {
				dbContext.tx, dbContext.err = dbContext.db.Begin()
			}
		}
	}
	return dbContext.err
}

// Close commits the transaction. In case of an error the transaction is rolled back.
// dbContext.Err is set to nil
func (dbContext *DbContext) Close() error {
	// commit in case of no error
	if dbContext.err == nil {
		log.Debug("Committing transaction")
		dbContext.Commit(false)
	} else {
		log.Debugf("Rolling back transaction due to error: %v", dbContext.err)
		dbContext.Rollback(false)
	}

	dbContext.err = nil

	// FIXME: do we need to clean up tx?
	if dbContext.tx != nil {
		dbContext.tx = nil
	}

	activeContexts.Dec()

	return dbContext.err
}

// getDBConnection opens a connection to given dbConnectionUrl
func getDBConnection(dbConnectionURL string) (db *sql.DB, err error) {
	log.Debugf("Opening DB connection to [%v]", dbConnectionURL)
	db, err = sql.Open("pgx", dbConnectionURL)

	if err != nil {
		return nil, errors.Wrapf(err, "Failed to connect to DB [%v]", dbConnectionURL)
	}
	err = db.Ping()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to ping DB [%v]", dbConnectionURL)
	}
	return db, nil
}

func (dbContext *DbContext) LastError() error {
	return dbContext.err
}

func (dbContext *DbContext) ResetError() {
	dbContext.err = nil
}

func (dbContext *DbContext) SetLastError(err error) {
	dbContext.err = err
}
