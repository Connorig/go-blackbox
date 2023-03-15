package web

var CONFIG = Web{
	FileMaxSize:    1024, // upload file size limit 1024M
	SessionTimeout: 60,   // session timeout after 60 Minute
	Except: Route{
		Uri:    "",
		Method: "",
	},
	System: System{
		Tls:        false,
		Level:      "debug",
		Addr:       "127.0.0.1:8085",
		DbType:     "mysql",
		TimeFormat: "2006-01-02 15:04:05",
	},
	Limit: Limit{
		Disable: true,
		Limit:   0,
		Burst:   5,
	},
	Captcha: Captcha{
		KeyLong:   4,
		ImgWidth:  240,
		ImgHeight: 80,
	},
}

type Web struct {
	FileMaxSize    int64   `mapstructure:"file-max-size" json:"file-max-siz" yaml:"file-max-siz"`
	SessionTimeout int64   `mapstructure:"session-timeout" json:"session-timeout" yaml:"session-timeout"`
	Except         Route   `mapstructure:"except" json:"except" yaml:"except"`
	System         System  `mapstructure:"system" json:"system" yaml:"system"`
	Limit          Limit   `mapstructure:"limit" json:"limit" yaml:"limit"`
	Captcha        Captcha `mapstructure:"captcha" json:"captcha" yaml:"captcha"`
}
type Route struct {
	Uri    string `mapstructure:"uri" json:"uri" yaml:"uri"`
	Method string `mapstructure:"method" json:"method" yaml:"method"`
}

type Captcha struct {
	KeyLong   int `mapstructure:"key-long" json:"key-long" yaml:"key-long"`
	ImgWidth  int `mapstructure:"img-width" json:"img-width" yaml:"img-width"`
	ImgHeight int `mapstructure:"img-height" json:"img-height" yaml:"img-height"`
}

type Limit struct {
	Disable bool    `mapstructure:"disable" json:"disable" yaml:"disable"`
	Limit   float64 `mapstructure:"limit" json:"limit" yaml:"limit"`
	Burst   int     `mapstructure:"burst" json:"burst" yaml:"burst"`
}

type System struct {
	Tls          bool   `mapstructure:"tls" json:"tls" yaml:"tls"`       // debug,release,test
	Level        string `mapstructure:"level" json:"level" yaml:"level"` // debug,release,test
	Addr         string `mapstructure:"addr" json:"addr" yaml:"addr"`
	StaticPrefix string `mapstructure:"static-prefix" json:"static-prefix" yaml:"static-prefix"`
	WebPrefix    string `mapstructure:"web-prefix" json:"web-prefix" yaml:"web-prefix"`
	DbType       string `mapstructure:"db-type" json:"db-type" yaml:"db-type"`
	TimeFormat   string `mapstructure:"time-format" json:"time-format" yaml:"time-format"`
}
