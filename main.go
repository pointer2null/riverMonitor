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

func main() {

	readAPI()
	go func() {
		for range time.Tick(7 * time.Minute) {
			readAPI()
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Info("Starting webservice...")
	defer log.Info("Exiting...")
	log.Fatal(http.ListenAndServe(":50000", nil))
}

func readAPI() {

	client := http.Client{
		Timeout: time.Second * 30, // Timeout after 30 seconds
	}

	res, err := client.Get(url)
	if err != nil {
		log.Error(err)
		riverlevel.Set(-0.1)
		riverperiod.Set(0.0)
		return
	}

	if res.Body == nil {
		return
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Error(err)
		riverlevel.Set(-0.1)
		riverperiod.Set(0.0)
		return
	}

	var result map[string]interface{}
	err = json.Unmarshal([]byte(data), &result)

	if err != nil {
		log.Errorf("Could not unmarshal data [%v]", err)
		riverlevel.Set(-0.1)
		riverperiod.Set(0.0)
		return
	}

	// this is nasty - but for a quick hack will do.

	items := result["items"].([]interface{})[0]

	period := items.(map[string]interface{})["period"]

	detail := items.(map[string]interface{})["latestReading"].(map[string]interface{})

	readTime, err := time.Parse("2006-01-02T15:04:05Z", detail["dateTime"].(string))

	if err != nil {
		log.Errorf("Could not parse dateTime [%v] [%v]", detail["dateTime"], err)
		riverlevel.Set(-0.1)
		riverperiod.Set(0.0)
		return
	}

	if readTime.After(lastData) {
		log.Infof("River level is [%v] at [%v] with period [%v]", detail["value"], readTime.Format(time.RFC822), period)
		riverlevel.Set(detail["value"].(float64))
		riverperiod.Set(period.(float64))
		lastData = readTime
	}
	staleTime := time.Now().AddDate(0, 0, -1)
	//log.Infof("ReadTime [%v], LastRead [%v], staleTime [%v]", readTime, lastData, staleTime)
	if lastData.Before(staleTime) {
		log.Warnf("Stale data - last read [%v]", lastData.Format(time.RFC822))
		riverlevel.Set(-0.1)
		riverperiod.Set(0.0)
	}
}
