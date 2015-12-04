package wcore

/**
Copyrights All rights reserved Gabriel Poulenard-Talbot
*/

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/context"
	"github.com/gorilla/handlers"
	"github.com/julienschmidt/httprouter"
	"gopkg.in/mgo.v2"
)

var (
	intReg      = regexp.MustCompile("[0-9]+")
	stringReg   = regexp.MustCompile("[^\\/]+")
	ctlBaseType = reflect.TypeOf(Controller{})
)

//tcpKeepAliveListener copyed from Golang net/http lib to allow ln.Close()
type tcpKeepAliveListener struct {
	*net.TCPListener
	cancelled bool
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	if ln.cancelled {
		return nil, fmt.Errorf("Use of cancelled connexion")
	}
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
func (ln tcpKeepAliveListener) Stop() error {
	if !ln.cancelled {
		ln.cancelled = true
		return ln.Close()
	}
	return nil
}

//tcpKeepAliveListener END

//WCore
type WCore struct {
	done     chan bool
	closed   bool
	router   *httprouter.Router
	ctls     []ControllerInterface //Controllers
	db       *mgo.Session
	services []WService
	wg       sync.WaitGroup
	mtx      sync.Mutex
}
type WService interface {
	Start() //will be called in a new Goroutine
	Stop()
}

type httpService struct {
	done           chan bool
	tcpln          tcpKeepAliveListener
	serv           http.Server
	connexionCount uint32
}

func New() (*WCore, error) {
	var err error
	wc := new(WCore)
	wc.db, err = mgo.Dial("localhost")
	if err != nil {
		return nil, fmt.Errorf("MongoDB error: %s", err.Error())
	}
	wc.db.SetMode(mgo.Monotonic, true)

	wc.done = make(chan bool, 1)
	wc.closed = false
	wc.router = httprouter.New()
	wc.ctls = []ControllerInterface{}
	wc.services = []WService{}

	return wc, nil
}
func (w *WCore) Close() {
	defer w.db.Close()
	func() {
		defer w.mtx.Unlock()
		w.mtx.Lock()
		if !w.closed {
			w.closed = true
			for _, servs := range w.services {
				servs.Stop()
			}
		}
	}()
	w.Wait()

}
func (w *WCore) Wait() {
	w.wg.Wait()
}

func (w *WCore) DB(name string) *mgo.Database {
	return w.db.DB(name)
}

func (w *WCore) AddController(ctl ControllerInterface) {
	defer w.mtx.Unlock()
	w.mtx.Lock()

	w.ctls = append(w.ctls, ctl)
}

func (w *WCore) Serve(path string) error {
	srv := new(httpService)
	srv.connexionCount = 0
	srv.done = make(chan bool, 1)
	srv.serv = http.Server{Addr: path, Handler: context.ClearHandler(srv.connexionCountHandler(handlers.ProxyHeaders(w.router)))}
	if srv.serv.Addr == "" {
		srv.serv.Addr = ":http"
	}
	ln, err := net.Listen("tcp", srv.serv.Addr)
	if err != nil {
		return err
	}

	srv.tcpln = tcpKeepAliveListener{ln.(*net.TCPListener), false}

	w.RunService(srv)

	return nil
}

//used to prevent memory leak from Goroutines or "dead" Goroutines
func (srv *httpService) connexionCountHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer atomic.AddUint32(&srv.connexionCount, ^uint32(0))
		atomic.AddUint32(&srv.connexionCount, 1)

		h.ServeHTTP(w, r)
	})
}

func (w *WCore) RunService(srv WService) {
	defer w.mtx.Unlock()
	w.mtx.Lock()

	w.services = append(w.services, srv)

	w.wg.Add(1) //Add() need to be called before Wait()
	go func(wc *WCore) {
		defer wc.wg.Done()

		srv.Start()
	}(w)
	runtime.Gosched() //Try to garanteed that service is running on return
}

func (srv *httpService) Start() {
	//handle the signal
	go func() {
		select {
		case <-srv.done:
			srv.tcpln.Stop()
		}
	}()

	err := srv.serv.Serve(srv.tcpln)

	if err != nil && !strings.HasSuffix(err.Error(), "use of closed network connection") {
		log.Println(err)
	}
}

func (srv *httpService) Stop() {
	//will probably panic if called multiple times
	srv.done <- true
	close(srv.done)

	startTime := time.Now()
	for atomic.LoadUint32(&srv.connexionCount) > 0 {
		runtime.Gosched()
		if time.Since(startTime).Minutes() > 3 {
			break //after 3 minutes don't wait
		}
	}
}

/*func Run(path string) error {
	sigs := make(chan os.Signal)
	done := make(chan bool)

	r := httprouter.New()
	r.GET("/", controllerRoute())
	r.GET("/:controller", controllerRoute())
	r.GET("/:controller/:action", controllerRoute())
	r.GET("/:controller/:action/*params", controllerRoute())

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	srv := http.Server{Addr: path, Handler: context.ClearHandler(connexionCountHandler(handlers.ProxyHeaders(r)))}
	if srv.Addr == "" {
		srv.Addr = ":http"
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}

	tcpln := tcpKeepAliveListener{ln.(*net.TCPListener), false}
	//handle the signal
	go func() {
		select {
		case <-sigs:
			tcpln.Stop()
		case <-done:
			tcpln.Stop()
		}
	}()

	err = srv.Serve(tcpln)

	startTime := time.Now()
	for atomic.LoadUint32(&connexionCount) > 0 {
		time.Sleep(time.Millisecond * 100)
		if time.Since(startTime).Minutes() > 3 {
			break //after 3 minutes don't wait
		}
	}

	select {
	case done <- true:
		return err
	default:
		return nil //if can't write to chan, signal got received so we created the error
	}
}*/

func controllerRoute() httprouter.Handle {
	return httprouter.Handle(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if len(r.URL.Path) > 1 && r.URL.Path[len(r.URL.Path)-1] == '/' {
			redirectPath := r.URL.Path
			for len(redirectPath) > 1 && redirectPath[len(redirectPath)-1] == '/' {
				redirectPath = redirectPath[:len(redirectPath)-1]
			}
			http.Redirect(w, r, redirectPath, http.StatusFound)
			return
		}

		defer handlePanic(w)

		ctx := NewContext(w, r)

		/*if p.ByName("controller") == "match" {
			action := p.ByName("action")
			if action != "" && intReg.FindString(action) == action {
				sessStr = "Int"
			} else {
				sessStr = "String"
			}
		}*/

		w.Header().Set("Content-Type", ctx.ContentType)

	})
}
func handlePanic(w http.ResponseWriter) {
	if re := recover(); re != nil {
		//panic happened, return 500
		var errorStr string

		if err, ok := re.(error); ok {
			errorStr = fmt.Sprintf("Internal error: %v", err)
		} else {
			errorStr = "Internal error"
		}

		http.Error(w, errorStr, http.StatusInternalServerError)
	}
}
