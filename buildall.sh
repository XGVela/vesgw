#!/bin/bash
# Copyright 2020 Mavenir
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

ARTIFACTS_PATH="./artifacts"
BUILDER_ARG=""
COMMON_ARG=""
CONFIG_RELEASE_LIST="charts/ves-agent"

if [ $# -lt 2 ];
then
echo "[VES-BUILD] Error, Too few arguments!!!"
exit 1
fi

##NANO SEC timestamp LABEL, to enable multiple build in same system
VESGW_BUILDER_LABEL="vesgw-builder-$(date +%s%9N)"
echo -e "[CIM-BUILD] Build MICROSERVICE_NAME:ves-gw, Version:$1"
docker build --rm \
             --build-arg VESGW_BUILDER_LABEL=$VESGW_BUILDER_LABEL \
             -f ./build/Ves_Gateway_Dockerfile \
             -t ves-gw:$1 .

#VESSIM_BUILDER_LABEL="vessim-builder-$(date +%s%9N)"
#echo -e "[CIM-BUILD] Build MICROSERVICE_NAME:ves-sim, Version:$2"
#docker build --rm \
#             --build-arg VESSIM_BUILDER_LABEL=$VESSIM_BUILDER_LABEL \
#             -f ./build/Ves_Collector_Simulator_Dockerfile \
#             -t ves-sim:$2 .

echo -e "[CIM-BUILD] Setting Artifacts Environment"
rm -rf $ARTIFACTS_PATH
mkdir -p $ARTIFACTS_PATH
mkdir -p $ARTIFACTS_PATH/images
mkdir -p $ARTIFACTS_PATH/charts

echo -e "[CIM-BUILD] Releasing Artifacts... @$ARTIFACTS_PATH"
docker save ves-gw:$1 | gzip > $ARTIFACTS_PATH/images/ves-gw-$1.tar.gz
#docker save ves-sim:$2 | gzip > $ARTIFACTS_PATH/images/ves-sim-$2.tar.gz
cp -rf charts/vesgw $ARTIFACTS_PATH/charts/
sed -i -e "s/ves_gw_tag/$1/" $ARTIFACTS_PATH/charts/vesgw/values.yaml
#cp -rf charts/ves-simulator $ARTIFACTS_PATH/charts/
#sed -i -e "s/ves_simulator_tag/$2/" $ARTIFACTS_PATH/charts/ves-simulator/values.yaml

echo -e "[CIM-BUILD] Deleting Intermidiate Containers..."
docker image prune -f --filter "label=IMAGE-TYPE=$VESGW_BUILDER_LABEL"
#docker image prune -f --filter "label=IMAGE-TYPE=$VESSIM_BUILDER_LABEL"
docker rmi -f ves-gw:$1
#docker rmi -f ves-sim:$2

