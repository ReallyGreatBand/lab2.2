package integration

import (
	"fmt"
	gocheck "gopkg.in/check.v1"
	"io/ioutil"
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
			req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/some-data?key=reallygreatband", baseAddress), nil)
			resp, err := client.Do(req) // Сервер занадто швидко обробляє запити, тест можливий тільки якщо є довга операція обробки запиту серверами(наприклад sleep)

			if err != nil {
				c.Error(err)
			}
			val, _ := ioutil.ReadAll(resp.Body)
			respServer := resp.Header.Get("Lb-from")
			authors <- [2]string{respServer, string(val)}
		}()
		time.Sleep(time.Duration(20) * time.Millisecond)
	}
	for i := 0; i < 10; i++ {
		auth := <- authors
		c.Assert(auth[0], gocheck.Equals, serverPool[i%3])
		c.Assert(auth[0], gocheck.Equals, "29050-41-299")
	}
}

func (s *MySuiteBalancer) BenchmarkBalancer(c *gocheck.C) {
	for i := 0; i < c.N; i++ {
		client.Get(fmt.Sprintf("%s/api/v1/some-data?key=reallygreatband", baseAddress))
	}
	// TODO: Реалізуйте інтеграційний бенчмарк для балансувальникка.
}
