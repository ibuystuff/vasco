package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AchievementNetwork/go-util/util"
	"github.com/AchievementNetwork/vasco/registry"
	"github.com/go-zoo/bone"
)

// whenever we know registration has changed, we want to update
// the status system soon -- but since every update also requires
// a bunch of http traffic, we don't want to hammer the servers during
// startup, so we just do it "soon"; if multiple calls are received
// during this timeout, the timer is reset.
func (v *Vasco) refreshStatusSoon() {
	v.statusTimer.Reset(5 * time.Second)
}

func (v *Vasco) register(rw http.ResponseWriter, req *http.Request) {
	v.refreshStatusSoon()
	var reg = new(registry.Registration)
	dec := json.NewDecoder(req.Body)
	err := dec.Decode(reg)
	if err != nil {
		log.Println("Couldn't read registration request: ", err.Error())
		util.WriteNewWebError(rw, http.StatusBadRequest, "VAS-100", err.Error())
		return
	}
	if err := reg.SetDefaults(); err != nil {
		log.Println("Couldn't set defaults: ", err.Error())
		util.WriteNewWebError(rw, http.StatusBadRequest, "VAS-101", err.Error())
		return
	}
	hash := v.registry.Register(reg, true)
	log.Printf("Registered %s %s as %s \n", reg.Name, reg.Address, hash)

	r2 := v.registry.Find(hash)
	if r2 == nil {
		log.Printf("Unable to find the hash we just registered!")
	}

	util.WriteJSON(rw, hash)
}

func (v *Vasco) refresh(rw http.ResponseWriter, req *http.Request) {
	hash := bone.GetValue(req, "hash")
	reg := v.registry.Find(hash)
	if reg == nil {
		log.Printf("FAILED: Refresh call for '%s'\n", hash)
		util.WriteNewWebError(rw, http.StatusNotFound, "VAS-102", "No registration found for that hash.")
		return
	}
	v.registry.Refresh(reg)
	v.refreshStatusSoon()
	log.Printf("Refreshing %s %s\n", reg.Name, reg.Address)
}

func (v *Vasco) testRegistration(rw http.ResponseWriter, req *http.Request) {
	u := req.URL.Query().Get("url")
	if u == "" {
		util.WriteNewWebError(rw, http.StatusNotFound, "VAS-103", "url query parameter required")
		return
	}
	match, err := v.registry.FindBestMatch(u)
	if err != nil {
		util.WriteNewWebError(rw, http.StatusNotFound, "VAS-101", err.Error())
		return
	}
	util.WriteJSON(rw, match)
}

func (v *Vasco) unregister(rw http.ResponseWriter, req *http.Request) {
	hash := bone.GetValue(req, "hash")
	v.registry.Unregister(v.registry.Find(hash))
	log.Printf("Unregistered %s\n", hash)
	v.refreshStatusSoon()
}

// the status request always returns 200 because we need to be able to
// examine status to figure out what's going on.
func (v *Vasco) statusGeneral(rw http.ResponseWriter, req *http.Request) {
	for _, v := range v.lastStatus {
		stat := v["StatusCode"]
		if stat == nil || stat.(int) < 200 || stat.(int) > 299 {
			log.Printf("Status problem %d on %s", stat, v["Name"])
		}
	}
}

const sumfmt = "%7s %6s %26s  %s\n"

func (v *Vasco) statusSummary(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, sumfmt, "State", "Code", "Ver", "Name")
	for _, v := range v.lastStatus {
		stat := v["StatusCode"]
		name := v["Name"]
		tag := v["deploytag"]
		if tag == nil || tag == "" {
			tag = "unknown"
		}
		state := "ok"
		if stat == nil || stat.(int) < 200 || stat.(int) > 299 {
			state = "NOT OK"
		}
		fmt.Fprintf(rw, sumfmt, state, strconv.FormatInt(int64(stat.(int)), 10), tag, name)
	}

	v.refreshStatusSoon()
}

// respond to options requests with appropriate headers
func (v *Vasco) statusOptions(rw http.ResponseWriter, req *http.Request) {
	acheaders := map[string]string{
		"Access-Control-Allow-Origin":  strings.Join(v.allowedOrigins, ","),
		"Access-Control-Allow-Methods": strings.Join(v.allowedMethods, ","),
		"Access-Control-Allow-Headers": strings.Join(v.allowedHeaders, ","),
	}
	for k, v := range acheaders {
		rw.Header().Add(k, v)
	}
}

func (v *Vasco) statusDetail(rw http.ResponseWriter, req *http.Request) {
	util.WriteJSON(rw, v.lastStatus)
	v.refreshStatusSoon()
}

func (v *Vasco) statusUpdate() {
	statSTime := getEnvWithDefault("STATUS_TIME", "60")
	statTime, _ := strconv.Atoi(statSTime)
	if statTime == 0 {
		statTime = 60
	}
	v.lastStatus = v.registry.DetailedStatus()
	vascostat := registry.StatusItem{
		"Name":          "vasco",
		"Port":          getEnvWithDefault("VASCO_REGISTRY", "8081"),
		"StatusCode":    200,
		"revision":      os.Getenv("REVISION"),
		"deploytag":     os.Getenv("DEPLOYTAG"),
		"configtype":    os.Getenv("DEPLOYTYPE"),
		"configversion": os.Getenv("CONFIGVERSION"),
		"pid":           os.Getpid(),
	}
	if ip, err := util.ExternalIP(); err != nil {
		vascostat["ip"] = err.Error()
	} else {
		vascostat["ip"] = ip
		vascostat["Address"] = "http://" + ip + ":" + vascostat["Port"].(string)
	}

	v.lastStatus = append(v.lastStatus, vascostat)
	v.statusTimer = time.AfterFunc(time.Duration(statTime)*time.Second, v.statusUpdate)
}
