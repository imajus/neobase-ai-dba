import React from 'react';

interface DatabaseLogoProps {
  type: 'postgresql' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j';
  size?: number;
  className?: string;
}

const databaseLogos: Record<DatabaseLogoProps['type'], string> = {
  postgresql: 'https://www.postgresql.org/media/img/about/press/elephant.png',
  mysql: 'https://labs.mysql.com/common/logos/mysql-logo.svg',
  mongodb: 'https://www.mongodb.com/assets/images/global/leaf.png',
  redis: 'https://redis.io/images/redis-white.png',
  clickhouse: 'https://clickhouse.com/images/ch_logo.svg',
  neo4j: 'https://dist.neo4j.com/wp-content/uploads/20230926085635/neo4j-logo-2023.svg'
};

export default function DatabaseLogo({ type, size = 24, className = '' }: DatabaseLogoProps) {
  return (
    <div 
      className={`relative flex items-center justify-center ${className}`}
      style={{ width: size, height: size }}
    >
      <img
        src={databaseLogos[type]}
        alt={`${type} database logo`}
        className="w-full h-full object-contain"
        onError={(e) => {
          // Fallback to a generic database icon if the logo fails to load
          e.currentTarget.style.display = 'none';
          const parent = e.currentTarget.parentElement;
          if (parent) {
            parent.innerHTML = `<svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              className="w-full h-full"
            >
              <path d="M4 7c0-1.1.9-2 2-2h12a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V7z" />
              <path d="M4 7h16" />
              <path d="M4 11h16" />
              <path d="M4 15h16" />
            </svg>`;
          }
        }}
      />
    </div>
  );
}