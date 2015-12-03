package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"wcore"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type testing struct {
	wcore.Controller
	ID  bson.ObjectId `bson:"_id,omitempty"`
	Val string
}

func (t *testing) Start() {
	fmt.Println("test")
	time.Sleep(time.Second * 10)
}
func (t *testing) Stop() {
	fmt.Println("stop")
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	/*if err := wcore.Run(":8080"); err != nil {
		log.Fatal(err)
	}*/
	func() {
		wc, err := wcore.New()
		defer wc.Close()

		test := new(testing)
		test.Val = "testing val"

		wc.RunService(test)
		go func(w *wcore.WCore) {
			time.Sleep(time.Second * 5)
			w.Close()
		}(wc)

		wc.DB("test").DropDatabase()

		c := wc.DB("test").C("testing")

		start := time.Now()

		var wg sync.WaitGroup
		wg.Add(10000)

		for i := 0; i < 10000; i++ {
			go func(c *mgo.Collection) {
				err = c.Insert(&testing{Val: "test number: " + fmt.Sprint(i)})
				if err != nil {
					fmt.Println("Error:", err)
				}

				wg.Done()
			}(c)
		}
		wg.Wait()

		fmt.Println("Insert took:", time.Since(start))

		start = time.Now()
		var t []testing
		err = c.Find(nil).All(&t)
		if err != nil {
			panic(err)
		}
		fmt.Println("Find took:", time.Since(start))

		fmt.Println("Value:", t[0].Val)

		wc.Wait()
	}()

}
