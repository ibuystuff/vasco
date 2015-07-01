#! /bin/bash

# vasco expects environment with VASCO_LOCAL for the command port,
# and VASCO_QUERY for the query port

nohup ./vasco --swagger=$USE_SWAGGER >>$LOGFILE 2>&1
echo $!>$PIDFILE
