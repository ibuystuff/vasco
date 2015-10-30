# Vasco

This is ANet's discovery server, named after Vasco de Gama, who was a Portugese explorer who found the first navigable water route from Europe to India, opening up an age of trade.

The purpose of the server is to provide discovery services and routing to a collection of other servers, allowing services to by found by each other and their clients without needing to know very much. It acts as a load balancer as well.

This system was designed by following the premise that the goal of server operations is to reduce the number of places with a list of machines to as close to zero as possible. It supports the idea of either setting up configuration at the same time as the services, or of allowing services to register themselves.

Once services are registered, Vasco then acts as a reverse proxy and load balancer -- queries to the "public" port are directed to the appropriate server behind the firewall by looking at the request and distributing it appropriately. Vasco supports multiple instances of a given pattern and can load balance using a weighted random probability.

## Build instructions

    ```bash
    git clone git@github.com:AchievementNetwork/vasco.git
    go get
    ./build.sh
    ```

## Design

* Vasco supports two ports -- one is intended to be public, one is intended to be private (behind the firewall).
* The private port has a DNS name that is visible behind the firewall. Let's call it "vasco.anet.io".
* When a server wakes up, it contacts vasco.anet.io and says "Hi, my name is 'user' and I respond to queries that match the pattern "/user/".
* Servers must maintain connectivity. Vasco periodically makes status queries and aggregates the responses on its own status port. (Still to be implemented: stop forwarding to a server if it fails to respond to a status query, fails to respond to a forwarded query, or returns 5xx from a forwarded query.)
* Client servers must include in their registration packets the mechanism for making status queries.
* Servers must also maintain connectivity by pinging the vasco refresh endpoint. If a server fails to do this, after the timeout it will be unregistered.
* The default vasco client will force re-registration on a SIGHUP, and also keeps connectivity alive with the refresh prompt.
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
                    [x] Then configuration values are read from the environment for the process. The environment variable DISCOVERY_CONFIG is read, again as a JSON object, and that object is merged with the existing environment (possibly overwriting values read from the user data).
                    [x] Finally, configuration values set with PUT are applied.
            /register goes to discovery server locally
                [x] POST /register
                    accepts {Registration}
                    returns 200 if registration worked and a hash for the registration entry
                [x] PUT /register/hash
                    refreshes an existing registration. This will put it back in service immediately. If the hash has expired, this will return a 404; you will need to re-register.
                [x] DELETE /register/hash
                    Removes the registration entry
                [x] GET /register/test/url
                    Returns the result of the load balancer (the registration object that the LB would resolve to this time -- repeating this request may return a different result.)

        External server has status endpoints predefined:
            [x] /status returns 200 if all servers are responding with 200-class status returns, and 500 if any server is failing.
            [x] /status/summary returns a one-line summary for each server in human-readable form
            [x] /status/detail returns a block of JSON, containing the JSON responses for each individual server.
            [ ] The detail status endpoint should probably be protected in some way, as it leaks a lot of information.

            [x] Any other URL is load-balanced and if found, the result is reverse-proxied to the caller. If no existing server matches, vasco forwards the request to the static server (which is specified in the config).


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

weight:

    When multiple possible paths are matched (usually because there are multiple machines handling a given path), Vasco chooses between them using a weighted random selection.

    Default 100 if not specified. Distributes load randomly to services based on the fraction of the total of all services that matched the query. So if two services match with values of 100 and 50, the first will get 2/3 of the traffic. This is evaluated on every query so it's possible to start a service with a low number for testing and then raise it.


status:

    [x] A JSON object specifying the status behavior:
    path:
        specify the path to be used to check status of the server (this path is concatenated with the address field to build a status query). Status is checked every N seconds, where N is defined in the Vasco configuration. A 200 reply means the server is up and functioning. A payload may be delivered with more detailed status information. It is returned as part of the discover server's status block (if it successfully parses as a JSON object, it is delivered that way, otherwise as a string). This must be specified.


Example
    Simplest usage:

        PUT discoveryserver/register/my.address?pattern=/foo

        That will forward everything to discoveryserver/foo to my.address/foo



## Short term ToDos
Things Vasco still needs:

* Ability to insert URL into swagger
* Generate proper 404s
* Clean up documentation for godoc
* Moar errors

## Longer term

* Support storing all the information in a Redis store so that multiple load balancers can run and will cooperate on things like "sticky" and "roundrobin" strategy -- AWS ElastiCache supports Redis. The design of the memory cache borrows directly from Redis, and the cache is a pluggable item, so it should be simple to create a Redis version of the cache.

