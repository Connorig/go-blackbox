package web

// init
func init() {
}

type WebBaseFunc interface {
	AddWebStatic(staticAbsPath, webPrefix string, paths ...string)
	AddUploadStatic(staticAbsPath, webPrefix string)
	InitRouter() error
	Run()
}

type WebFunc interface {
	WebBaseFunc
}

// Start
func Start(wf WebFunc) {
	err := wf.InitRouter()
	if err != nil {
		return
	}
	wf.Run()
}
