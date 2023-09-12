package loader

//func TestLoadConf(t *testing.T) {
//o := NewLoader()
//o.SetConfigFileSearcher("config", "../../")
////读取优先级
//调用Set显式设置的；
//命令行选项；
//环境变量；
//配置文件；
//默认值。
//o.EnableEnvSearcher("test1")
//
//o.LoadToStruct(&Config)
//t.Logf("%v", Config)

//LoadConfig(&Config, func(l Loader) {
//	l.SetConfigFileSearcher("config", "../../").EnableEnvSearcher("test")
//})
//
//t.Logf("%v", Config)
//}

//func LoadConfig(config interface{}, callback func(Loader)) (err error) {
//	var cb = callback
//	loader := NewLoader()
//	if cb == nil {
//		cb = func(ld Loader) {
//			ld.SetConfigFileSearcher("config", ".").EnableEnvSearcher("")
//		}
//	}
//	cb(loader)
//	err = loader.LoadToStruct(config)
//	return
//}

//func TestSlice(t *testing.T) {
//	slice1 := make([]int, 0)
//	t.Logf("%p", slice1)
//	t.Logf("%p", &slice1)
//	testSlice(&slice1)
//	t.Log(slice1)
//}
//
//func testSlice(slice *[]int) {
//	fmt.Printf("%p\n", slice)
//	fmt.Printf("%p\n", *slice)
//	*slice = append(*slice, 1, 2, 3, 4)
//}

//func TestSlice2(t *testing.T) {
//	slice1 := make([]int, 5, 5)
//	t.Log(slice1)
//	fmt.Printf("st 原始切片的地址为：%p\n", &slice1)
//	testSlice2(&slice1)
//	t.Log(slice1)
//}
//
//func testSlice2(slice *[]int) {
//	fmt.Printf("st 原始切片的地址为：%p\n", slice)
//	*slice = append(*slice, 1)
//}
