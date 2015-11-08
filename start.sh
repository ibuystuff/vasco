#! /bin/bash

# vasco expects environment with VASCO_REGISTRY for the command port,
# VASCO_STATUS for the status port, and VASCO_PROXY for the proxy port

nohup ./vasco >>$LOGFILE 2>&1 &
echo $! >$PIDFILE
