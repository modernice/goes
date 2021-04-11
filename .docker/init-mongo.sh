#!/bin/bash

echo "Starting replica set initialize"
until mongo --host mongostore_replicaset --eval "print(\"Waiting for connection...\")"
do
    sleep 2
done

echo "Connected."
echo "Creating replica set..."

mongo --host mongostore_replicaset <<EOF
rs.initiate(
  {
    _id : 'rs0',
    members: [ { _id : 0, host : "mongostore_replicaset:27017" }]
  }
)
EOF

echo "Replica set created."

sleep infinity
