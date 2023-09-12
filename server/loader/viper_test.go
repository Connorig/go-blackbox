package loader

//func TestViper(t *testing.T) {
//
//	v := viper.New()
//
//	v.SetConfigName("config") // 文件名称，无需指定后缀类型
//	//v.SetConfigType("toml") // 文件类型
//	v.AddConfigPath("../../") // 文件位置，多个path匹配
//
//	if err := v.ReadInConfig(); err != nil {
//		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
//			// Config file not found; ignore error if desired
//		} else {
//			// Config file was found but another error was produced
//		}
//	}
//
//	t.Logf("%s", v.Get("web.listen"))
//	v.Unmarshal(&Config)
//	t.Logf("%v", Config)
//}
//
//func TestENV(t *testing.T) {
//	// 从环境变量中读取并解析到 struct
//	v := viper.New()
//	v.SetEnvPrefix("test")
//
//	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
//
//	// 读取环境变量
//	v.AutomaticEnv()
//
//	// 将 配置文件结构体转换 map形式
//	confMap := structs.Map(Config)
//	// 将map 扁平化解析，映射键转换为复合键 名称，
//	flat, err := flatten.Flatten(confMap, "", flatten.DotStyle) // 默认树级属性 Father.Children
//	// Name
//	// Version
//	// Web.Listen
//	// Db.user
//	// Db.Password
//	if err != nil {
//		t.Logf("%s", err)
//	}
//
//	for key := range flat {
//		// 通过解析的flat，将对应的Key绑定到对应的环境变量中。
//		err = v.BindEnv(key)
//		// Name ==> 环境变量= TEST_NAME , TEST是前缀
//		// Version
//		// Web.Listen ==> TEST_WEB_LISTEN
//		// Db.user    ==> TEST_DB_USER
//		// Db.Password  ==> TEST_DB_PASSWORD
//		if err != nil {
//			t.Logf("%s", err)
//
//		}
//	}
//
//	v.Unmarshal(&Config)
//
//	t.Logf("%v", Config)
//}
