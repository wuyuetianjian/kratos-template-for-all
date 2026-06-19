package data

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"temperate/internal/conf"
	"temperate/internal/data/ent"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v3/log"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/wire"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/robfig/cron/v3"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewCron, NewAuthRepo, NewSSOProviderRepo)

// Data .
type Data struct {
	log      *slog.Logger
	cnf      *conf.Data
	WriteEnt *ent.Client
	ReadEnt  *ent.Client
}

// NewData .
func NewData(c *conf.Data, logger *slog.Logger) (*Data, func(), error) {
	if logger == nil {
		logger = log.Default()
	}
	writeEnt, err := newEntClient(c.GetDatabase(), c.GetDatabase().GetSource())
	if err != nil {
		return nil, nil, err
	}
	readEnt := writeEnt
	readSources := c.GetDatabase().GetReadSources()
	if len(readSources) > 0 && readSources[0] != "" && readSources[0] != c.GetDatabase().GetSource() {
		readEnt, err = newEntClient(c.GetDatabase(), readSources[0])
		if err != nil {
			_ = writeEnt.Close()
			return nil, nil, err
		}
	}
	if c.GetDatabase().GetAutoMigrate() {
		if err := writeEnt.Schema.Create(context.Background()); err != nil {
			_ = readEnt.Close()
			if readEnt != writeEnt {
				_ = writeEnt.Close()
			}
			return nil, nil, err
		}
	}
	cleanup := func() {
		logger.Info("closing the data resources")
		if readEnt != writeEnt {
			_ = readEnt.Close()
		}
		_ = writeEnt.Close()
	}
	return &Data{
		log:      logger.With("module", "data"),
		cnf:      c,
		WriteEnt: writeEnt,
		ReadEnt:  readEnt,
	}, cleanup, nil
}

func newEntClient(c *conf.Data_Database, source string) (*ent.Client, error) {
	if c == nil {
		return nil, fmt.Errorf("database config is required")
	}
	driverName, dialectName, err := databaseDriver(c.GetDriver())
	if err != nil {
		return nil, err
	}
	if source == "" {
		return nil, fmt.Errorf("database source is required")
	}
	db, err := sql.Open(driverName, source)
	if err != nil {
		return nil, err
	}
	if c.GetMaxIdleConns() > 0 {
		db.SetMaxIdleConns(int(c.GetMaxIdleConns()))
	}
	if c.GetMaxOpenConns() > 0 {
		db.SetMaxOpenConns(int(c.GetMaxOpenConns()))
	}
	if c.GetConnMaxLifetime() != nil {
		db.SetConnMaxLifetime(c.GetConnMaxLifetime().AsDuration())
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialectName, db)))
	if c.GetDebug() {
		client = client.Debug()
	}
	return client, nil
}

func databaseDriver(driver string) (string, string, error) {
	switch driver {
	case "mysql":
		return "mysql", dialect.MySQL, nil
	case "pgsql", "postgres", "postgresql":
		return "pgx", dialect.Postgres, nil
	default:
		return "", "", fmt.Errorf("unsupported database driver %q", driver)
	}
}

func NewCron() *cron.Cron {
	return cron.New()
}
