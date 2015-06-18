#! /bin/bash

# Runs a simple test of registrations through the server's web-based API

echo '{"name": "/user", "address": "1.1.1.1", "pattern": "/user", "status": {"path": "/status"}}' >F1.txt
echo '{"name": "/user", "address": "1.1.1.2", "pattern": "/user", "status": {"path": "/status"}}' >F2.txt
echo '{"name": "/tags", "address": "1.1.1.3", "pattern": "/tags", "status": {"path": "/status"}}' >F3.txt
echo '{"name": "/tags", "address": "1.1.1.4", "pattern": "/tags", "status": {"path": "/status"}}' >F4.txt
echo '{"name": "default", "address": "1.1.1.5", "pattern": "/", "status": {"path": "/status"}}' >F5.txt

curl --include "http://localhost:8090/register/" --data @F1.txt -H "Content-Type:application/json"
curl --include "http://localhost:8090/register/" --data @F2.txt -H "Content-Type:application/json"
curl --include "http://localhost:8090/register/" --data @F3.txt -H "Content-Type:application/json"
curl --include "http://localhost:8090/register/" --data @F4.txt -H "Content-Type:application/json"
curl --include "http://localhost:8090/register/" --data @F5.txt -H "Content-Type:application/json"

rm F?.txt

curl --include --get "http://localhost:8090/register/test?url=%2Fuser"
curl --include --get "http://localhost:8090/register/test?url=%2Ftags%2Fbizarre%2Fquery"
curl --include --get "http://localhost:8090/register/test?url=%2Fbadpage"
