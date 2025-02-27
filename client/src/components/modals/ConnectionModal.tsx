import { AlertCircle, ChevronDown, Database, Loader2, X } from 'lucide-react';
import React, { useEffect, useRef, useState } from 'react';
import { Chat, Connection } from '../../types/chat';

interface ConnectionModalProps {
  initialData?: Chat;
  onClose: () => void;
  onEdit?: (data: Connection) => Promise<{ success: boolean, error?: string }>;
  onSubmit: (data: Connection) => Promise<{ success: boolean, error?: string }>;
}

interface FormErrors {
  host?: string;
  port?: string;
  database?: string;
  username?: string;
}

export default function ConnectionModal({ initialData, onClose, onEdit, onSubmit }: ConnectionModalProps) {
  const [isLoading, setIsLoading] = useState(false);
  const [formData, setFormData] = useState<Connection>({
    type: initialData?.connection.type || 'postgresql',
    host: initialData?.connection.host || '',
    port: initialData?.connection.port || '',
    username: initialData?.connection.username || '',
    password: '',  // Password is never sent back from server
    database: initialData?.connection.database || ''
  });
  const [errors, setErrors] = useState<FormErrors>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [error, setError] = useState<string | null>(null);

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

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);

    // Validate all fields first
    const newErrors: FormErrors = {};
    let hasErrors = false;

    ['host', 'port', 'database', 'username'].forEach(field => {
      const error = validateField(field, formData);
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

    if (hasErrors) {
      setIsLoading(false);
      return;
    }

    try {
      if (initialData) {
        const result = await onEdit?.(formData);
        console.log("edit result in connection modal", result);
        if (result?.success) {
          onClose();
        } else if (result?.error) {
          setError(result.error);
        }
      } else {
        const result = await onSubmit(formData);
        console.log("submit result in connection modal", result);
        if (result?.success) {
          onClose();
        } else if (result?.error) {
          setError(result.error);
        }
      }
    } catch (err: any) {
      setError(err.message || 'An error occurred while updating the connection');
    } finally {
      setIsLoading(false);
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
      const error = validateField(name, {
        ...formData,
        [name]: value,
      });
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
    const error = validateField(name, formData);
    setErrors(prev => ({
      ...prev,
      [name]: error,
    }));
  };

  const parseConnectionString = (text: string): Partial<Connection> => {
    const result: Partial<Connection> = {};
    const lines = text.split('\n');

    lines.forEach(line => {
      const [key, value] = line.split('=').map(s => s.trim());
      switch (key) {
        case 'DATABASE_TYPE':
          if (['postgresql', 'yugabytedb', 'mysql', 'clickhouse', 'mongodb', 'redis', 'neo4j'].includes(value)) {
            result.type = value as 'postgresql' | 'yugabytedb' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j';
          }
          break;
        case 'DATABASE_HOST':
          result.host = value;
          break;
        case 'DATABASE_PORT':
          result.port = value;
          break;
        case 'DATABASE_NAME':
          result.database = value;
          break;
        case 'DATABASE_USERNAME':
          result.username = value;
          break;
        case 'DATABASE_PASSWORD':
          result.password = value;
          break;
      }
    });
    return result;
  };

  // Add this helper function to format connection details
  const formatConnectionString = (connection: Connection): string => {
    return `DATABASE_TYPE=${connection.type}
DATABASE_HOST=${connection.host}
DATABASE_PORT=${connection.port}
DATABASE_NAME=${connection.database}
DATABASE_USERNAME=${connection.username}
DATABASE_PASSWORD=`; // Mask password
  };

  // Add a ref for the textarea
  const credentialsTextAreaRef = useRef<HTMLTextAreaElement>(null);

  // In the component, add useEffect to populate textarea in edit mode
  useEffect(() => {
    if (initialData && credentialsTextAreaRef.current) {
      credentialsTextAreaRef.current.value = formatConnectionString(initialData.connection);
    }
  }, [initialData]);

  return (
    <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4 z-[200]">
      <div className="bg-white neo-border rounded-lg w-full max-w-lg max-h-[90vh] overflow-y-auto relative z-[201]">
        <div className="flex justify-between items-center p-6 border-b-4 border-black mb-2">
          <div className="flex items-center gap-2">
            <Database className="w-6 h-6" />
            <h2 className="text-2xl font-bold">{initialData ? 'Edit Connection' : 'New Connection'}</h2>
          </div>
          <button
            onClick={onClose}
            className="hover:bg-neo-gray rounded-lg p-2 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-6">
          {error && (
            <div className="p-4 bg-red-50 border-2 border-red-500 rounded-lg">
              <div className="flex items-center gap-2 text-red-600">
                <AlertCircle className="w-5 h-5" />
                <p className="font-medium">{error}</p>
              </div>
            </div>
          )}

          <div>
            <label className="block font-bold mb-2 text-lg">Paste Credentials</label>
            <p className="text-gray-600 text-sm mb-2">
              Paste your database credentials in the following format:
            </p>
            <textarea
              ref={credentialsTextAreaRef}
              className="neo-input w-full font-mono text-sm"
              placeholder={`DATABASE_TYPE=postgresql
DATABASE_HOST=your-host.example.com
DATABASE_PORT=5432
DATABASE_NAME=your_database
DATABASE_USERNAME=your_username
DATABASE_PASSWORD=your_password`}
              rows={6}
              onChange={(e) => {
                const parsed = parseConnectionString(e.target.value);
                setFormData(prev => ({
                  ...prev,
                  ...parsed,
                  // Keep existing password if we're editing and no new password provided
                  password: parsed.password || (initialData ? formData.password : '')
                }));
                // Clear any errors for fields that were filled
                const newErrors = { ...errors };
                Object.keys(parsed).forEach(key => {
                  delete newErrors[key as keyof FormErrors];
                });
                setErrors(newErrors);
                // Mark fields as touched
                const newTouched = { ...touched };
                Object.keys(parsed).forEach(key => {
                  newTouched[key] = true;
                });
                setTouched(newTouched);
              }}
            />
            <p className="text-gray-500 text-xs mt-2">
              All the fields will be automatically filled based on the pasted credentials
            </p>
          </div>
          <div className="my-6 border-t border-gray-200"></div>
          <div>
            <label className="block font-bold mb-2 text-lg">Database Type</label>
            <p className="text-gray-600 text-sm mb-2">Select your database system</p>
            <div className="relative">
              <select
                name="type"
                value={formData.type}
                onChange={handleChange}
                className="neo-input w-full appearance-none pr-12"
              >
                {[
                  { value: 'postgresql', label: 'PostgreSQL' },
                  { value: 'yugabytedb', label: 'YugabyteDB' },
                  { value: 'mysql', label: 'MySQL (Coming Soon)' },
                  { value: 'clickhouse', label: 'ClickHouse (Coming Soon)' },
                  { value: 'mongodb', label: 'MongoDB (Coming Soon)' },
                  { value: 'redis', label: 'Redis (Coming Soon)' },
                  { value: 'neo4j', label: 'Neo4J (Coming Soon)' }
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
              value={formData.host}
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
              value={formData.port}
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
              value={formData.database}
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
              value={formData.username}
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
              value={formData.password}
              onChange={handleChange}
              className="neo-input w-full"
              placeholder="Enter your database password"
              required
            />
          </div>

          <div className="flex gap-4 pt-4">
            <button
              type="submit"
              className="neo-button flex-1 relative"
              disabled={isLoading}
            >
              {isLoading ? (
                <div className="flex items-center justify-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>{initialData ? 'Updating...' : 'Creating...'}</span>
                </div>
              ) : (
                <span>{initialData ? 'Update' : 'Create'}</span>
              )}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="neo-button-secondary flex-1"
              disabled={isLoading}
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}