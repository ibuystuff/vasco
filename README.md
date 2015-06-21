# Vasco

This is ANet's discovery server, named after Vasco de Gama, who was a Portugese explorer who found the first navigable water route from Europe to India, opening up an age of trade.

The purpose of the server is to provide discovery services to a collection of other servers, allowing them to find one another without needing to know very much. It acts as a load balancer as well.

The basic idea is that servers register themselves (or are registered in a configuration). Vasco then acts as a reverse proxy and load balancer -- queries to the public port are directed to the appropriate server behind the firewall by looking at the request and distributing. Vasco supports multiple instances of a given pattern and can follow different strategies for load balancing.

## Build instructions

    ```bash
    git clone git@github.com:AchievementNetwork/vasco.git
    go get
    go build
    ```

## Design

* Vasco supports two ports -- one is intended to be public, one is intended to be private (behind the firewall).
* The private port has a DNS name that is visible behind the firewall. Let's call it "vasco.anet.io".
* When a server wakes up, it contacts vasco.anet.io and says "Hi, my name is 'user' and I respond to queries that match the pattern "/user/".
* Servers must maintain connectivity. Vasco periodically makes status queries, and will stop forwarding to a server if it fails to respond to a status query OR to a forwarded query.
* Client servers must include in their registration packets the mechanism for making status queries - Vasco periodically pings them and if they fail to respond it will stop distributing to them; once they start responding again it will re-enable them.
* The clients should consider adding a timeout to the status check endpoint. If they don't receive periodic pings, they should assume that Vasco has crashed or otherwise lost the connection and attempt re-discovery.
* Vasco receives queries and reverse-proxies them to the servers.

## API

(items with [x] are implemented, [ ] not yet)

    Main server listens on two address/ports, internal and external. Both are configured in the environment.
        Internal server has a couple of endpoints:
            /config provides a configuration key/value store that can be set up in environment but also updated with http requests
                [x] GET /config/key -- returns value as text/plain
                [x] PUT /config/key/value -- sets key to value
                [x] DELETE /config/key
                Configuration can be set up in several ways:
                    [x] Set up as default values by the vasco app. This is done for certain configuration parameters like:
                        DISCOVERY_EXPIRATION = 60       // the time it takes for discovery records to expire
                    [ ] read from the AWS user data (set at instance setup time) as a JSON object. Text read from the configuration is loaded as a JSON object (keys and values must be strings) -- if it contains a key called "discovery", that value is the initial value for the configuration keys -- otherwise, the entire object is used. So { "foo": 1, "bar": "space" } ends up as /config/foo and /config/bar, as does {"discovery": { "foo": 1, "bar": "space" }, "test":"blah" }.
                    [x] Then configuration values are read from the environment for the process. The environment variable DISCOVERY_CONFIG is read, again as a JSON object, and that object is merged with the existing environment (possibly overwriting values read from the user data).
                    [x] Finally, configuration values set with PUT are applied.
            /register goes to discovery server locally
                [x] POST /register
                    accepts {Registration}
                    returns 200 if registration worked
                [x] PUT /register/myName/myAddr
                    refreshes an existing registration (it also works to re-register it). This will put it back in service immediately.
                [x] DELETE /register/myName/myAddr
                    Removes the IP and all its children
                [x] GET /register/test/url
                    Returns the result of the load balancer (the registration object that the LB would resolve to this time -- repeating this request may return a different result.)

        External server has one pre-defined endpoint:
            [ ] /status/{validation} reports a status block containing top-level status information a JSON object with status information for all servers that are currently registered. The validation string is specified in the configuration so that configuration information is not casually leaked.

            [x] Any other URL is load-balanced and if found, the result is reverse-proxied to the caller. If nothing matches, the discovery server generates a 404 unless a default URL is registered.

        The myAddr parameter can be any valid domain name or IP address, and may include a port number.
        So:
            192.168.1.100:8080
            192.168.1.100
            localhost:8080
            my.server.com

Registration is a JSON object that supports the following fields:

name:

    An arbitrary name string that is used in status reporting.

address:

    The HTTP scheme and host (IP/port combination) used for forwarding requests

pattern:

    pattern: A regex match for the path starting at the leading slash.
    Note that the use of a regex implies that you need to be careful about the trailing
    elements of your pattern. If you mean "/tags/" you have to say "/tags/" not "/tags".

    If the regex includes parentheses for part of the pattern, the redirected request is made with only the parenthesized part of the query.
    Example:
        { "pattern": "/foo/" } -- server/foo/bar redirects to myAddr/foo/bar
        { "pattern": "/foo(/.*)" } -- server/foo/bar redirects to myAddr/bar

    Requests are matched against all outstanding patterns with a status of "up" or "failing" -- the pattern with the longest successful unparenthesized match is used to redirect the request. If multiple matching patterns have the same length, the strategy field is used to decide which match is used.

    If a forwarded request times out, the server is immediately marked with a status of "down".

strategy:

    strategy: "roundrobin" or "random" or "stickyrandom" or "stickyroundrobin"
    This controls how requests are routed.
        Roundrobin cycles between servers for a given request in a fixed order
        Random chooses a server randomly to respond to a given request
        Sticky uses the given strategy for a new request from a given IP but then directs further requests from that same IP to the given server.
    [x] Default is random.
    [ ] Sticky and roundrobin are not yet implemented (and probably won't be for a while)

ttl:

    [ ] Only used for a sticky strategy -- controls how long an IP lives in the cache. Specify a time in integer numbers of seconds. Default is 15 minutes (900 seconds)

status:

    [ ] A JSON object specifying the status behavior:
    path:
        specify the path to be used to check status of the server (this path is concatenated with the address field to build a status query). A 200 reply means the server is up and functioning. A JSON payload may be delivered with more detailed status information; it is not inspected, merely returned as part of the discover server's status block. Default is myAddr/status.

    frequency:
        An integer number of seconds. how often the status should be checked. Default = 5 seconds.

    downcount:
        An integer. The server is marked as out of service if it fails to reply to a status request (times out) this many times in a row. Default = 2. Note that if a server replies with a non-200 value it is marked down immediately. This can be used to throttle requests to a server under heavy load.

    upcount:
        An integer. After being marked out of service, the server must respond 200 to this many repeated requests in order to be considered "up". Default = 3.

    timeout:
        An integer number of milliseconds before a request should be considered nonresponsive. Default = 500 msec.


Example
    Simplest usage:

        PUT discoveryserver/register/my.address?pattern=/foo

        That will forward everything to discoveryserver/foo to my.address/foo

Status:
    [ ] Returns a status block like this:

    {
        summary: "All servers up" // or "All services up but some servers down" or "Some services down."
        // summary exists so that services like pingdom can get an easy answer for "is everything ok?"
        serverAddr : {
            registeredAt: "2015-03-05T08:43:12.123Z",
            pingsUp: 1233,
            pingsDown: 17
            currentStatus: "up"  // or "down" or "failing" (when it hasn't reached downcount yet) or "recovering" (when it hasn't reached upcount yet after being down)
            serving: [ (list of patterns it is serving) ]
            lastDownAt: "2015-03-05T08:43:12.123Z" // the last time the server transitioned to Down
            lastUpAt: "2015-03-05T08:43:12.123Z" // the last time the server transitioned to Up
        }
    }



## Short term ToDos
Things Vasco still needs:

* Make ports and other info configurable
    * Ability to insert URL into swagger
    * Generate proper 404s
* Clean up documentation for godoc
* Moar errors
* Separate util library
* Separate stringset library
* Start checking status URLs
* Clean up status URL stuff

## Longer term

* Add support for load balancing strategies
* Support storing all the information in a Redis store so that multiple load balancers can run and will cooperate on things like "sticky" and "roundrobin" strategy -- AWS ElastiCache supports Redis. The design of the memory cache borrows directly from Redis, and the cache is a pluggable item, so it should be simple to create a Redis version of the cache.

