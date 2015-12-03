package wcore

//Controller will be used to store permanent datas
type Controller struct {
	ControllerName string //the name of the controller
	TplName        string //filename of the template. view/{ControllerName}/{TplName}
	Layout         string //filename of the layout file. view/layout/{Layout}
	//session will be a wrapper for gorilla session
	methodsMap map[string]func() //methods used by the router
}

func (c *Controller) Name() string {
	return c.ControllerName
}

type ControllerInterface interface {
	/*	Init()
		beforeRun()*/
	Name() string
}
