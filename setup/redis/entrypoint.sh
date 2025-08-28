#!/bin/bash

for i in {7000..7005}
do
  until redis-cli -h redis-node-$((i - 6999)) -p $i ping
  do
    echo "Waiting for redis-node-$((i - 6999)) on port $i..."
    sleep 1
  done
done

cluster_info=$(redis-cli -h redis-node-1 -p 7000 cluster info | grep cluster_state)
if echo "$cluster_info" | grep -q "ok"; then
  echo "Cluster already created"
else
  echo "Creating cluster..."
  yes yes | redis-cli --cluster create \
    redis-node-1:7000 redis-node-2:7001 redis-node-3:7002 \
    redis-node-4:7003 redis-node-5:7004 redis-node-6:7005 \
    --cluster-replicas 1
fi

tail -f /dev/null
