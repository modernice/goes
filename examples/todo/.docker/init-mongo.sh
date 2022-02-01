#!/bin/bash

host=$REPLICA_HOST

echo "Starting replica set initialize"
until mongosh --quiet --host $host --eval "print(\"Waiting for connection...\")"
do
  sleep 0.5
done

echo "Connected."
echo "Creating replica set..."

mongosh --quiet --host $host <<EOF
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
