#!/bin/bash

echo "鍚姩鍩虹璁炬�?..."
docker-compose -f scripts/docker-compose.yml up -d

echo "绛夊緟鏈嶅姟鍚�?..."
sleep 30

echo "鍒濆鍖栨暟鎹�?..."
mysql -h 127.0.0.1 -P 3306 -u root -proot < scripts/init_db.sql

echo "鍒涘缓Kafka涓婚�?..."
docker exec -it $(docker ps -qf "name=kafka") kafka-topics --create --topic order.history --bootstrap-server localhost:9092 --partitions 3 --replication-factor 1
docker exec -it $(docker ps -qf "name=kafka") kafka-topics --create --topic order.create --bootstrap-server localhost:9092 --partitions 3 --replication-factor 1
docker exec -it $(docker ps -qf "name=kafka") kafka-topics --create --topic order.paid --bootstrap-server localhost:9092 --partitions 3 --replication-factor 1

echo "鍒濆鍖朢edis闆嗙�?..."
docker exec -it $(docker ps -qf "name=redis-node-1") redis-cli --cluster create 127.0.0.1:7000 127.0.0.1:7001 127.0.0.1:7002 --cluster-replicas 0

echo "閮ㄧ讲瀹屾�?"
echo "Consul: http://localhost:8500"
echo "Prometheus: http://localhost:9090"
echo "Grafana: http://localhost:3000 (admin/admin)"
echo "Jaeger: http://localhost:16686"
