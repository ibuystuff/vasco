package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/AchievementNetwork/vasco/cache"
	"github.com/go-zoo/bone"
	"github.com/stretchr/testify/assert"
)

var v *Vasco
var registrymux *bone.Mux
var statusmux *bone.Mux

func TestMain(m *testing.M) {
	v = NewVasco(cache.NewLocalCache(), "/static", "")
	registrymux = v.CreateRegistryService()
	statusmux = v.CreateStatusService()

	memResult := m.Run()
	os.Exit(memResult)
}

func TestSimple(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		log.Fatal(err)
	}
	greeting, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s", greeting)

}

func TestSaveDocs(t *testing.T) {
	req, _ := http.NewRequest("GET", "/md", nil)
	w := httptest.NewRecorder()
	registrymux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req2, _ := http.NewRequest("GET", "/md", nil)
	w2 := httptest.NewRecorder()
	statusmux.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	f, err := os.Create("README.md")
	if err != nil {
		panic("Couldn't create README")
	}
	io.Copy(f, w.Body)
	io.Copy(f, w2.Body)
	f.Close()
}
