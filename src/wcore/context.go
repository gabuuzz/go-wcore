package wcore

import (
	"net/http"

	"github.com/gorilla/sessions"
)

var store = sessions.NewCookieStore([]byte("%6[u<fQ{Zae~4ANAc\\E:KB_P{g'2}8.WbJhbpA8x-8J~;t"))

//Context will be used to store request datas
type Context struct {
	Data        map[string]interface{} //template datas
	ContentType string                 //text/html by default
	req         *http.Request
	writer      http.ResponseWriter
	cancel      bool //if true, stop the rendering process
}

func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		Data:        make(map[string]interface{}),
		ContentType: "text/html",
		req:         r,
		writer:      w,
		cancel:      false,
	}
}

func (c *Context) Session(name string) (*sessions.Session, error) {
	return store.Get(c.req, name)
}

//save all
func (c *Context) SaveSession() error {
	return sessions.Save(c.req, c.writer)
}
