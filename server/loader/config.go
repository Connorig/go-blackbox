package loader

var Config Configuration

type Configuration struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`

	Web     web     `toml:"web"`
	Db      db      `toml:"db"`
	Redis   redis   `toml:"redis"`
	LogConf logConf `toml:"logConf"`
}

type web struct {
	Listen     string `toml:"listen"`
	DebugLevel string `toml:"debugLevel"`
}

type db struct {
	User     string `toml:"user"`
	Password string `toml:"password"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	DbName   string `toml:"dbName"`
	Ssl      string `toml:"ssl"`

	MaxIdleConns int `toml:"maxIdleConns"`
	MaxOpenConns int `toml:"maxOpenConns"`
}

type redis struct {
	Adders   string `toml:"addrs"`
	Password string `toml:"password"`
	PoolSize int    `toml:"poolSize"`
	Db       int    `toml:"db"`
}

type logConf struct {
	OutDirPath string `toml:"outDirPath"`
	LogLevel   string `toml:"logLevel"`
}
