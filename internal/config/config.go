package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type DB struct {
	Host, Name, User, Password string
	Port                       int
	MaxOpen, MaxIdle           int
	MaxIdleTime, MaxLifetime   time.Duration
}

type Config struct {
	HTTPAddr        string
	LogLevel        string
	ShutdownTimeout time.Duration
	CardDB          DB
	AppDB           DB
	CardDataDir     string
	RebuildEnabled  bool
}

type fileConfig struct {
	HTTP struct {
		Addr            string `yaml:"addr"`
		ShutdownTimeout string `yaml:"shutdown_timeout"`
	} `yaml:"http"`
	Log struct {
		Level string `yaml:"level"`
	} `yaml:"log"`
	CardData struct {
		Dir            string `yaml:"dir"`
		RebuildEnabled *bool  `yaml:"rebuild_enabled"`
	} `yaml:"card_data"`
	Databases struct {
		Card fileDB `yaml:"card"`
		App  fileDB `yaml:"app"`
	} `yaml:"databases"`
}

type fileDB struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	Name        string `yaml:"name"`
	User        string `yaml:"user"`
	Password    string `yaml:"password"`
	MaxOpen     int    `yaml:"max_open_conns"`
	MaxIdle     int    `yaml:"max_idle_conns"`
	MaxIdleTime string `yaml:"conn_max_idle_time"`
	MaxLifetime string `yaml:"conn_max_lifetime"`
}

func Load() (Config, error) { return LoadFile(os.Getenv("CONFIG_FILE")) }

// LoadFile applies defaults, an optional YAML file, then environment overrides.
func LoadFile(path string) (Config, error) {
	db := DB{Host: "127.0.0.1", Port: 3306, MaxOpen: 5, MaxIdle: 1, MaxIdleTime: 5 * time.Minute, MaxLifetime: 30 * time.Minute}
	c := Config{HTTPAddr: ":8080", LogLevel: "info", ShutdownTimeout: 10 * time.Second, CardDB: db, AppDB: db}
	if path != "" {
		if err := applyFile(&c, path); err != nil {
			return Config{}, err
		}
	}
	if err := applyEnvironment(&c); err != nil {
		return Config{}, err
	}
	if err := validateDB(c.CardDB, "card database"); err != nil {
		return Config{}, err
	}
	if err := validateDB(c.AppDB, "app database"); err != nil {
		return Config{}, err
	}
	if c.CardDB.Name == "" || c.AppDB.Name == "" {
		return Config{}, fmt.Errorf("card and app database names are required")
	}
	if c.CardDB.Name == c.AppDB.Name {
		return Config{}, fmt.Errorf("card and app database names must differ")
	}
	if c.RebuildEnabled && c.CardDataDir == "" {
		return Config{}, fmt.Errorf("card_data.dir is required when rebuild is enabled")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("log.level must be debug, info, warn, or error")
	}
	return c, nil
}

func validateDB(db DB, name string) error {
	if db.Port <= 0 || db.MaxOpen <= 0 || db.MaxIdle < 0 {
		return fmt.Errorf("%s contains an invalid port or pool value", name)
	}
	return nil
}

func applyFile(c *Config, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config file %q: %w", path, err)
	}
	defer f.Close()
	var raw fileConfig
	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil {
		return fmt.Errorf("decode config file %q: %w", path, err)
	}
	if raw.HTTP.Addr != "" {
		c.HTTPAddr = raw.HTTP.Addr
	}
	if raw.Log.Level != "" {
		c.LogLevel = raw.Log.Level
	}
	if raw.CardData.Dir != "" {
		c.CardDataDir = raw.CardData.Dir
	}
	if raw.CardData.RebuildEnabled != nil {
		c.RebuildEnabled = *raw.CardData.RebuildEnabled
	}
	if raw.HTTP.ShutdownTimeout != "" {
		v, err := positiveDuration("http.shutdown_timeout", raw.HTTP.ShutdownTimeout)
		if err != nil {
			return err
		}
		c.ShutdownTimeout = v
	}
	if err := applyFileDB(&c.CardDB, raw.Databases.Card, "databases.card"); err != nil {
		return err
	}
	return applyFileDB(&c.AppDB, raw.Databases.App, "databases.app")
}

func applyFileDB(db *DB, raw fileDB, prefix string) error {
	if raw.Host != "" {
		db.Host = raw.Host
	}
	if raw.Port != 0 {
		db.Port = raw.Port
	}
	if raw.Name != "" {
		db.Name = raw.Name
	}
	if raw.User != "" {
		db.User = raw.User
	}
	if raw.Password != "" {
		db.Password = raw.Password
	}
	if raw.MaxOpen != 0 {
		db.MaxOpen = raw.MaxOpen
	}
	if raw.MaxIdle != 0 {
		db.MaxIdle = raw.MaxIdle
	}
	if raw.MaxIdleTime != "" {
		v, err := positiveDuration(prefix+".conn_max_idle_time", raw.MaxIdleTime)
		if err != nil {
			return err
		}
		db.MaxIdleTime = v
	}
	if raw.MaxLifetime != "" {
		v, err := positiveDuration(prefix+".conn_max_lifetime", raw.MaxLifetime)
		if err != nil {
			return err
		}
		db.MaxLifetime = v
	}
	if err := validateDB(*db, prefix); err != nil {
		return err
	}
	return nil
}

func applyEnvironment(c *Config) error {
	setString("HTTP_ADDR", &c.HTTPAddr)
	setString("LOG_LEVEL", &c.LogLevel)
	setString("CARD_DATA_DIR", &c.CardDataDir)
	if err := setDuration("SHUTDOWN_TIMEOUT", &c.ShutdownTimeout); err != nil {
		return err
	}
	if err := setBool("CARD_DATA_REBUILD_ENABLED", &c.RebuildEnabled); err != nil {
		return err
	}
	if err := applyDBEnvironment("CARD_DB", &c.CardDB); err != nil {
		return err
	}
	return applyDBEnvironment("APP_DB", &c.AppDB)
}

func applyDBEnvironment(prefix string, db *DB) error {
	setString(prefix+"_HOST", &db.Host)
	setString(prefix+"_NAME", &db.Name)
	setString(prefix+"_USER", &db.User)
	setString(prefix+"_PASSWORD", &db.Password)
	for _, item := range []struct {
		key    string
		target *int
	}{{prefix + "_PORT", &db.Port}, {prefix + "_MAX_OPEN_CONNS", &db.MaxOpen}, {prefix + "_MAX_IDLE_CONNS", &db.MaxIdle}} {
		if err := setInt(item.key, item.target); err != nil {
			return err
		}
	}
	if err := setDuration(prefix+"_CONN_MAX_IDLE_TIME", &db.MaxIdleTime); err != nil {
		return err
	}
	return setDuration(prefix+"_CONN_MAX_LIFETIME", &db.MaxLifetime)
}

func setString(key string, target *string) {
	if value, ok := os.LookupEnv(key); ok {
		*target = value
	}
}
func setInt(key string, target *int) error {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fmt.Errorf("%s must be a non-negative integer", key)
	}
	*target = parsed
	return nil
}
func setDuration(key string, target *time.Duration) error {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	parsed, err := positiveDuration(key, value)
	if err != nil {
		return err
	}
	*target = parsed
	return nil
}
func setBool(key string, target *bool) error {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("%s must be a boolean", key)
	}
	*target = parsed
	return nil
}
func positiveDuration(key, value string) (time.Duration, error) {
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return parsed, nil
}
