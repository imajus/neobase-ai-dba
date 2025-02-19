import { AlertCircle, ChevronDown, X } from 'lucide-react';
import React, { useState } from 'react';
import { Chat, Connection } from '../../types/chat';

interface ConnectionModalProps {
  initialData?: Chat;
  onClose: () => void;
  onSubmit: (data: Chat) => void;
  onEdit?: (data: Chat) => void;
}

interface FormErrors {
  host?: string;
  port?: string;
  database?: string;
  username?: string;
}

export default function ConnectionModal({
  initialData,
  onClose,
  onSubmit,
  onEdit,
}: ConnectionModalProps) {
  const [formData, setFormData] = useState<Chat>(
    initialData || {
      id: '',
      user_id: '',
      connection: {
        id: '',
        type: 'postgresql',
        host: '',
        port: '',
        username: '',
        password: '',
        database: '',
      },
      created_at: '',
      updated_at: '',
    }
  );
  const [errors, setErrors] = useState<FormErrors>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});

  const validateField = (name: string, value: Connection) => {
    switch (name) {
      case 'host':
        if (!value.host.trim()) {
          return 'Host is required';
        }
        if (!/^[a-zA-Z0-9.-]+$/.test(value.host)) {
          return 'Invalid host format';
        }
        break;
      case 'port':
        if (!value.port.trim()) {
          return 'Port is required';
        }
        const port = parseInt(value.port);
        if (isNaN(port) || port < 1 || port > 65535) {
          return 'Port must be between 1 and 65535';
        }
        break;
      case 'database':
        if (!value.database.trim()) {
          return 'Database name is required';
        }
        if (!/^[a-zA-Z0-9_-]+$/.test(value.database)) {
          return 'Invalid database name format';
        }
        break;
      case 'username':
        if (!value.username.trim()) {
          return 'Username is required';
        }
        if (!/^[a-zA-Z0-9_-]+$/.test(value.username)) {
          return 'Invalid username format';
        }
        break;
      default:
        return '';
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    // Validate all fields
    const newErrors: FormErrors = {};
    let hasErrors = false;

    ['host', 'port', 'database', 'username'].forEach(field => {
      const error = validateField(field, formData.connection);
      if (error) {
        newErrors[field as keyof FormErrors] = error;
        hasErrors = true;
      }
    });

    setErrors(newErrors);
    setTouched({
      host: true,
      port: true,
      database: true,
      username: true,
    });

    if (!hasErrors) {
      onEdit?.(formData) ?? onSubmit(formData);
    }
  };

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>
  ) => {
    const { name, value } = e.target;
    setFormData((prev) => ({
      ...prev,
      [name]: value,
    }));

    if (touched[name]) {
      const error = validateField(name, formData.connection);
      setErrors(prev => ({
        ...prev,
        [name]: error,
      }));
    }
  };

  const handleBlur = (e: React.FocusEvent<HTMLInputElement>) => {
    const { name } = e.target;
    setTouched(prev => ({
      ...prev,
      [name]: true,
    }));
    const error = validateField(name, formData.connection);
    setErrors(prev => ({
      ...prev,
      [name]: error,
    }));
  };

  return (
    <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4 z-[200]">
      <div className="bg-white neo-border rounded-lg w-full max-w-lg max-h-[90vh] overflow-y-auto relative z-[201]">
        <div className="flex justify-between items-center p-6 border-b-4 border-black mb-2">
          <h2 className="text-2xl font-bold">{onEdit ? 'Edit Database Connection' : 'New Database Connection'}</h2>
          <button
            onClick={onClose}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-6">
          <div>
            <label className="block font-bold mb-2 text-lg">Database Type</label>
            <p className="text-gray-600 text-sm mb-2">Select your database system</p>
            <div className="relative">
              <select
                name="type"
                value={formData.connection.type}
                onChange={handleChange}
                className="neo-input w-full appearance-none pr-12"
              >
                {[
                  { value: 'postgresql', label: 'PostgreSQL' },
                  { value: 'mysql', label: 'MySQL' },
                  { value: 'clickhouse', label: 'ClickHouse' },
                  { value: 'mongodb', label: 'MongoDB' },
                  { value: 'redis', label: 'Redis' },
                  { value: 'neo4j', label: 'Neo4J' }
                ].map(option => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
              <div className="absolute inset-y-0 right-0 flex items-center pr-4 pointer-events-none">
                <ChevronDown className="w-5 h-5 text-gray-400" />
              </div>
            </div>
          </div>

          <div>
            <label className="block font-bold mb-2 text-lg">Host</label>
            <p className="text-gray-600 text-sm mb-2">The hostname or IP address of your database server</p>
            <input
              type="text"
              name="host"
              value={formData.connection.host}
              onChange={handleChange}
              onBlur={handleBlur}
              className={`neo-input w-full ${errors.host && touched.host ? 'border-neo-error' : ''}`}
              placeholder="e.g., localhost, db.example.com, 192.168.1.1"
              required
            />
            {errors.host && touched.host && (
              <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                <AlertCircle className="w-4 h-4" />
                <span>{errors.host}</span>
              </div>
            )}
          </div>

          <div>
            <label className="block font-bold mb-2 text-lg">Port</label>
            <p className="text-gray-600 text-sm mb-2">The port number your database is listening on</p>
            <input
              type="text"
              name="port"
              value={formData.connection.port}
              onChange={handleChange}
              onBlur={handleBlur}
              className={`neo-input w-full ${errors.port && touched.port ? 'border-neo-error' : ''}`}
              placeholder="e.g., 5432 (PostgreSQL), 3306 (MySQL), 27017 (MongoDB)"
              required
            />
            {errors.port && touched.port && (
              <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                <AlertCircle className="w-4 h-4" />
                <span>{errors.port}</span>
              </div>
            )}
          </div>

          <div>
            <label className="block font-bold mb-2 text-lg">Database Name</label>
            <p className="text-gray-600 text-sm mb-2">The name of the specific database to connect to</p>
            <input
              type="text"
              name="database"
              value={formData.connection.database}
              onChange={handleChange}
              onBlur={handleBlur}
              className={`neo-input w-full ${errors.database && touched.database ? 'border-neo-error' : ''}`}
              placeholder="e.g., myapp_production, users_db"
              required
            />
            {errors.database && touched.database && (
              <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                <AlertCircle className="w-4 h-4" />
                <span>{errors.database}</span>
              </div>
            )}
          </div>

          <div>
            <label className="block font-bold mb-2 text-lg">Username</label>
            <p className="text-gray-600 text-sm mb-2">Database user with appropriate permissions</p>
            <input
              type="text"
              name="username"
              value={formData.connection.username}
              onChange={handleChange}
              onBlur={handleBlur}
              className={`neo-input w-full ${errors.username && touched.username ? 'border-neo-error' : ''}`}
              placeholder="e.g., db_user, assistant"
              required
            />
            {errors.username && touched.username && (
              <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                <AlertCircle className="w-4 h-4" />
                <span>{errors.username}</span>
              </div>
            )}
          </div>

          <div>
            <label className="block font-bold mb-2 text-lg">Password</label>
            <p className="text-gray-600 text-sm mb-2">Password for the database user</p>
            <input
              type="password"
              name="password"
              value={formData.connection.password}
              onChange={handleChange}
              className="neo-input w-full"
              placeholder="Enter your database password"
              required
            />
          </div>

          <div className="flex gap-4 pt-4">
            <button type="submit" className="neo-button flex-1">
              {onEdit ? 'Save & Reconnect' : 'Connect'}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="neo-button-secondary flex-1"
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}