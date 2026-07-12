package apploader

var Config Configuration

type Configuration struct {
	Name string `toml:"name"`

	Version string `toml:"version"`

	Web     web     `toml:"web"`
	Db      db      `toml:"db"`
	Redis   redis   `toml:"redis"`
	LogConf logConf `toml:"logConf"`
}

type web struct {
	Listen string `toml:"listen"`
	Level  string `toml:"level"`
}

type db struct {
	User         string `toml:"user"`
	Password     string `toml:"password"`
	Host         string `toml:"host"`
	Port         int    `toml:"port"`
	DbName       string `toml:"dbName"`
	Ssl          string `toml:"ssl"`
	MaxIdleCones int    `toml:"maxIdleCones"`
	MaxOpenCones int    `toml:"maxOpenCones"`
}

type redis struct {
	Host     string `toml:"host"`
	Password string `toml:"password"`
	PoolSize int    `toml:"poolSize"`
	Db       int    `toml:"db"`
}

type logConf struct {
	OutDirPath string `toml:"outDirPath"`
	LogLevel   string `toml:"logLevel"`
}
