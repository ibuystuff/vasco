/**
 * Name: vasco.go
 * Original author: Kent Quirk
 * Created: 12 June 2015
 * Description: Discovery server for The Achievement Network
 * Copyright 2015 The Achievement Network. All rights reserved.
 */

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"github.com/AchievementNetwork/vasco/cache"
	"github.com/AchievementNetwork/vasco/registry"
	"github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/swagger"
)

type Vasco struct {
	cache    cache.Cache
	registry registry.Registry
}

func makeConfigService(path string, v *Vasco) *restful.WebService {
	svc := new(restful.WebService)
	svc.
		Path(path).
		Doc("Manage a key/value store for config values").
		Consumes(restful.MIME_JSON, "text/plain").
		Produces(restful.MIME_JSON, "text/plain") // you can specify this per route as well

	svc.Route(svc.PUT("/{key}/{value}").To(v.createKey).
		Doc("create a key with an initial value").
		Operation("createKey").
		Param(svc.PathParameter("key", "key to identify this Entry").DataType("string").Required(true)).
		Param(svc.PathParameter("value", "any string").DataType("string").Required(true)))

	svc.Route(svc.GET("/{key}").To(v.findKey).
		Doc("get the contents of a key").
		Operation("findKey").
		Param(svc.PathParameter("key", "the key to fetch").DataType("string")).
		Returns(http.StatusNotFound, "Key not found", nil))

	svc.Route(svc.DELETE("/{key}").To(v.removeKey).
		Doc("delete a key and its tag string.").
		Operation("removeKey").
		Param(svc.PathParameter("key", "the key to delete").DataType("string")).
		Returns(http.StatusNotFound, "Key not found", nil))

	return svc

}

// /register goes to discovery server locally
//     POST /register/myAddr
//         accepts {Registration}
//         returns 200 if registration worked
//     GET /register/url
//         Returns the result of the load balancer (where the LB would resolve to this time --
//         repeating this request may return a different result.)
//     DELETE /register/myAddr
//         Removes the IP and all its children

func makeRegisterService(path string, v *Vasco) *restful.WebService {
	svc := new(restful.WebService)
	svc.
		Path(path).
		Doc("Manage the registration service").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)

	svc.Route(svc.POST("").To(v.register).
		Doc("create a registration object").
		Operation("register").
		Reads(registry.Registration{}))

	svc.Route(svc.PUT("/{name}/{addr}").To(v.refresh).
		Doc("refresh an existing registration object").
		Operation("register").
		Param(svc.PathParameter("name", "the Name field from the registration object").DataType("string")).
		Param(svc.PathParameter("addr", "the host name (and port) for this entry").DataType("string").Required(true)).
		Reads(registry.Registration{}))

	svc.Route(svc.DELETE("/{name}/{addr}").To(v.unregister).
		Doc("delete a registration.").
		Operation("unregister").
		Param(svc.PathParameter("name", "the Name field from the registration object").DataType("string")).
		Param(svc.PathParameter("addr", "the host name (and port) for this entry").DataType("string").Required(true)).
		Returns(http.StatusNotFound, "Key not found", nil))

	svc.Route(svc.GET("/test").To(v.testRegistration).
		Doc("Returns the result of the load balancer (where the LB would resolve to this time -- repeating this request may return a different result.)").
		Operation("testRegistration").
		Param(svc.QueryParameter("url", "the url to test").DataType("string").Required(true)).
		Returns(http.StatusNotFound, "No matching url found", nil).
		Writes(registry.Registration{}))

	return svc

}

func makeStatusService(v *Vasco) *restful.WebService {
	svc := new(restful.WebService)
	svc.
		Doc("Reports aggregated status statistics (but not yet)")

	return svc

}

func (v *Vasco) RegisterContainer(container *restful.Container) {
	container.Add(makeConfigService("/config", v))
	container.Add(makeRegisterService("/register", v))
}

func (v *Vasco) PreloadFromEnvironment(envname string) {
	e := os.Getenv(envname)
	if e == "" {
		return
	}

	type Env map[string]string
	var env Env
	dec := json.NewDecoder(strings.NewReader(e))
	if err := dec.Decode(&env); err != nil {
		log.Fatal(err)
	}

	v.PreloadFromMap(env)
}

func (v *Vasco) PreloadFromMap(m map[string]string) {
	for k, val := range m {
		log.Printf("cache setting '%s' to '%s'", k, val)
		v.cache.Set(k, val)
	}
}

// helper function to write a standard error response
func writeError(response *restful.Response, code int, err error) {
	response.AddHeader("Content-Type", "text/plain")
	response.WriteErrorString(code, err.Error())
}

func (v *Vasco) createKey(request *restful.Request, response *restful.Response) {
	key := request.PathParameter("key")
	value := request.PathParameter("value")

	if err := v.cache.Set(key, value); err != nil {
		writeError(response, http.StatusInternalServerError, err)
	} else {
		response.WriteHeader(http.StatusCreated)
	}
}

func (v *Vasco) findKey(request *restful.Request, response *restful.Response) {
	key := request.PathParameter("key")
	if value, err := v.cache.Get(key); err != nil {
		writeError(response, http.StatusNotFound, err)
	} else {
		response.WriteEntity(value)
	}
}

func (v *Vasco) removeKey(request *restful.Request, response *restful.Response) {
	key := request.PathParameter("key")
	if err := v.cache.Delete(key); err != nil {
		writeError(response, http.StatusNotFound, err)
	}
}

func (v *Vasco) register(request *restful.Request, response *restful.Response) {
	reg := new(registry.Registration)
	if err := request.ReadEntity(reg); err != nil {
		writeError(response, http.StatusForbidden, err)
	}
	if err := reg.SetDefaults(); err != nil {
		writeError(response, http.StatusForbidden, err)
	}
	v.registry.Register(reg)

	log.Printf("Registered %s %s\n", reg.Name, reg.Address)
	response.WriteHeader(http.StatusOK)

}

func (v *Vasco) refresh(request *restful.Request, response *restful.Response) {
	name := request.PathParameter("name")
	addr := request.PathParameter("addr")
	v.registry.Refresh(v.registry.Find(name, addr))
	log.Printf("Refreshing %s %s\n", name, addr)
}

func (v *Vasco) testRegistration(request *restful.Request, response *restful.Response) {
	// to match, we fetch the list of patterns, mat
	url := request.QueryParameter("url")
	if url == "" {
		writeError(response, http.StatusNotFound, errors.New("url query parameter required"))
	}
	if match, err := v.registry.FindBestMatch(url); err != nil {
		writeError(response, http.StatusNotFound, err)
	} else {
		response.WriteEntity(match)
		response.WriteHeader(http.StatusOK)
	}
}

func (v *Vasco) unregister(request *restful.Request, response *restful.Response) {
	name := request.PathParameter("name")
	addr := request.PathParameter("addr")
	v.registry.Unregister(v.registry.Find(name, addr))
	log.Printf("Unregistered %s %s\n", name, addr)
}

// NewMatchingReverseProxy returns a new ReverseProxy that rewrites
// URLs to the scheme and host provided by the registration system. It may
// rewrite the path as well if that was specified.
func NewMatchingReverseProxy(v *Vasco) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		v.registry.RewriteUrl(req.URL)
	}
	return &httputil.ReverseProxy{Director: director}
}

// goroutine that does a ListenAndServe and reports any errors on the error channel
func LandS(srv *http.Server, errs chan error) {
	err := srv.ListenAndServe()
	errs <- err
}

func main() {
	// to see what happens in the package, uncomment the following
	restful.TraceLogger(log.New(os.Stdout, "[restful] ", log.LstdFlags|log.Lshortfile))

	var kindOfCache string
	flag.StringVar(&kindOfCache, "cache", "memory", "Specify the type of cache: memory or redis")
	flag.Parse()

	var v Vasco
	switch kindOfCache {
	case "redis":
		log.Fatal("The redis store is not yet implemented.")
	case "memory":
		c := cache.NewLocalCache()
		r := *registry.NewRegistry(c)
		v = Vasco{cache: c, registry: r}
	default:
		panic("Valid cache types are 'memory' and 'redis'")
	}

	v.PreloadFromMap(map[string]string{
		"Env:DISCOVERY_EXPIRATION": "3600", // the time it takes to expire a server if it disappears
	})
	v.PreloadFromEnvironment("DISCOVERY_CONFIG")

	wsContainer := restful.NewContainer()
	restful.EnableTracing(true)
	wsContainer.Router(restful.CurlyRouter{})
	v.RegisterContainer(wsContainer)

	// Optionally, you can install the Swagger Service which provides a nice Web UI on your REST API
	// You need to download the Swagger HTML5 assets and change the FilePath location in the config below.
	// Open http://localhost:8080/apidocs and enter http://localhost:8080/apidocs.json in the api input field.
	config := swagger.Config{
		WebServices:    wsContainer.RegisteredWebServices(), // you control what services are visible
		WebServicesUrl: "http://localhost:8080",
		ApiPath:        "/apidocs.json",
		ApiVersion:     "0.1.0",
		// Someday we want to have a little more documentation, and we might want to add some additional
		// fields to the swagger.Config object to allow us to specify some of the high-level description
		// stuff (see getListing function).

		// Specify where the UI is located
		SwaggerPath: "/apidocs/",
		// This needs to point to a copy of the dist folder in the docs that can be fetched with:
		// git clone https://github.com/swagger-api/swagger-ui.git
		// Use the dist folder there, and then change the index.html file in it to point to this.
		// url = "http://localhost:8080/apidocs.json";
		SwaggerFilePath: "./swagger-ui/dist",
	}
	swagger.RegisterSwaggerService(config, wsContainer)

	serverErrors := make(chan error)

	log.Printf("forwarder listening on localhost:8081")
	forwarder := &http.Server{Addr: ":8081", Handler: NewMatchingReverseProxy(&v)}
	go LandS(forwarder, serverErrors)

	log.Printf("registry listening on localhost:8080")
	server := &http.Server{Addr: ":8080", Handler: wsContainer}
	go LandS(server, serverErrors)

	err := <-serverErrors
	log.Fatal(err)
}
