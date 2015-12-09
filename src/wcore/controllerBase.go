package wcore

//Controller will be used to store permanent datas
type Controller struct {
	ControllerName string //the name of the controller
	TplName        string //filename of the template. view/{ControllerName}/{TplName}
	Layout         string //filename of the layout file. view/layout/{Layout}
	//session will be a wrapper for gorilla session
	methodsMap map[string]func() //methods used by the router
}

func (c *Controller) Init() {

}
func (c *Controller) BeforeRun(ctx *Context) bool {
	return true
}
func (c *Controller) Name() string {
	return c.ControllerName
}

type ControllerInterface interface {
	Init()                       //Run at the start of the program only
	BeforeRun(ctx *Context) bool //Run before each request
	AfterRun(ctx *Context)       //Run after each request. At this point, result is already be returned (good for logging...)
	Name() string
}
