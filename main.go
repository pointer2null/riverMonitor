package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	log "github.com/sirupsen/logrus"
)

var riverlevel = prometheus.NewGauge(
    prometheus.GaugeOpts{
        Name: "riverlevel",
        Help: "River Level m",
    },
)

var riverperiod = prometheus.NewGauge(
    prometheus.GaugeOpts{
        Name: "period",
        Help: "Period",
    },
)

var lastData time.Time

const url = "http://environment.data.gov.uk/flood-monitoring/id/stations/53125/measures"

// history of readings from
// http://environment.data.gov.uk/flood-monitoring/id/stations/53125/readings?_sorted&_limit=100
// all latest
// http://environment.data.gov.uk/flood-monitoring/data/readings?latest

// problem is they are updated in batches - sometimes only one update in 6 or more hours


func init() {
	log.Infof("%v: Initialize prometheus...", time.Now().Format(time.RFC822))
	prometheus.MustRegister(riverlevel)
	prometheus.MustRegister(riverperiod)
}

func main (){

	readAPI()
	go func (){
		for range time.Tick(7 * time.Minute) {
			readAPI()
		}
	}()
		
	http.Handle("/metrics", promhttp.Handler())
	log.Info("Starting webservice...")
	defer log.Info("Exiting...")
	log.Fatal(http.ListenAndServe(":50000", nil))
}

func readAPI(){
	
	client := http.Client{
		Timeout: time.Second * 2, // Timeout after 2 seconds
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}

	res, getErr := client.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	data, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	var result map[string]interface{}
	json.Unmarshal([]byte(data), &result)	

	items := result["items"].([]interface{})[0]

	period := items.(map[string]interface{})["period"]
	
	detail := items.(map[string]interface{})["latestReading"].(map[string]interface{})
	
	readTime, err := time.Parse("2006-01-02T15:04:05Z", detail["dateTime"].(string))

	if err != nil {
		log.Errorf("Could not parse dateTime [%v] [%v]", detail["dateTime"], err)
	}

	if readTime.After(lastData) {
		log.Infof("River level is [%v] at [%v] with period [%v]", detail["value"], readTime .Format(time.RFC822), period)
		riverlevel.Set(detail["value"].(float64))
		riverperiod.Set(period.(float64))
		lastData = readTime
	}
}