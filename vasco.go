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
	"github.com/AchievementNetwork/vasco/internal/github.com/emicklei/go-restful"
	"github.com/AchievementNetwork/vasco/internal/github.com/emicklei/go-restful/swagger"
	"github.com/AchievementNetwork/vasco/registry"
)

type Vasco struct {
	cache    cache.Cache
	registry registry.Registry
}

func NewVasco(c cache.Cache) *Vasco {
	r := registry.NewRegistry(c)
	return &Vasco{cache: c, registry: *r}
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
		Doc("Manage the registration service")

	svc.Route(svc.POST("").To(v.register).
		Doc("create a registration object and return its hash").
		Operation("register").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON).
		Reads(registry.Registration{}))

	svc.Route(svc.PUT("/{hash}").To(v.refresh).
		Doc("refresh an existing registration object (I'm still here)").
		Operation("refresh").
		Param(svc.PathParameter("hash", "the hash returned by the registration").DataType("string")).
		Reads(registry.Registration{}))

	svc.Route(svc.DELETE("/{hash}").To(v.unregister).
		Doc("delete a registration.").
		Operation("unregister").
		Param(svc.PathParameter("hash", "the hash returned by the registration").DataType("string")).
		Returns(http.StatusNotFound, "Key not found", nil))

	svc.Route(svc.GET("/test").To(v.testRegistration).
		Doc("Returns the result of the load balancer (where the LB would resolve to this time -- repeating this request may return a different result.)").
		Operation("testRegistration").
		Param(svc.QueryParameter("url", "the url to test").DataType("string").Required(true)).
		Produces(restful.MIME_JSON).
		Returns(http.StatusNotFound, "No matching url found", nil).
		Writes(registry.Registration{}))

	svc.Route(svc.GET("/whoami").To(v.whoami).
		Doc("Responds with the caller's address").
		Produces(restful.MIME_JSON).
		Operation("whoami"))

	return svc

}

func makeStatusService(path string, v *Vasco) *restful.WebService {
	svc := new(restful.WebService)
	svc.
		Path(path).
		Doc("Reports aggregated status statistics.")

	svc.Route(svc.GET("").To(v.statusGeneral).
		Doc("Generates aggregated status information.").
		Produces(restful.MIME_JSON).
		Returns(http.StatusInternalServerError, "At least some servers are down.", nil).
		Operation("statusGeneral"))

	svc.Route(svc.GET("/detail").To(v.statusDetail).
		Doc("Generates detailed status information.").
		Produces(restful.MIME_JSON).
		Returns(http.StatusInternalServerError, "At least some servers are down.", nil).
		Operation("statusDetail"))

	return svc
}

func (v *Vasco) RegisterContainer(container *restful.Container) {
	container.Add(makeConfigService("/config", v))
	container.Add(makeRegisterService("/register", v))
	container.Add(makeStatusService("/status", v))
}

func (v *Vasco) RegisterStatusContainer(container *restful.Container) {
	container.Add(makeStatusService("/status", v))
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
	hash := v.registry.Register(reg)

	log.Printf("Registered %s %s as %s \n", reg.Name, reg.Address, hash)
	response.WriteEntity(hash)
	response.WriteHeader(http.StatusOK)
}

func (v *Vasco) refresh(request *restful.Request, response *restful.Response) {
	hash := request.PathParameter("hash")
	reg := v.registry.Find(hash)
	if reg == nil {
		log.Printf("FAILED: Refresh call for %s\n", hash)
		writeError(response, 404, errors.New("No registration found for that hash."))
		return
	}
	v.registry.Refresh(reg)
	log.Printf("Refreshing %s %s\n", reg.Name, reg.Address)
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

func (v *Vasco) whoami(request *restful.Request, response *restful.Response) {
	response.WriteEntity(request.Request.RemoteAddr)
}

func (v *Vasco) unregister(request *restful.Request, response *restful.Response) {
	hash := request.PathParameter("hash")
	v.registry.Unregister(v.registry.Find(hash))
	log.Printf("Unregistered %s\n", hash)
}

func (v *Vasco) statusGeneral(request *restful.Request, response *restful.Response) {
	response.WriteEntity("OK")
}

func (v *Vasco) statusDetail(request *restful.Request, response *restful.Response) {
	st := v.registry.DetailedStatus()
	response.WriteEntity(st)
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

func getEnvWithDefault(name, def string) string {
	if s := os.Getenv(name); s != "" {
		return s
	}
	return def
}

func main() {
	// to see what happens in the package, uncomment the following
	// restful.TraceLogger(log.New(os.Stdout, "[restful] ", log.LstdFlags|log.Lshortfile))

	var kindOfCache string
	var useSwagger bool
	var proxyPort string = getEnvWithDefault("VASCO_PROXY", "8080")
	var registryPort string = getEnvWithDefault("VASCO_REGISTRY", "8081")
	var statusPort string = getEnvWithDefault("VASCO_STATUS", "8082")

	flag.StringVar(&registryPort, "registryport", registryPort, "The registry (management) port.")
	flag.StringVar(&proxyPort, "proxyport", proxyPort, "The proxy (forwarding) port.")
	flag.StringVar(&statusPort, "statusport", statusPort, "The status port.")
	flag.StringVar(&kindOfCache, "cache", "memory", "Specify the type of cache: memory or redis")
	flag.BoolVar(&useSwagger, "swagger", false, "Include the swagger API documentation/testbed")
	flag.Parse()

	var v *Vasco
	switch kindOfCache {
	case "redis":
		log.Fatal("The redis store is not yet implemented.")
	case "memory":
		v = NewVasco(cache.NewLocalCache())
	default:
		panic("Valid cache types are 'memory' and 'redis'")
	}

	v.PreloadFromMap(map[string]string{
		"Env:DISCOVERY_EXPIRATION": "3600", // the time it takes to expire a server if it disappears
	})
	v.PreloadFromEnvironment("DISCOVERY_CONFIG")

	restful.EnableTracing(true)
	wsContainer := restful.NewContainer()
	wsContainer.Router(restful.CurlyRouter{})
	v.RegisterContainer(wsContainer)

	statusContainer := restful.NewContainer()
	statusContainer.Router(restful.CurlyRouter{})
	v.RegisterStatusContainer(statusContainer)

	if useSwagger {
		// Optionally, you can install the Swagger Service which provides a nice Web UI on your REST API
		// You need to download the Swagger HTML5 assets and change the FilePath location in the config below.
		// Open http://localhost:8080/apidocs and enter http://localhost:8080/apidocs.json in the api input field.
		config := swagger.Config{
			WebServices:    wsContainer.RegisteredWebServices(), // you control what services are visible
			WebServicesUrl: "http://localhost:" + registryPort,
			ApiPath:        "/apidocs.json",
			ApiVersion:     "0.1.0", // this should get the current git revision
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
	}

	serverErrors := make(chan error)

	log.Printf("reverse proxy listening on port %s", proxyPort)
	forwarder := &http.Server{Addr: ":" + proxyPort, Handler: NewMatchingReverseProxy(v)}
	go LandS(forwarder, serverErrors)

	log.Printf("status system listening on port %s", statusPort)
	statuser := &http.Server{Addr: ":" + statusPort, Handler: statusContainer}
	go LandS(statuser, serverErrors)

	log.Printf("registry listening on port %s", registryPort)
	server := &http.Server{Addr: ":" + registryPort, Handler: wsContainer}
	go LandS(server, serverErrors)

	err := <-serverErrors
	log.Fatal(err)
}
