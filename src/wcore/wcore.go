package wcore

/**
Copyrights All rights reserved Gabriel Poulenard-Talbot
*/

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/context"
	"github.com/gorilla/handlers"
	"github.com/gorilla/sessions"
	"github.com/julienschmidt/httprouter"
)

var (
	IntReg    = regexp.MustCompile("[0-9]+")
	StringReg = regexp.MustCompile("[^\\/]+")
	//	ctlBaseType = reflect.TypeOf(Controller{})
	CookieStore = sessions.NewCookieStore([]byte("secret-code"))
)

type ContextKey uint8

const (
	Language ContextKey = iota
	ContentType
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
	services []WService
	DB       *sql.DB
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

func New(dbinfo string) (*WCore, error) {
	var err error
	wc := new(WCore)

	wc.DB, err = sql.Open("mysql", dbinfo)
	if err != nil {
		return nil, fmt.Errorf("DB error: %s", err.Error())
	}
	err = wc.DB.Ping()
	if err != nil {
		//TODO re-enable
		/*	wc.DB.Close()
			return nil, fmt.Errorf("DB error: %s", err.Error())*/
	}

	wc.done = make(chan bool, 1)
	wc.closed = false
	wc.router = httprouter.New()
	wc.ctls = []ControllerInterface{}
	wc.services = []WService{}

	return wc, nil
}
func (w *WCore) Close() {
	defer w.DB.Close()

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
	defer w.mtx.Unlock()
	w.mtx.Lock()

	for _, ctl := range w.ctls {
		ctl.Destroy()
	}

}
func (w *WCore) Wait() {
	w.wg.Wait()
}

func (w *WCore) AddController(ctl ControllerInterface) {
	defer w.mtx.Unlock()
	w.mtx.Lock()

	ctl.Init(w.router) //adding paths to the router

	w.ctls = append(w.ctls, ctl)
}

func (w *WCore) Serve(path string) error {
	srv := new(httpService)
	srv.connexionCount = 0
	srv.done = make(chan bool, 1)
	srv.serv = http.Server{Addr: path, Handler: context.ClearHandler(handlers.ProxyHeaders(srv.connexionCountHandler(w)))}
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

func (wc *WCore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer wc.handlePanic(w)
	//https://godoc.org/github.com/nicksnyder/go-i18n/i18n
	if len(r.URL.Path) >= 3 {
		if strings.ToLower(r.URL.Path[1:3]) == "en" {
			context.Set(r, Language, "en-ca")
		} else {
			context.Set(r, Language, "fr-fr")
		}
		r.URL.Path = r.URL.Path[3:]
		if len(r.URL.Path) == 0 || r.URL.Path[0] != '/' {
			r.URL.Path = "/" + r.URL.Path
		}
	} //else doesn't set it, will use default value

	//Set default context values
	context.Set(r, ContentType, "text/html")

	bw := NewBufferedWriter(w)
	defer bw.Flush()

	wc.router.ServeHTTP(bw, r)

	if ct, ok := context.GetOk(r, ContentType); ok {
		bw.Header().Set("Content-Type", ct.(string))
	}
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

//used to prevent memory leak from Goroutines or "dead" Goroutines
func (srv *httpService) connexionCountHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer atomic.AddUint32(&srv.connexionCount, ^uint32(0))
		atomic.AddUint32(&srv.connexionCount, 1)

		h.ServeHTTP(w, r)
	})
}

func (wc *WCore) handlePanic(w http.ResponseWriter) {
	//that way program won't crash, but it's better to use httprouter.PanicHandler for custom page
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
