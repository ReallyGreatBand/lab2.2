package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/ReallyGreatBand/lab2.2/httptools"
	"github.com/ReallyGreatBand/lab2.2/signal"
)

var port = flag.Int("port", 8080, "server port")
var db = flag.String("database", "http://database:18080/db/", "server database")

const confResponseDelaySec = "CONF_RESPONSE_DELAY_SEC"
const confHealthFailure = "CONF_HEALTH_FAILURE"

type Msg struct {
	Key string
	Value string
}

func main() {
	h := new(http.ServeMux)

	h.HandleFunc("/health", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("content-type", "text/plain")
		if failConfig := os.Getenv(confHealthFailure); failConfig == "true" {
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte("FAILURE"))
		} else {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte("OK"))
		}
	})

	report := make(Report)

	h.HandleFunc("/api/v1/some-data", func(rw http.ResponseWriter, r *http.Request) {
		_ = os.Setenv(confResponseDelaySec, "1")
		respDelayString := os.Getenv(confResponseDelaySec)
		if delaySec, parseErr := strconv.Atoi(respDelayString); parseErr == nil && delaySec > 0 && delaySec < 300 {
			time.Sleep(time.Duration(delaySec) * time.Second)
		}

		report.Process(r)

		rw.Header().Set("content-type", "application/json")
		key, ok := r.URL.Query()["key"]

		if !ok || len(key) == 0 {
			log.Printf("Bad request")
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		body, err := http.Get(*db + key[0])
		if err != nil {
			log.Printf("Error sending request to database: %s", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		if body.StatusCode != http.StatusOK {
			switch body.StatusCode {
			case http.StatusNotFound:
				rw.WriteHeader(http.StatusNotFound)
				log.Printf("Key not found %s", key)
				return
			default:
				rw.WriteHeader(http.StatusInternalServerError)
				log.Printf("Internal error looking for %s", key)
				return
			}
		}

		var val Msg
		err = json.NewDecoder(body.Body).Decode(&val)

		if err != nil {
			log.Printf("Failed to read body: %s", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}


		rw.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(rw).Encode(val)
	})

	h.Handle("/report", report)

	server := httptools.CreateServer(*port, h)
	date := "2010-05-29"
	res, err := http.Post(*db + "reallygreatband", "application/json", bytes.NewBuffer([]byte(fmt.Sprintf(`{"value": "%s"}`, date))))
	if err != nil || res.StatusCode != http.StatusOK {
		log.Printf("Error posting value current date to database: %s", err)
	}

	log.Printf("Sent value %s", date)
	server.Start()
	signal.WaitForTerminationSignal()
}
