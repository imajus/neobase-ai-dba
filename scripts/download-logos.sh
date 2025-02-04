#!/bin/bash

# Create public directory if it doesn't exist
mkdir -p public

# Download logos
curl -o public/postgresql-logo.png "https://www.postgresql.org/media/img/about/press/elephant.png"
curl -o public/mysql-logo.svg "https://www.mysql.com/common/logos/logo-mysql-170x115.png"
curl -o public/mongodb-logo.png "https://www.mongodb.com/assets/images/global/leaf.png"
curl -o public/neo4j-logo.png "https://dist.neo4j.com/wp-content/uploads/20230926085635/neo4j-logo-2023.svg"

# Redis and ClickHouse logos are already provided 