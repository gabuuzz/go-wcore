package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
	"wcore"

	"gopkg.in/mgo.v2/bson"
)

type testing struct {
	wcore.Controller
	ID  bson.ObjectId `bson:"_id,omitempty"`
	Val string
}

func (t *testing) Start() {
	log.Println("test")
	time.Sleep(time.Second * 10)
}
func (t *testing) Stop() {
	log.Println("stop")
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	done := make(chan bool, 1)
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	func() {
		wc, err := wcore.New()
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

		test := new(testing)
		test.Val = "testing val"

		wc.AddController(test)

		wc.Serve(":8080")
		wc.RunService(test)

		/*wc.DB("test").DropDatabase()

		c := wc.DB("test").C("testing")

		start := time.Now()

		var wg sync.WaitGroup
		wg.Add(1000)

		for i := 0; i < 1000; i++ {
			go func(c *mgo.Collection, count int) {
				e := c.Insert(&testing{Val: "test number: " + fmt.Sprint(count)})
				if e != nil {
					log.Println("Error:", e)
				}

				wg.Done()
			}(c, i)
		}
		wg.Wait()

		log.Println("Insert took:", time.Since(start))

		start = time.Now()
		var t []testing
		err = c.Find(nil).All(&t)
		if err != nil {
			panic(err)
		}
		log.Println("Find took:", time.Since(start))

		log.Println("Value:", t[0].Val)*/

		wc.Wait()
		done <- true
		close(done)

		runtime.Gosched()
		time.Sleep(time.Millisecond * 100)
	}()
}
