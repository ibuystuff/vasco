/**
 * Name: vasco.go
 * Original author: Kent Quirk
 * Created: 12 June 2015
 * Description: Discovery server for The Achievement Network
 * Copyright 2015, 2016 The Achievement Network. All rights reserved.
 */

package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AchievementNetwork/go-util/boneful"
	"github.com/AchievementNetwork/vasco/cache"
	"github.com/AchievementNetwork/vasco/registry"
	"github.com/go-zoo/bone"
)

// Vasco is a struct that manages the collection of data
type Vasco struct {
	cache          cache.Cache
	registry       *registry.Registry
	lastStatus     registry.StatusBlock
	statusTimer    *LoopTimer
	allowedMethods []string
	allowedHeaders []string
	allowedOrigins []string
}

func NewVasco(c cache.Cache, staticPath string, expected string) *Vasco {
	stimeout := getEnvWithDefault("DISCOVERY_EXPIRATION", "3600")
	timeout, _ := strconv.Atoi(stimeout)
	r := registry.NewRegistry(c, staticPath, expected, timeout)
	return &Vasco{
		cache:    c,
		registry: r,
		// if these ever need to vary based on the deploy it would be better if
		// they came from the environment. But right now it doesn't seem necessary.
		allowedOrigins: []string{"*"},
		allowedMethods: []string{"POST", "GET", "DELETE", "PUT", "OPTIONS"},
		allowedHeaders: []string{
			"X-ANET-TOKEN",
			"X-ACCESS_TOKEN",
			"Access-Control-Allow-Origin",
			"Authorization",
			"Origin",
			"x-requested-with",
			"Content-Type",
			"Content-Range",
			"Content-Disposition",
			"Content-Description",
		},
	}
}

// logit is middleware to log requests
func logit(handler http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		log.Printf("%s %s\n", req.Method, req.URL)
		handler(rw, req)
	}
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

func (v *Vasco) CreateRegistryService() *bone.Mux {
	svc := new(boneful.Service).
		Path("/").
		Doc(`This is ANet's discovery server, named after Vasco de Gama, who was
			a Portugese explorer who found the first navigable water route from
			Europe to India, opening up an age of trade.

			The purpose of the server is to provide discovery services and
			routing to a collection of other servers, allowing services to by
			found by each other and their clients without needing to know very
			much. It acts as a load balancer as well.

			This system was designed by following the premise that the goal of
			server operations is to reduce the number of places with a list of
			machines to as close to zero as possible. It supports the idea of
			either setting up configuration at the same time as the services, or
			of allowing services to register themselves.

			Once services are registered, Vasco then acts as a reverse proxy and
			load balancer -- queries to the "public" port are directed to the
			appropriate server behind the firewall by looking at the request and
			distributing it appropriately. Vasco supports multiple instances of
			a given pattern and can load balance using a weighted random
			probability.


		## Design

		* Vasco supports two ports -- one is intended to be public, one is intended to be private (behind the firewall).
		* The private port has a DNS name that is visible behind the firewall. Let's call it "vasco.anet.io".
		* When a server wakes up, it contacts vasco.anet.io and says "Hi, my name is 'user' and I respond to queries that match the pattern "/user/".
		* Servers must maintain connectivity. Vasco periodically makes status queries and aggregates the responses on its own status port.
		* Client servers must include in their registration packets the mechanism for making status queries.
		* Servers must also maintain connectivity by pinging the vasco refresh endpoint. If a server fails to do this, after the timeout it will be unregistered.
		* The default vasco client will force re-registration on a SIGHUP, and also keeps connectivity alive with the refresh prompt.
		* Vasco receives queries and reverse-proxies them to the servers.

		## Registration
		Registration is a JSON object that supports the following fields:

		### name

		An arbitrary name string that is used in status reporting.

		### address

	    The HTTP scheme and host (IP/port combination) used for forwarding requests

		### pattern

	    A regex match for the path starting at the leading slash.
	    Note that the use of a regex implies that you need to be careful about the trailing
	    elements of your pattern. If you mean "/tags/" you have to say "/tags/" not "/tags".

	    If the regex includes parentheses for part of the pattern, the redirected request is made with only the parenthesized part of the query.
	    Example:
	        { "pattern": "/foo/" } -- server/foo/bar redirects to myAddr/foo/bar
	        { "pattern": "/foo(/.*)" } -- server/foo/bar redirects to myAddr/bar

	    Requests are matched against all outstanding patterns with a status of "up" or "failing" -- the pattern with the longest successful unparenthesized match is used to redirect the request. If multiple matching patterns have the same length, the strategy field is used to decide which match is used.

	    If a forwarded request times out, the server is immediately marked with a status of "down".

		### weight

	    When multiple possible paths are matched (usually because there are multiple machines handling a given path), Vasco chooses between them using a weighted random selection.

	    Default 100 if not specified. Distributes load randomly to services based on the fraction of the total of all services that matched the query. So if two services match with values of 100 and 50, the first will get 2/3 of the traffic. This is evaluated on every query so it's possible to start a service with a low number for testing and then raise it.

		### status

    	A JSON object specifying the status behavior:

	    ### path

        The path to be used to check status of the server (this path is concatenated with the address field to build a status query). Status is checked every N seconds, where N is defined in the Vasco configuration. A 200 reply means the server is up and functioning. A payload may be delivered with more detailed status information. It is returned as part of the discover server's status block (if it successfully parses as a JSON object, it is delivered that way, otherwise as a string). This must be specified.


		### Example

	    Simplest usage:

	        PUT discoveryserver/register/my.address?pattern=/foo

	        That will forward everything to discoveryserver/foo to my.address/foo


		`)

	svc.Route(svc.POST("/register").To(logit(v.register)).
		Doc("create a registration object and return its hash").
		Operation("register").
		Consumes("application/json").
		Produces("application/json").
		Reads(registry.Registration{}).
		Writes(""))

	svc.Route(svc.PUT("/register/:hash").To(logit(v.refresh)).
		Doc("refresh an existing registration object (I'm still here)").
		Operation("refresh").
		Param(boneful.PathParameter("hash", "the hash returned by the registration").DataType("string")).
		Reads(registry.Registration{}))

	svc.Route(svc.DELETE("/register/:hash").To(logit(v.unregister)).
		Doc("delete a registration.").
		Operation("unregister").
		Param(boneful.PathParameter("hash", "the hash returned by the registration").DataType("string")).
		Returns(http.StatusNotFound, "Key not found", nil))

	svc.Route(svc.GET("/register/test").To(logit(v.testRegistration)).
		Doc("Returns the result of the load balancer (where the LB would resolve to this time -- repeating this request may return a different result.)").
		Operation("testRegistration").
		Param(boneful.QueryParameter("url", "the url to test").DataType("string").Required(true)).
		Produces("application/json").
		Returns(http.StatusNotFound, "No matching url found", nil).
		Writes(registry.Registration{}))

	return svc.Mux()

}

func (v *Vasco) CreateStatusService() *bone.Mux {
	svc := new(boneful.Service).
		Path("/").
		Doc("The status portion of Vasco reports aggregated status statistics.")

	svc.Route(svc.OPTIONS("/status").To(v.statusOptions).
		Doc("Responds to OPTIONS requests to handle CORS.").
		Operation("statusOptions"))

	svc.Route(svc.GET("/status").To(v.statusGeneral).
		Doc("Generates aggregated status information.").
		Returns(http.StatusInternalServerError, "There is a major service problem.", nil).
		Operation("statusGeneral"))

	svc.Route(svc.GET("/status/strict").To(v.statusStrict).
		Doc("Returns 200 only if all expected servers are up.").
		Returns(http.StatusInternalServerError, "At least one server is down.", nil).
		Operation("statusStrict"))

	svc.Route(svc.GET("/status/detail").To(v.statusDetail).
		Doc("Generates detailed status information.").
		Param(boneful.QueryParameter("wait", "if non-empty, wait for current status from all services before returning result.").DataType("string").Required(false)).
		Produces("application/json").
		Returns(http.StatusInternalServerError, "There is a major service problem.", nil).
		Operation("statusDetail").
		Writes(registry.StatusBlock{registry.StatusItem{
			"Address":       "http://192.168.1.181:9023",
			"Name":          "assess",
			"StatusCode":    200,
			"configtype":    "devel",
			"configversion": "None",
			"deploytag":     "Branch:master",
			"revision":      "b1b171d",
			"uptime":        "21h18m0.252103556s",
		}}))

	svc.Route(svc.GET("/status/summary").To(v.statusSummary).
		Doc("Generates summarized status information.").
		Produces("text/plain").
		Writes("  State   Code                        Ver  Name").
		Returns(http.StatusInternalServerError, "There is a major service problem.", nil).
		Operation("statusSummary"))

	return svc.Mux()
}

// Base type for a proxy that rewrites URLs
type MatchingReverseProxy struct {
	H http.Handler
	V *Vasco
	A requestAuthenticator
}

// we can inject headers this way and also handle options methods
func (f MatchingReverseProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	acheaders := map[string]string{
		"Access-Control-Allow-Origin":  strings.Join(f.V.allowedOrigins, ","),
		"Access-Control-Allow-Methods": strings.Join(f.V.allowedMethods, ","),
		"Access-Control-Allow-Headers": strings.Join(f.V.allowedHeaders, ","),
	}
	for k, v := range acheaders {
		w.Header().Add(k, v)
	}

	// if it's just an options request, we don't need to do anything
	// and can short-circuit the response
	if req.Method == "OPTIONS" {
		log.Printf("Access-Control-Request-Headers: %s", req.Header["Access-Control-Request-Headers"])
		return
	}

	usr, err := f.A.authenticateRequest(req)
	if err != nil {
		log.Printf("Forbidden: %s | %s", req.URL.Path, err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// It's possible for user to legitimately be nil here in the case of paths
	// that aren't eligible for authentication checks (e.g. assets served
	// by the following services: static, snap, pdf). Those paths are defined
	// in acl.json.
	if usr != nil {
		w.Header().Add("X-USER-ARN", usr.arn)
	}

	t := time.Now()
	f.H.ServeHTTP(w, req)
	dt := time.Now().Sub(t) / time.Microsecond

	log.Printf("%s -> %s: %d uSec", req.RequestURI, req.URL.String(), dt)
}

// NewMatchingReverseProxy returns a new ReverseProxy that rewrites
// URLs to the scheme and host provided by the registration system. It may
// rewrite the path as well if that was specified.
func NewMatchingReverseProxy(v *Vasco, a requestAuthenticator) *MatchingReverseProxy {
	director := func(req *http.Request) {
		v.registry.RewriteUrl(req.URL)
	}

	return &MatchingReverseProxy{V: v, H: &httputil.ReverseProxy{Director: director}, A: a}
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
	var kindOfCache string
	var proxyPort string = getEnvWithDefault("VASCO_PROXY", "8080")
	var registryPort string = getEnvWithDefault("VASCO_REGISTRY", "8081")
	var statusPort string = getEnvWithDefault("VASCO_STATUS", "8082")
	var staticPath string = getEnvWithDefault("STATIC_PATH", "")
	var expectedServices string = getEnvWithDefault("EXPECTED_SERVICES", "")
	var redisAddr string = getEnvWithDefault("REDIS_ADDR", "")

	flag.StringVar(&registryPort, "registryport", registryPort, "The registry (management) port.")
	flag.StringVar(&proxyPort, "proxyport", proxyPort, "The proxy (forwarding) port.")
	flag.StringVar(&statusPort, "statusport", statusPort, "The status port.")
	flag.StringVar(&kindOfCache, "cache", "memory", "Specify the type of cache: memory or redis")
	flag.Parse()

	var err error
	if _, err = url.Parse(redisAddr); redisAddr != "" && err == nil {
		kindOfCache = "redis"
		log.Printf("kindOfCache: %s", kindOfCache)
		log.Printf("redisAddr: %s", redisAddr)
	}

	var v *Vasco
	switch kindOfCache {
	case "redis":
		v = NewVasco(cache.NewRedisCache(redisAddr), staticPath, expectedServices)
	case "memory":
		v = NewVasco(cache.NewLocalCache(), staticPath, expectedServices)
	default:
		panic("Valid cache types are 'memory' and 'redis'")
	}

	registryMux := v.CreateRegistryService()
	statusMux := v.CreateStatusService()

	// wait a few seconds to let clients find us and then start requesting and watching status
	statusTime, _ := strconv.Atoi(getEnvWithDefault("STATUS_TIME", "60"))
	v.statusTimer = NewLoopTimer(250*time.Millisecond, time.Duration(statusTime)*time.Second, v.statusUpdate)
	v.statusTimer.AtMost(10 * time.Second)

	serverErrors := make(chan error)

	rulesFile, err := os.Open("acl.json")
	if err != nil {
		log.Fatalf("unable to load rules file because %s", err)
	}
	defer rulesFile.Close()
	iam, err := newRequestAuthenticator(rulesFile)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("reverse proxy listening on port %s", proxyPort)
	forwarder := &http.Server{Addr: ":" + proxyPort, Handler: NewMatchingReverseProxy(v, iam)}
	go LandS(forwarder, serverErrors)

	log.Printf("status system listening on port %s", statusPort)
	statuser := &http.Server{Addr: ":" + statusPort, Handler: statusMux}
	go LandS(statuser, serverErrors)

	log.Printf("registry listening on port %s", registryPort)
	server := &http.Server{Addr: ":" + registryPort, Handler: registryMux}
	go LandS(server, serverErrors)

	err = <-serverErrors
	log.Fatal(err)
}

func newRequestAuthenticator(r io.Reader) (*IAM, error) {
	pac, err := newPathAccessController(r)
	if err != nil {
		return nil, err
	}
	return newIAM(pac)
}
