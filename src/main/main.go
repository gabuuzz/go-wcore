package main

import (
	"herbw/controllers"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
	"wcore"

	"github.com/nicksnyder/go-i18n/i18n"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	done := make(chan bool, 1)
	sigs := make(chan os.Signal)
	signal.Notify(sigs, os.Interrupt, os.Kill, syscall.SIGTERM)

	i18n.MustLoadTranslationFile("translation/fr-fr.json")
	i18n.MustLoadTranslationFile("translation/en-ca.json")

	func() {
		wc, err := wcore.New("user:password@/dbname?parseTime=true&timeout=2m")
		if err != nil {
			log.Fatalln(err)
		}
		defer wc.Close()

		go func(w *wcore.WCore) {
			select {
			case <-sigs:
				w.Close()
			case <-done:
				w.Close()
			}
		}(wc)

		wc.AddController(&controllers.ErrorsController{})
		wc.AddController(&controllers.TestController{})

		wc.Serve(":8080")

		wc.Wait()
		done <- true
		close(done)

		runtime.Gosched()
		time.Sleep(time.Millisecond * 100)
	}()

	close(sigs)
}
