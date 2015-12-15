package wcore

import (
	"net/http"

	"github.com/gorilla/context"
	"github.com/julienschmidt/httprouter"
	"github.com/nicksnyder/go-i18n/i18n"
)

//Controller will be used to store permanent datas
type Controller struct {
	ControllerName string //the name of the controller
	TplName        string //filename of the template. view/{ControllerName}/{TplName}
	Layout         string //filename of the layout file. view/layout/{Layout}
	//session will be a wrapper for gorilla session
	methodsMap map[string]func() //methods used by the router
}

func (c *Controller) Init(r *httprouter.Router) {
}

func (c *Controller) Destroy() {
}

func (c *Controller) Name() string {
	return c.ControllerName
}

func (c *Controller) Tfunc(r *http.Request) i18n.TranslateFunc {
	var trlang string
	trv := context.Get(r, Language)
	if trv != nil {
		trlang = trv.(string)
	}
	acceptLang := r.Header.Get("Accept-Language")
	T, err := i18n.Tfunc(trlang, acceptLang, "fr-fr")
	if err != nil {
		panic(err)
	}

	return T
}

type ControllerInterface interface {
	Init(r *httprouter.Router) //called on start of server only
	Destroy()
	Name() string
}
