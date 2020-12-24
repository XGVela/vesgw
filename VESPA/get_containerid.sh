#!/bin/bash
ctrlen=`cat /proc/self/cgroup | head -1 | cut -d '/' -f 5 | wc -c`
if [ "$ctrlen" == "65" ] ;
then
  export K8S_CONTAINER_ID=`cat /proc/self/cgroup | head -1 | cut -d '/' -f 5`
else
  export K8S_CONTAINER_ID=`cat /proc/self/cgroup | head -1 | cut -d '/' -f 4`
fi

echo $K8S_CONTAINER_ID
