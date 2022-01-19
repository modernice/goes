#!/bin/bash

host=$REPLICA_HOST

echo "Starting replica set initialize"
until mongo --host $host --eval "print(\"Waiting for connection...\")"
do
  sleep 2
done

echo "Connected."
echo "Creating replica set..."

mongo --host $host <<EOF
rs.initiate(
  {
    _id : 'rs0',
    members: [ { _id : 0, host : "$host:27017" }]
  }
)
EOF

echo "Replica set created."

sleep infinity &
wait $!
