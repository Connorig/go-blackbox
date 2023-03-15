package web_iris

import (
	"awesomeProject1/server/web"
	"github.com/kataras/iris/v12"
	"github.com/snowlyg/helper/arr"
	"net/http"
	"strings"
)

// InitRouter
func (ws *WebServer) InitRouter() error {
	app := ws.app.Party("/").AllowMethods(iris.MethodOptions)

	for _, party := range ws.parties {
		app.PartyFunc(party.Perfix, party.PartyFunc)
	}

	// http test must build
	if err := ws.app.Build(); err != nil {
		return err
	}

	return nil
}

// GetSources
// - PermRoutes
// - NoPermRoutes
func (ws *WebServer) GetSources() ([]map[string]string, []map[string]string) {
	methodExcepts := strings.Split(web.CONFIG.Except.Method, ";")
	uris := strings.Split(web.CONFIG.Except.Uri, ";")
	routeLen := len(ws.app.GetRoutes())
	permRoutes := make([]map[string]string, 0, routeLen)
	noPermRoutes := make([]map[string]string, 0, routeLen)

	for _, r := range ws.app.GetRoutes() {
		route := map[string]string{
			"path": r.Path,
			"name": r.Name,
			"act":  r.Method,
		}
		httpStatusType := arr.NewCheckArrayType(4)
		httpStatusType.AddMutil(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete)
		if !httpStatusType.Check(r.Method) {
			noPermRoutes = append(noPermRoutes, route)
			continue
		}

		if len(methodExcepts) > 0 && len(uris) > 0 && len(methodExcepts) == len(uris) {
			for i := 0; i < len(methodExcepts); i++ {
				if strings.EqualFold(r.Method, strings.ToLower(methodExcepts[i])) && strings.EqualFold(r.Path, strings.ToLower(uris[i])) {
					noPermRoutes = append(noPermRoutes, route)
					continue
				}
			}
		}

		permRoutes = append(permRoutes, route)
	}
	return permRoutes, noPermRoutes
}
