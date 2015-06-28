#! /bin/bash

# vasco expects environment with VASCO_LOCAL for the command port,
# and VASCO_QUERY for the query port

./vasco --swagger >$LOGFILE 2>&1
