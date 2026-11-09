// Package testmysql restricts local CRM tools to the dedicated test schema.
package testmysql

import (
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

const Database = "dealflow_crm_test"

func Config(dsn string) (*mysql.Config, error) {
	if dsn == "" {
		return nil, errors.New("set DEALFLOW_TEST_DSN")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, errors.New("invalid DEALFLOW_TEST_DSN format")
	}
	if cfg.DBName != Database {
		return nil, fmt.Errorf("DEALFLOW_TEST_DSN must select %s", Database)
	}
	return cfg, nil
}
