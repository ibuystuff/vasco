
---
# `/`

This is ANet's discovery server, named after Vasco de Gama, who was
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






* [register](#register)

* [refresh](#refresh)

* [unregister](#unregister)

* [testRegistration](#testregistration)




---
## register

### `POST /register`

_create a registration object and return its hash_




_**Parameters:**_

Name | Kind | Description | DataType
---- | ---- | ----------- | --------
 body | Body |  | registry.Registration




_**Consumes:**_ `[application/json]`


_**Reads:**_
```json
        {
          "name": "",
          "address": "",
          "pattern": "",
          "status": {
            "path": ""
          }
        }
```


_**Produces:**_ `[application/json]`

Hash value that will be used to refresh or unregister the server later.  Something like:

```"a11807a8b35440438193b436e249a8e7"
```

_**Writes:**_
```json
        ""
```



---
## refresh

### `PUT /register/:hash`

_refresh an existing registration object (I'm still here)_




_**Parameters:**_

Name | Kind | Description | DataType
---- | ---- | ----------- | --------
 hash | Path | the hash returned by the registration | string
 body | Body |  | registry.Registration









---
## unregister

### `DELETE /register/:hash`

_delete a registration._




_**Parameters:**_

Name | Kind | Description | DataType
---- | ---- | ----------- | --------
 hash | Path | the hash returned by the registration | string








_**Error returns:**_

Code | Meaning
---- | --------
 404 | Key not found



---
## testRegistration

### `GET /register/test`

_Returns the result of the load balancer (where the LB would resolve to this time -- repeating this request may return a different result.)_




_**Parameters:**_

Name | Kind | Description | DataType
---- | ---- | ----------- | --------
 url | Query | the url to test | string






_**Produces:**_ `[application/json]`


_**Writes:**_
```json
        {
          "name": "",
          "address": "",
          "pattern": "",
          "status": {
            "path": ""
          }
        }
```


_**Error returns:**_

Code | Meaning
---- | --------
 404 | No matching url found




---
# `/`

The status portion of Vasco reports aggregated status statistics.



* [statusOptions](#statusoptions)

* [statusGeneral](#statusgeneral)

* [statusDetail](#statusdetail)

* [statusSummary](#statussummary)




---
## statusOptions

### `OPTIONS /status`

_Responds to OPTIONS requests to handle CORS._











---
## statusGeneral

### `GET /status`

_Generates aggregated status information._










_**Error returns:**_

Code | Meaning
---- | --------
 500 | There is a major service problem.



---
## statusDetail

### `GET /status/detail`

_Generates detailed status information._








_**Produces:**_ `[application/json]`


_**Writes:**_
```json
        [
          {
            "Address": "http://192.168.1.181:9023",
            "Name": "assess",
            "StatusCode": 200,
            "configtype": "devel",
            "configversion": "None",
            "deploytag": "Branch:master",
            "revision": "b1b171d",
            "uptime": "21h18m0.252103556s"
          }
        ]
```


_**Error returns:**_

Code | Meaning
---- | --------
 500 | There is a major service problem.



---
## statusSummary

### `GET /status/summary`

_Generates summarized status information._








_**Produces:**_ `[text/plain]`


_**Writes:**_
```json
          State   Code                        Ver  Name
```


_**Error returns:**_

Code | Meaning
---- | --------
 500 | There is a major service problem.



