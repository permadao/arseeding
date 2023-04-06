package config

import (
	"github.com/go-co-op/gocron"
	"github.com/permadao/arseeding/config/schema"
	"time"
)

type Config struct {
	wdb            *Wdb
	speedTxFee     int64
	bundleServeFee int64
	ipWhiteList    map[string]struct{}
	scheduler      *gocron.Scheduler
	Param          schema.Param
}

func New(configDSN, sqliteDir string, useSqlite bool) *Config {
	wdb := &Wdb{}
	if useSqlite {
		wdb = NewSqliteDb(sqliteDir)
	} else {
		wdb = NewMysqlDb(configDSN)
	}
	err := wdb.Migrate()
	if err != nil {
		panic(err)
	}
	fee, err := wdb.GetFee()
	if err != nil {
		panic(err)
	}
	param, err := wdb.GetParam()
	if err != nil {
		panic(err)
	}
	return &Config{
		wdb:            wdb,
		speedTxFee:     fee.SpeedTxFee,
		bundleServeFee: fee.BundleServeFee,
		ipWhiteList:    make(map[string]struct{}),
		scheduler:      gocron.NewScheduler(time.UTC),
		Param:          param,
	}
}

func (c *Config) GetSpeedFee() int64 {
	return c.speedTxFee
}

func (c *Config) GetServeFee() int64 {
	return c.bundleServeFee
}

func (c *Config) GetIPWhiteList() *map[string]struct{} {
	return &c.ipWhiteList
}

func (c *Config) Run() {
	go c.runJobs()
}

func (c *Config) Close() {
	c.wdb.Close()
}
