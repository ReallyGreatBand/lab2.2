package integration

import (
	"encoding/json"
	"fmt"
	gocheck "gopkg.in/check.v1"
	"log"
	"net/http"
	"testing"
	"time"
)

func TestBalancer(t *testing.T) { gocheck.TestingT(t) }

type MySuiteBalancer struct{}

var _ = gocheck.Suite(&MySuiteBalancer{})


const baseAddress = "http://balancer:8090"

var client = http.Client{
	Timeout: 3 * time.Second,
}

type Msg struct {
	Key string
	Value string
}

func (s *MySuiteBalancer) TestBalancer(c *gocheck.C) {
	// TODO: Реалізуйте інтеграційний тест для балансувальникка.
	serverPool := []string{
		"server1:8080",
		"server2:8080",
		"server3:8080",
	}
	authors := make(chan [2]string, 10)
	for i := 0; i < 10; i++ {
		go func() {
			resp, err := client.Get(fmt.Sprintf("%s/api/v1/some-data?key=reallygreatband", baseAddress)) // Сервер занадто швидко обробляє запити, тест можливий тільки якщо є довга операція обробки запиту серверами(наприклад sleep)

			if err != nil {
				c.Error(err)
			}
			var val Msg
			err = json.NewDecoder(resp.Body).Decode(&val)
			if err != nil {
				log.Printf("%s", err)
			}
			respServer := resp.Header.Get("Lb-from")
			authors <- [2]string{respServer, val.Value}
		}()
		time.Sleep(time.Duration(20) * time.Millisecond)
	}
	for i := 0; i < 10; i++ {
		auth := <- authors
		log.Printf("%v", auth[1])
		log.Printf("%v", i)
		c.Assert(auth[0], gocheck.Equals, serverPool[i%3])
		c.Assert(auth[1], gocheck.Equals, "2010-05-29")
	}
}

func (s *MySuiteBalancer) BenchmarkBalancer(c *gocheck.C) {
	for i := 0; i < c.N; i++ {
		client.Get(fmt.Sprintf("%s/api/v1/some-data?key=reallygreatband", baseAddress))
	}
	// TODO: Реалізуйте інтеграційний бенчмарк для балансувальникка.
}
