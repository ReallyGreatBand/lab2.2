package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ReallyGreatBand/lab2.2/httptools"
	"github.com/ReallyGreatBand/lab2.2/signal"
)

var (
	port = flag.Int("port", 8090, "load balancer port")
	timeoutSec = flag.Int("timeout-sec", 3, "request timeout time in seconds")
	https = flag.Bool("https", false, "whether backends support HTTPs")

	traceEnabled = flag.Bool("trace", false, "whether to include tracing information into responses")
)

type server struct {
	host string
	counter int
	status bool
}

type leastConnections struct {
	servers []*server
	mutex *sync.Mutex
}

func (lc *leastConnections) getLeastConnected() (*server, func(), error) {
	var (
		min = -1
		index = 0
	)
	lc.mutex.Lock()
	for i, server := range lc.servers {
		if (min == -1 || server.counter < min) && server.status {
			min = server.counter
			index = i
		}
	}
	if !lc.servers[index].status {
		lc.mutex.Unlock()
		return lc.servers[index], func() {
			lc.servers[index].counter--
		}, fmt.Errorf("no servers online")
	}

	lc.servers[index].counter++
	lc.mutex.Unlock()
	return lc.servers[index], func() {
		lc.servers[index].counter--
	}, nil
}

func Initialize(hosts []string) (*leastConnections, error) {
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no servers available")
	}

	servers := make([]*server, len(hosts))
	for index := range servers {
		servers[index] = &server{
			host: hosts[index],
			counter: 0,
			status: true,
		}
	}

	return &leastConnections{
		servers,
		new(sync.Mutex),
	}, nil
}

var (
	timeout        = time.Duration(*timeoutSec) * time.Second
	serversPool, _ = Initialize([]string{
		"server1:8080",
		"server2:8080",
		"server3:8080",
	})
)

func scheme() string {
	if *https {
		return "https"
	}
	return "http"
}

func health(dst string) bool {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s://%s/health", scheme(), dst), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	if resp.StatusCode != http.StatusOK {
		return false
	}
	return true
}

func forward(dst string, rw http.ResponseWriter, r *http.Request) {
	ctx, _ := context.WithTimeout(r.Context(), timeout)
	fwdRequest := r.Clone(ctx)
	fwdRequest.RequestURI = ""
	fwdRequest.URL.Host = dst
	fwdRequest.URL.Scheme = scheme()
	fwdRequest.Host = dst

	resp, err := http.DefaultClient.Do(fwdRequest)
	if err == nil {
		for k, values := range resp.Header {
			for _, value := range values {
				rw.Header().Add(k, value)
			}
		}
		if *traceEnabled {
			rw.Header().Set("lb-from", dst)
		}
		log.Println("fwd", resp.StatusCode, resp.Request.URL)
		rw.WriteHeader(resp.StatusCode)
		defer resp.Body.Close()
		_, err := io.Copy(rw, resp.Body)
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	} else {
		log.Printf("Failed to get response from %s: %s", dst, err)
		rw.WriteHeader(http.StatusServiceUnavailable)
	}
}

func main() {
	flag.Parse()

	// TODO: Використовуйте дані про стан сервреа, щоб підтримувати список тих серверів, яким можна відправляти запит.
	for _, server := range serversPool.servers {
		server := server
		go func() {
			for range time.Tick(10 * time.Second) {
				server.status = health(server.host)
				log.Println(server.host, server.status, server.counter)
			}
		}()
	}

	frontend := httptools.CreateServer(*port, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// TODO: Рееалізуйте свій алгоритм балансувальника.
		server, restore, err := serversPool.getLeastConnected()
		if err != nil {
			log.Println(err)
			rw.WriteHeader(500)
		} else {
			log.Println(server)
			forward(server.host, rw, r)
			restore()
		}
	}))

	log.Println("Starting load balancer...")
	log.Printf("Tracing support enabled: %t", *traceEnabled)
	frontend.Start()
	signal.WaitForTerminationSignal()
}
