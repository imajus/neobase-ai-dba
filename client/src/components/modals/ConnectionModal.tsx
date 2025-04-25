import { AlertCircle, ChevronDown, Database, KeyRound, Loader2, Monitor, Shield, Table, X } from 'lucide-react';
import React, { useEffect, useRef, useState } from 'react';
import { Chat, Connection, SSLMode } from '../../types/chat';
import SelectTablesModal from './SelectTablesModal';
import chatService from '../../services/chatService';

// Connection tab type
type ConnectionType = 'basic' | 'ssh';

interface ConnectionModalProps {
  initialData?: Chat;
  onClose: () => void;
  onEdit?: (data: Connection, autoExecuteQuery: boolean) => Promise<{ success: boolean, error?: string }>;
  onSubmit: (data: Connection, autoExecuteQuery: boolean) => Promise<{ 
    success: boolean;
    error?: string;
    chatId?: string;
    selectedCollections?: string;
  }>;
  onUpdateSelectedCollections?: (chatId: string, selectedCollections: string) => Promise<void>;
  onUpdateAutoExecuteQuery?: (chatId: string, autoExecuteQuery: boolean) => Promise<void>;
}

interface FormErrors {
  host?: string;
  port?: string;
  database?: string;
  username?: string;
  ssl_cert_url?: string;
  ssl_key_url?: string;
  ssl_root_cert_url?: string;
  ssh_host?: string;
  ssh_port?: string;
  ssh_username?: string;
  ssh_private_key?: string;
}

export default function ConnectionModal({ 
  initialData, 
  onClose, 
  onEdit, 
  onSubmit,
  onUpdateSelectedCollections,
  onUpdateAutoExecuteQuery
}: ConnectionModalProps) {
  // Add connection type state to toggle between basic and SSH tabs
  const [connectionType, setConnectionType] = useState<ConnectionType>('basic');
  const [isLoading, setIsLoading] = useState(false);
  const [formData, setFormData] = useState<Connection>({
    type: initialData?.connection.type || 'postgresql',
    host: initialData?.connection.host || '',
    port: initialData?.connection.port || '',
    username: initialData?.connection.username || '',
    password: '',  // Password is never sent back from server
    database: initialData?.connection.database || '',
    use_ssl: initialData?.connection.use_ssl || false,
    ssl_mode: initialData?.connection.ssl_mode || 'disable',
    ssl_cert_url: initialData?.connection.ssl_cert_url || '',
    ssl_key_url: initialData?.connection.ssl_key_url || '',
    ssl_root_cert_url: initialData?.connection.ssl_root_cert_url || '',
    ssh_enabled: initialData?.connection.ssh_enabled || false,
    ssh_host: initialData?.connection.ssh_host || '',
    ssh_port: initialData?.connection.ssh_port || '22',
    ssh_username: initialData?.connection.ssh_username || '',
    ssh_private_key: initialData?.connection.ssh_private_key || '',
    ssh_passphrase: initialData?.connection.ssh_passphrase || '',
    is_example_db: false
  });
  const [errors, setErrors] = useState<FormErrors>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [error, setError] = useState<string | null>(null);
  const [showSelectTablesModal, setShowSelectTablesModal] = useState(false);
  const [autoExecuteQuery, setAutoExecuteQuery] = useState<boolean>(
    initialData?.auto_execute_query !== undefined ? initialData.auto_execute_query : true
  );

  // Update autoExecuteQuery when initialData changes
  useEffect(() => {
    if (initialData) {
      if (initialData.auto_execute_query !== undefined) {
        setAutoExecuteQuery(initialData.auto_execute_query);
      }
      
      // Set the connection type tab based on whether SSH is enabled
      if (initialData.connection.ssh_enabled) {
        setConnectionType('ssh');
      } else {
        setConnectionType('basic');
      }
    }
  }, [initialData]);

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
        // For MongoDB, port is optional and can be empty
        if (value.type === 'mongodb') {
          return '';
        }
        if (!value.port) {
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
        break;
      case 'ssl_cert_url':
        if (value.use_ssl && value.ssl_mode !== 'disable' && value.ssl_mode !== 'require' && !value.ssl_cert_url?.trim()) {
          return 'SSL Certificate URL is required for this SSL mode';
        }
        if (value.ssl_cert_url && !isValidUrl(value.ssl_cert_url)) {
          return 'Invalid URL format';
        }
        break;
      case 'ssl_key_url':
        if (value.use_ssl && value.ssl_mode !== 'disable' && value.ssl_mode !== 'require' && !value.ssl_key_url?.trim()) {
          return 'SSL Key URL is required for this SSL mode';
        }
        if (value.ssl_key_url && !isValidUrl(value.ssl_key_url)) {
          return 'Invalid URL format';
        }
        break;
      case 'ssl_root_cert_url':
        if (value.use_ssl && value.ssl_mode !== 'disable' && value.ssl_mode !== 'require' && !value.ssl_root_cert_url?.trim()) {
          return 'SSL Root Certificate URL is required for this SSL mode';
        }
        if (value.ssl_root_cert_url && !isValidUrl(value.ssl_root_cert_url)) {
          return 'Invalid URL format';
        }
        break;
      // SSH validation
      case 'ssh_host':
        if (value.ssh_enabled && !value.ssh_host?.trim()) {
          return 'SSH Host is required';
        }
        if (value.ssh_host && !/^[a-zA-Z0-9.-]+$/.test(value.ssh_host)) {
          return 'Invalid SSH host format';
        }
        break;
      case 'ssh_port':
        if (value.ssh_enabled && !value.ssh_port) {
          return 'SSH Port is required';
        }
        if (value.ssh_port) {
          const sshPort = parseInt(value.ssh_port);
          if (isNaN(sshPort) || sshPort < 1 || sshPort > 65535) {
            return 'SSH Port must be between 1 and 65535';
          }
        }
        break;
      case 'ssh_username':
        if (value.ssh_enabled && !value.ssh_username?.trim()) {
          return 'SSH Username is required';
        }
        break;
      case 'ssh_private_key':
        if (value.ssh_enabled && !value.ssh_private_key?.trim()) {
          return 'SSH Private Key is required';
        }
        break;
      default:
        return '';
    }
  };

  // Helper function to validate URLs
  const isValidUrl = (url: string): boolean => {
    try {
      new URL(url);
      return true;
    } catch (e) {
      return false;
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);

    // Update ssh_enabled based on current tab
    const updatedFormData = {
      ...formData,
      ssh_enabled: connectionType === 'ssh'
    };
    setFormData(updatedFormData);

    // Validate all fields first
    const newErrors: FormErrors = {};
    let hasErrors = false;

    // Always validate these fields
    ['host', 'port', 'database', 'username'].forEach(field => {
      const error = validateField(field, updatedFormData);
      if (error) {
        newErrors[field as keyof FormErrors] = error;
        hasErrors = true;
      }
    });

    // Validate SSL fields if SSL is enabled in Basic mode
    if (connectionType === 'basic' && updatedFormData.use_ssl) {
      // For verify-ca and verify-full modes, we need certificates
      if (['verify-ca', 'verify-full'].includes(updatedFormData.ssl_mode || '')) {
        ['ssl_cert_url', 'ssl_key_url', 'ssl_root_cert_url'].forEach(field => {
          const error = validateField(field, updatedFormData);
          if (error) {
            newErrors[field as keyof FormErrors] = error;
            hasErrors = true;
          }
        });
      }
    }

    // Validate SSH fields if SSH tab is active
    if (connectionType === 'ssh') {
      ['ssh_host', 'ssh_port', 'ssh_username', 'ssh_private_key'].forEach(field => {
        const error = validateField(field, updatedFormData);
        if (error) {
          newErrors[field as keyof FormErrors] = error;
          hasErrors = true;
        }
      });
    }

    setErrors(newErrors);
    setTouched({
      host: true,
      port: true,
      database: true,
      username: true,
      ...(updatedFormData.use_ssl && connectionType === 'basic' ? {
        ssl_cert_url: true,
        ssl_key_url: true,
        ssl_root_cert_url: true
      } : {}),
      ...(connectionType === 'ssh' ? {
        ssh_host: true,
        ssh_port: true,
        ssh_username: true,
        ssh_private_key: true
      } : {})
    });

    if (hasErrors) {
      setIsLoading(false);
      return;
    }

    try {
      if (initialData) {
        // Check if critical connection details have changed
        const credentialsChanged = 
          initialData.connection.database !== updatedFormData.database ||
          initialData.connection.host !== updatedFormData.host ||
          initialData.connection.port !== updatedFormData.port ||
          initialData.connection.username !== updatedFormData.username;

        const result = await onEdit?.(updatedFormData, autoExecuteQuery);
        console.log("edit result in connection modal", result);
        if (result?.success) {
          // Update auto_execute_query if it has changed
          if (initialData.auto_execute_query !== autoExecuteQuery && onUpdateAutoExecuteQuery) {
            await onUpdateAutoExecuteQuery(initialData.id, autoExecuteQuery);
          }

          // If credentials changed, show the select tables modal
          if (credentialsChanged) {
            setShowSelectTablesModal(true);
          } else {
            onClose();
          }
        } else if (result?.error) {
          setError(result.error);
        }
      } else {
        // For new connections, pass autoExecuteQuery to onSubmit
        const result = await onSubmit(updatedFormData, autoExecuteQuery);
        console.log("submit result in connection modal", result);
        if (result?.success) {
          // If this is a new connection and the selected_collections is "ALL", refresh the schema
          if (result.chatId && result.selectedCollections === 'ALL') {
            try {
              const abortController = new AbortController();
              await chatService.refreshSchema(result.chatId, abortController);
              console.log('Knowledge base refreshed successfully for new connection');
            } catch (error) {
              console.error('Failed to refresh knowledge base:', error);
            }
          }
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

  const handleUpdateSelectedCollections = async (selectedCollections: string) => {
    if (initialData && onUpdateSelectedCollections) {
      await onUpdateSelectedCollections(initialData.id, selectedCollections);
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
        case 'USE_SSL':
          result.use_ssl = value.toLowerCase() === 'true';
          break;
        case 'SSL_MODE':
          if (['disable', 'require', 'verify-ca', 'verify-full'].includes(value)) {
            result.ssl_mode = value as SSLMode;
          }
          break;
        case 'SSL_CERT_URL':
          result.ssl_cert_url = value;
          break;
        case 'SSL_KEY_URL':
          result.ssl_key_url = value;
          break;
        case 'SSL_ROOT_CERT_URL':
          result.ssl_root_cert_url = value;
          break;
        case 'SSH_ENABLED':
          result.ssh_enabled = value.toLowerCase() === 'true';
          break;
        case 'SSH_HOST':
          result.ssh_host = value;
          break;
        case 'SSH_PORT':
          result.ssh_port = value;
          break;
        case 'SSH_USERNAME':
          result.ssh_username = value;
          break;
        case 'SSH_PRIVATE_KEY':
          result.ssh_private_key = value;
          break;
        case 'SSH_PASSPHRASE':
          result.ssh_passphrase = value;
          break;
      }
    });
    return result;
  };

  const formatConnectionString = (connection: Connection): string => {
    let result = `DATABASE_TYPE=${connection.type}
DATABASE_HOST=${connection.host}
DATABASE_PORT=${connection.port}
DATABASE_NAME=${connection.database}
DATABASE_USERNAME=${connection.username}
DATABASE_PASSWORD=`; // Mask password

    // Add SSL configuration if enabled
    if (connection.use_ssl) {
      result += `\nUSE_SSL=true`;
      result += `\nSSL_MODE=${connection.ssl_mode || 'disable'}`;
      
      if (connection.ssl_cert_url) {
        result += `\nSSL_CERT_URL=${connection.ssl_cert_url}`;
      }
      
      if (connection.ssl_key_url) {
        result += `\nSSL_KEY_URL=${connection.ssl_key_url}`;
      }
      
      if (connection.ssl_root_cert_url) {
        result += `\nSSL_ROOT_CERT_URL=${connection.ssl_root_cert_url}`;
      }
    }
    
    // Add SSH configuration if enabled
    if (connection.ssh_enabled) {
      result += `\nSSH_ENABLED=true`;
      result += `\nSSH_HOST=${connection.ssh_host || ''}`;
      result += `\nSSH_PORT=${connection.ssh_port || '22'}`;
      result += `\nSSH_USERNAME=${connection.ssh_username || ''}`;
      result += `\nSSH_PRIVATE_KEY=`; // Mask private key
      
      if (connection.ssh_passphrase) {
        result += `\nSSH_PASSPHRASE=`; // Mask passphrase
      }
    }
    
    return result;
  };

  // Add a ref for the textarea
  const credentialsTextAreaRef = useRef<HTMLTextAreaElement>(null);

  // Add a new ref for MongoDB URI input fields
  const mongoUriInputRef = useRef<HTMLInputElement>(null);
  const mongoUriSshInputRef = useRef<HTMLInputElement>(null);

  // In the component, modify the useEffect to also populate MongoDB URI fields in edit mode
  useEffect(() => {
    if (initialData) {
      if (credentialsTextAreaRef.current) {
        credentialsTextAreaRef.current.value = formatConnectionString(initialData.connection);
      }
      
      // Populate MongoDB URI field if type is mongodb
      if (initialData.connection.type === 'mongodb') {
        const formatMongoURI = (connection: Connection): string => {
          const auth = connection.username ? 
            `${connection.username}${connection.password ? `:${connection.password}` : ''}@` : '';
          const srv = connection.host.includes('.mongodb.net') ? '+srv' : '';
          const portPart = srv ? '' : `:${connection.port || '27017'}`;
          const dbPart = connection.database ? `/${connection.database}` : '';
          
          return `mongodb${srv}://${auth}${connection.host}${portPart}${dbPart}`;
        };

        const mongoUri = formatMongoURI(initialData.connection);
        
        // Set the value for both URI inputs (basic and SSH tabs)
        if (mongoUriInputRef.current) {
          mongoUriInputRef.current.value = mongoUri;
        }
        
        if (mongoUriSshInputRef.current) {
          mongoUriSshInputRef.current.value = mongoUri;
        }
      }
    }
  }, [initialData]);

  return (
    <>
      <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4 z-[200]">
        <div className="bg-white neo-border rounded-lg w-full max-w-xl max-h-[90vh] overflow-y-auto relative z-[201]">
          <div className="flex justify-between items-center p-6 border-b-4 border-black mb-2">
            <div className="flex items-center gap-2">
              <Database className="w-6 h-6" />
              <div className="flex flex-col gap-1 mt-2">
                <h2 className="text-2xl font-bold">{initialData ? 'Edit Connection' : 'New Connection'}</h2>
                <p className="text-gray-500 text-sm">Your database credentials are stored in encrypted form.</p>
              </div>
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
DATABASE_PASSWORD=your_password
USE_SSL=false
SSL_MODE=disable
SSL_CERT_URL=https://example.com/cert.pem
SSL_KEY_URL=https://example.com/key.pem
SSL_ROOT_CERT_URL=https://example.com/ca.pem
SSH_ENABLED=false
SSH_HOST=ssh.example.com
SSH_PORT=22
SSH_USERNAME=ssh_user
SSH_PRIVATE_KEY=your_private_key`}
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
                  
                  // Set the connection type tab based on SSH enabled
                  if (parsed.ssh_enabled) {
                    setConnectionType('ssh');
                  }
                }}
              />
              <p className="text-gray-500 text-xs mt-2">
                All the fields will be automatically filled based on the pasted credentials
              </p>
            </div>
            
            <div className="my-6 border-t border-gray-200"></div>
            
            {/* Connection type tabs */}
            <div className="flex border-b border-gray-200 mb-6">
              <button
                type="button"
                className={`py-2 px-4 font-semibold border-b-2 ${
                  connectionType === 'basic'
                    ? 'border-black text-black'
                    : 'border-transparent text-gray-500 hover:text-gray-700'
                }`}
                onClick={() => setConnectionType('basic')}
              >
                <div className="flex items-center gap-2">
                  <Monitor className="w-4 h-4" />
                  <span>Basic Connection</span>
                </div>
              </button>
              <button
                type="button"
                className={`py-2 px-4 font-semibold border-b-2 ${
                  connectionType === 'ssh'
                    ? 'border-black text-black'
                    : 'border-transparent text-gray-500 hover:text-gray-700'
                }`}
                onClick={() => setConnectionType('ssh')}
              >
                <div className="flex items-center gap-2">
                  <KeyRound className="w-4 h-4" />
                  <span>SSH Tunnel</span>
                </div>
              </button>
            </div>

            <div className="my-6">
              <div className="flex items-center justify-between">
                <div>
                  <label className="block font-bold mb-1 text-lg">Auto Fetch Results</label>
                  <p className="text-gray-600 text-sm">Automatically fetches results from the database upon a user request. However, the critical queries still need to be executed manually by the user.</p>
                </div>
                <label className="relative inline-flex items-center cursor-pointer">
                  <input 
                    type="checkbox" 
                    className="sr-only peer" 
                    checked={autoExecuteQuery}
                    onChange={(e) => {
                      const newValue = e.target.checked;
                      setAutoExecuteQuery(newValue);
                      if (initialData && onUpdateAutoExecuteQuery) {
                        onUpdateAutoExecuteQuery(initialData.id, newValue);
                      }
                    }}
                  />
                  <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
                </label>
              </div>
            </div>

            {/* Add Select Tables button for edit mode */}
            {initialData && onUpdateSelectedCollections && (
              <div className="my-6 pt-4 border-t border-gray-200">
                <button
                  type="button"
                  onClick={() => setShowSelectTablesModal(true)}
                  className="neo-button-secondary w-full flex items-center justify-center gap-2"
                >
                  <Table className="w-5 h-5" />
                  <span>Select Tables/Collections</span>
                </button>
                <p className="text-gray-500 text-xs mt-2 text-center">
                  Choose which tables to include in your database schema
                </p>
              </div>
            )}

            {/* Basic Connection Tab */}
            {connectionType === 'basic' && (
              <>
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
                        { value: 'mysql', label: 'MySQL' },
                        { value: 'clickhouse', label: 'ClickHouse' },
                        { value: 'mongodb', label: 'MongoDB' },
                        { value: 'cassandra', label: 'Cassandra (Coming Soon)' },
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

                {/* MongoDB Connection URI Field - Only show when MongoDB is selected */}
                {formData.type === 'mongodb' && (
                  <div>
                    <label className="block font-bold mb-2 text-lg">MongoDB Connection URI</label>
                    <p className="text-gray-600 text-sm mb-2">Paste your MongoDB connection string to auto-fill fields</p>
                    <input
                      type="text"
                      name="mongo_uri"
                      ref={mongoUriInputRef}
                      className="neo-input w-full"
                      placeholder="mongodb://username:password@host:port/database or mongodb+srv://username:password@host/database"
                      onChange={(e) => {
                        const uri = e.target.value;
                        try {
                          // Better parsing logic for MongoDB URIs that can handle special characters in credentials
                          const srvFormat = uri.startsWith('mongodb+srv://');
                          
                          // Extract the protocol and the rest
                          const protocolMatch = uri.match(/^(mongodb(?:\+srv)?:\/\/)(.*)/);
                          if (!protocolMatch) {
                            console.log("Invalid MongoDB URI format: Missing protocol");
                            return;
                          }
                          
                          const [, protocol, remainder] = protocolMatch;
                          
                          // Check if credentials are provided (look for @ after the protocol)
                          const hasCredentials = remainder.includes('@');
                          let username = '';
                          let password = '';
                          let hostPart = remainder;
                          
                          if (hasCredentials) {
                            // Find the last @ which separates credentials from host
                            const lastAtIndex = remainder.lastIndexOf('@');
                            const credentialsPart = remainder.substring(0, lastAtIndex);
                            hostPart = remainder.substring(lastAtIndex + 1);
                            
                            // Find the first : which separates username from password
                            const firstColonIndex = credentialsPart.indexOf(':');
                            if (firstColonIndex !== -1) {
                              username = credentialsPart.substring(0, firstColonIndex);
                              password = credentialsPart.substring(firstColonIndex + 1);
                            } else {
                              username = credentialsPart;
                            }
                          }
                          
                          // Parse host, port and database
                          let host = '';
                          let port = srvFormat ? '27017' : ''; // Default for SRV format
                          let database = 'test'; // Default database name
                          
                          // Check if there's a / after the host[:port] part
                          const pathIndex = hostPart.indexOf('/');
                          if (pathIndex !== -1) {
                            const hostPortPart = hostPart.substring(0, pathIndex);
                            const pathPart = hostPart.substring(pathIndex + 1);
                            
                            // Extract database name (everything before ? or end of string)
                            const dbEndIndex = pathPart.indexOf('?');
                            if (dbEndIndex !== -1) {
                              database = pathPart.substring(0, dbEndIndex);
                            } else {
                              database = pathPart;
                            }
                            
                            // Parse host and port
                            const portIndex = hostPortPart.indexOf(':');
                            if (portIndex !== -1) {
                              host = hostPortPart.substring(0, portIndex);
                              port = hostPortPart.substring(portIndex + 1);
                            } else {
                              host = hostPortPart;
                            }
                          } else {
                            // No database specified in the URI
                            const portIndex = hostPart.indexOf(':');
                            if (portIndex !== -1) {
                              host = hostPart.substring(0, portIndex);
                              port = hostPart.substring(portIndex + 1);
                            } else {
                              host = hostPart;
                            }
                          }
                          
                          if (host) {
                            console.log("MongoDB URI parsed successfully", { username, host, port, database });
                            
                            setFormData(prev => ({
                              ...prev,
                              host: host,
                              port: port || (srvFormat ? '27017' : prev.port),
                              database: database || 'test',
                              username: username || prev.username,
                              password: password || prev.password
                            }));
                            
                            // Mark fields as touched
                            setTouched(prev => ({
                              ...prev,
                              host: true,
                              port: !!port,
                              database: !!database,
                              username: !!username
                            }));
                          } else {
                            console.log("MongoDB URI parsing failed: could not extract host");
                          }
                        } catch (err) {
                          // Invalid URI format, just continue
                          console.log("Invalid MongoDB URI format", err);
                        }
                      }}
                    />
                    <p className="text-gray-500 text-xs mt-2">
                      Connection URI will be used to auto-fill the fields below. Both standard and Atlas SRV formats supported.
                    </p>
                  </div>
                )}

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

                <div className="mb-4">
                  <label className="block font-bold mb-2 text-lg">Password</label>
                  <p className="text-gray-600 text-sm mb-2">Password for the database user</p>
                  <input
                    type="password"
                    name="password"
                    value={formData.password || ''}
                    onChange={handleChange}
                    className="neo-input w-full"
                    placeholder="Enter your database password"
                  />
                  <p className="text-gray-500 text-xs mt-2">Leave blank if the database has no password, but it's recommended to set a password for the database user</p>
                </div>
 
                {/* Divider line */}
                <div className="border-t border-gray-200"></div>

                {/* SSL Toggle */}
                <div className="mb-4">
                  <label className="block font-bold mb-2 text-lg">SSL/TLS Security</label>
                  <p className="text-gray-600 text-sm mb-2">Enable secure connection to your database</p>
                  <div className="flex items-center">
                    <input
                      type="checkbox"
                      id="use_ssl"
                      name="use_ssl"
                      checked={formData.use_ssl || false}
                      onChange={(e) => {
                        const useSSL = e.target.checked;
                        setFormData((prev) => ({
                          ...prev,
                          use_ssl: useSSL,
                          // Reset SSL mode to disable if SSL is turned off
                          ssl_mode: useSSL ? prev.ssl_mode || 'disable' : 'disable'
                        }));
                        
                        // If enabling SSL, validate the SSL fields
                        if (useSSL) {
                          const newErrors = { ...errors };
                          const newTouched = { ...touched };
                          
                          if (formData.ssl_mode === 'verify-ca' || formData.ssl_mode === 'verify-full') {
                            ['ssl_cert_url', 'ssl_key_url', 'ssl_root_cert_url'].forEach(field => {
                              newTouched[field] = true;
                              const error = validateField(field, {
                                ...formData,
                                use_ssl: true
                              });
                              if (error) {
                                newErrors[field as keyof FormErrors] = error;
                              } else {
                                delete newErrors[field as keyof FormErrors];
                              }
                            });
                          }
                          
                          setErrors(newErrors);
                          setTouched(newTouched);
                        } else {
                          // If disabling SSL, clear SSL field errors
                          const newErrors = { ...errors };
                          ['ssl_cert_url', 'ssl_key_url', 'ssl_root_cert_url'].forEach(field => {
                            delete newErrors[field as keyof FormErrors];
                          });
                          setErrors(newErrors);
                        }
                      }}
                      className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
                    />
                    <label htmlFor="use_ssl" className="ml-2 block text-sm font-medium text-gray-700">
                      Use SSL/TLS encryption
                    </label>
                  </div>
                </div>

                {/* SSL Mode Selector - Only show when SSL is enabled */}
                {formData.use_ssl && (
                  <div className="mb-4">
                    <label className="block font-medium mb-2">SSL Mode</label>
                    <div className="relative">
                      <select
                        name="ssl_mode"
                        value={formData.ssl_mode || 'disable'}
                        onChange={(e) => {
                          const newMode = e.target.value as SSLMode;
                          setFormData(prev => ({
                            ...prev,
                            ssl_mode: newMode
                          }));
                          
                          // Validate certificate fields for verify-ca and verify-full modes
                          if (newMode === 'verify-ca' || newMode === 'verify-full') {
                            const newErrors = { ...errors };
                            const newTouched = { ...touched };
                            
                            ['ssl_cert_url', 'ssl_key_url', 'ssl_root_cert_url'].forEach(field => {
                              newTouched[field] = true;
                              const error = validateField(field, {
                                ...formData,
                                ssl_mode: newMode,
                                use_ssl: true
                              });
                              if (error) {
                                newErrors[field as keyof FormErrors] = error;
                              } else {
                                delete newErrors[field as keyof FormErrors];
                              }
                            });
                            
                            setErrors(newErrors);
                            setTouched(newTouched);
                          } else {
                            // For other modes, clear certificate field errors
                            const newErrors = { ...errors };
                            ['ssl_cert_url', 'ssl_key_url', 'ssl_root_cert_url'].forEach(field => {
                              delete newErrors[field as keyof FormErrors];
                            });
                            setErrors(newErrors);
                          }
                        }}
                        className="neo-input w-full appearance-none pr-12"
                      >
                        <option value="disable">Disable - No SSL</option>
                        <option value="require">Require - Encrypted only</option>
                        <option value="verify-ca">Verify CA - Verify certificate authority</option>
                        <option value="verify-full">Verify Full - Verify CA and hostname</option>
                      </select>
                      <div className="absolute inset-y-0 right-0 flex items-center pr-4 pointer-events-none">
                        <ChevronDown className="w-5 h-5 text-gray-400" />
                      </div>
                    </div>
                    <p className="text-gray-500 text-xs mt-2">
                      {formData.ssl_mode === 'disable' && 'SSL will not be used.'}
                      {formData.ssl_mode === 'require' && 'Connection must be encrypted, but certificates are not verified.'}
                      {formData.ssl_mode === 'verify-ca' && 'Connection must be encrypted and the server certificate must be verified.'}
                      {formData.ssl_mode === 'verify-full' && 'Connection must be encrypted and both the server certificate and hostname must be verified.'}
                    </p>
                  </div>
                )}

                {/* SSL Certificate Fields - Only show when SSL is enabled and mode requires verification */}
                {formData.use_ssl && (formData.ssl_mode === 'verify-ca' || formData.ssl_mode === 'verify-full') && (
                  <div className="mb-4 p-4 border border-gray-200 rounded-md bg-gray-50">
                    <h4 className="font-bold mb-3 text-md">SSL/TLS Certificate Configuration</h4>
                    
                    <div className="mb-4">
                      <label className="block font-medium mb-1 text-sm">SSL Certificate URL</label>
                      <p className="text-gray-600 text-xs mb-1">URL to your client certificate file (.pem or .crt)</p>
                      <input
                        type="text"
                        name="ssl_cert_url"
                        value={formData.ssl_cert_url || ''}
                        onChange={handleChange}
                        onBlur={handleBlur}
                        className={`neo-input w-full ${errors.ssl_cert_url && touched.ssl_cert_url ? 'border-red-500' : ''}`}
                        placeholder="https://example.com/cert.pem"
                      />
                      {errors.ssl_cert_url && touched.ssl_cert_url && (
                        <p className="text-red-500 text-xs mt-1">{errors.ssl_cert_url}</p>
                      )}
                    </div>
                    
                    <div className="mb-4">
                      <label className="block font-medium mb-1 text-sm">SSL Key URL</label>
                      <p className="text-gray-600 text-xs mb-1">URL to your private key file (.pem or .key)</p>
                      <input
                        type="text"
                        name="ssl_key_url"
                        value={formData.ssl_key_url || ''}
                        onChange={handleChange}
                        onBlur={handleBlur}
                        className={`neo-input w-full ${errors.ssl_key_url && touched.ssl_key_url ? 'border-red-500' : ''}`}
                        placeholder="https://example.com/key.pem"
                      />
                      {errors.ssl_key_url && touched.ssl_key_url && (
                        <p className="text-red-500 text-xs mt-1">{errors.ssl_key_url}</p>
                      )}
                    </div>
                    
                    <div className="mb-2">
                      <label className="block font-medium mb-1 text-sm">SSL Root Certificate URL</label>
                      <p className="text-gray-600 text-xs mb-1">URL to the CA certificate file (.pem or .crt)</p>
                      <input
                        type="text"
                        name="ssl_root_cert_url"
                        value={formData.ssl_root_cert_url || ''}
                        onChange={handleChange}
                        onBlur={handleBlur}
                        className={`neo-input w-full ${errors.ssl_root_cert_url && touched.ssl_root_cert_url ? 'border-red-500' : ''}`}
                        placeholder="https://example.com/ca.pem"
                      />
                      {errors.ssl_root_cert_url && touched.ssl_root_cert_url && (
                        <p className="text-red-500 text-xs mt-1">{errors.ssl_root_cert_url}</p>
                      )}
                    </div>
                  </div>
                )}
              </>
            )}

            {/* SSH Tab */}
            {connectionType === 'ssh' && (
              <div className="relative">
                {/* SSH Coming Soon Overlay */}
                <div className="absolute inset-0 -right-2 flex items-center justify-center bg-white/80 z-10 rounded-lg -bottom-2">
                  <div className="text-center max-w-xs p-4">
                    <KeyRound className="w-12 h-12 mx-auto text-yellow-500 mb-2" />
                    <h3 className="text-lg font-bold">SSH Coming Soon</h3>
                    <p className="text-gray-600 mt-2">
                      We're currently working on SSH tunnel support for secure database connections. Stay tuned!
                    </p>
                  </div>
                </div>
                
                <div className="space-y-6">
                  <div>
                    <label className="block font-bold mb-2 text-lg">SSH Host</label>
                    <p className="text-gray-600 text-sm mb-2">The hostname or IP address of your SSH server</p>
                    <input
                      type="text"
                      name="ssh_host"
                      value={formData.ssh_host || ''}
                      onChange={handleChange}
                      onBlur={handleBlur}
                      className={`neo-input w-full ${errors.ssh_host && touched.ssh_host ? 'border-neo-error' : ''}`}
                      placeholder="e.g., ssh.example.com"
                    />
                    {errors.ssh_host && touched.ssh_host && (
                      <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                        <AlertCircle className="w-4 h-4" />
                        <span>{errors.ssh_host}</span>
                      </div>
                    )}
                  </div>
                  
                  <div>
                    <label className="block font-bold mb-2 text-lg">SSH Port</label>
                    <p className="text-gray-600 text-sm mb-2">The port number your SSH server is listening on</p>
                    <input
                      type="text"
                      name="ssh_port"
                      value={formData.ssh_port || '22'}
                      onChange={handleChange}
                      onBlur={handleBlur}
                      className={`neo-input w-full ${errors.ssh_port && touched.ssh_port ? 'border-neo-error' : ''}`}
                      placeholder="e.g., 22"
                    />
                    {errors.ssh_port && touched.ssh_port && (
                      <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                        <AlertCircle className="w-4 h-4" />
                        <span>{errors.ssh_port}</span>
                      </div>
                    )}
                  </div>
                  
                  <div>
                    <label className="block font-bold mb-2 text-lg">SSH Username</label>
                    <p className="text-gray-600 text-sm mb-2">SSH user with appropriate permissions</p>
                    <input
                      type="text"
                      name="ssh_username"
                      value={formData.ssh_username || ''}
                      onChange={handleChange}
                      onBlur={handleBlur}
                      className={`neo-input w-full ${errors.ssh_username && touched.ssh_username ? 'border-neo-error' : ''}`}
                      placeholder="SSH username"
                    />
                    {errors.ssh_username && touched.ssh_username && (
                      <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                        <AlertCircle className="w-4 h-4" />
                        <span>{errors.ssh_username}</span>
                      </div>
                    )}
                  </div>
                  
                  <div>
                    <label className="block font-bold mb-2 text-lg">SSH Private Key</label>
                    <p className="text-gray-600 text-sm mb-2">Private key for SSH authentication</p>
                    <textarea
                      name="ssh_private_key"
                      value={formData.ssh_private_key || ''}
                      onChange={(e) => {
                        setFormData((prev) => ({
                          ...prev,
                          ssh_private_key: e.target.value,
                        }));
                        
                        if (touched.ssh_private_key) {
                          const error = validateField('ssh_private_key', {
                            ...formData,
                            ssh_private_key: e.target.value,
                          });
                          setErrors(prev => ({
                            ...prev,
                            ssh_private_key: error,
                          }));
                        }
                      }}
                      onBlur={() => {
                        setTouched(prev => ({
                          ...prev,
                          ssh_private_key: true,
                        }));
                        const error = validateField('ssh_private_key', formData);
                        setErrors(prev => ({
                          ...prev,
                          ssh_private_key: error,
                        }));
                      }}
                      className={`neo-input w-full h-32 font-mono text-sm ${errors.ssh_private_key && touched.ssh_private_key ? 'border-neo-error' : ''}`}
                      placeholder="-----BEGIN RSA PRIVATE KEY-----&#10;...&#10;-----END RSA PRIVATE KEY-----"
                    />
                    {errors.ssh_private_key && touched.ssh_private_key && (
                      <div className="flex items-center gap-1 mt-1 text-neo-error text-sm">
                        <AlertCircle className="w-4 h-4" />
                        <span>{errors.ssh_private_key}</span>
                      </div>
                    )}
                  </div>
                  
                  <div>
                    <label className="block font-bold mb-2 text-lg">SSH Passphrase (Optional)</label>
                    <p className="text-gray-600 text-sm mb-2">Passphrase for your private key if it's protected</p>
                    <input
                      type="password"
                      name="ssh_passphrase"
                      value={formData.ssh_passphrase || ''}
                      onChange={handleChange}
                      className="neo-input w-full"
                      placeholder="Private key passphrase"
                    />
                    <p className="text-gray-500 text-xs mt-2">
                      Leave blank if your private key is not protected with a passphrase
                    </p>
                  </div>
                  
                  <div className="my-6 border-t border-gray-200 pt-4">
                    <label className="block font-bold mb-2 text-lg">Database Settings</label>
                    <p className="text-gray-600 text-sm mb-2">Configure your database through SSH tunnel</p>
                  </div>

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
                          { value: 'mysql', label: 'MySQL' },
                          { value: 'clickhouse', label: 'ClickHouse' },
                          { value: 'mongodb', label: 'MongoDB' },
                          { value: 'cassandra', label: 'Cassandra (Coming Soon)' },
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

                  {/* MongoDB Connection URI Field - Only show when MongoDB is selected */}
                  {formData.type === 'mongodb' && (
                    <div>
                      <label className="block font-bold mb-2 text-lg">MongoDB Connection URI</label>
                      <p className="text-gray-600 text-sm mb-2">Paste your MongoDB connection string to auto-fill fields</p>
                      <input
                        type="text"
                        name="mongo_uri_ssh"
                        ref={mongoUriSshInputRef}
                        className="neo-input w-full"
                        placeholder="mongodb://username:password@host:port/database or mongodb+srv://username:password@host/database"
                        onChange={(e) => {
                          const uri = e.target.value;
                          try {
                            // Better parsing logic for MongoDB URIs that can handle special characters in credentials
                            const srvFormat = uri.startsWith('mongodb+srv://');
                            
                            // Extract the protocol and the rest
                            const protocolMatch = uri.match(/^(mongodb(?:\+srv)?:\/\/)(.*)/);
                            if (!protocolMatch) {
                              console.log("Invalid MongoDB URI format: Missing protocol");
                              return;
                            }
                            
                            const [, protocol, remainder] = protocolMatch;
                            
                            // Check if credentials are provided (look for @ after the protocol)
                            const hasCredentials = remainder.includes('@');
                            let username = '';
                            let password = '';
                            let hostPart = remainder;
                            
                            if (hasCredentials) {
                              // Find the last @ which separates credentials from host
                              const lastAtIndex = remainder.lastIndexOf('@');
                              const credentialsPart = remainder.substring(0, lastAtIndex);
                              hostPart = remainder.substring(lastAtIndex + 1);
                              
                              // Find the first : which separates username from password
                              const firstColonIndex = credentialsPart.indexOf(':');
                              if (firstColonIndex !== -1) {
                                username = credentialsPart.substring(0, firstColonIndex);
                                password = credentialsPart.substring(firstColonIndex + 1);
                              } else {
                                username = credentialsPart;
                              }
                            }
                            
                            // Parse host, port and database
                            let host = '';
                            let port = srvFormat ? '27017' : ''; // Default for SRV format
                            let database = 'test'; // Default database name
                            
                            // Check if there's a / after the host[:port] part
                            const pathIndex = hostPart.indexOf('/');
                            if (pathIndex !== -1) {
                              const hostPortPart = hostPart.substring(0, pathIndex);
                              const pathPart = hostPart.substring(pathIndex + 1);
                              
                              // Extract database name (everything before ? or end of string)
                              const dbEndIndex = pathPart.indexOf('?');
                              if (dbEndIndex !== -1) {
                                database = pathPart.substring(0, dbEndIndex);
                              } else {
                                database = pathPart;
                              }
                              
                              // Parse host and port
                              const portIndex = hostPortPart.indexOf(':');
                              if (portIndex !== -1) {
                                host = hostPortPart.substring(0, portIndex);
                                port = hostPortPart.substring(portIndex + 1);
                              } else {
                                host = hostPortPart;
                              }
                            } else {
                              // No database specified in the URI
                              const portIndex = hostPart.indexOf(':');
                              if (portIndex !== -1) {
                                host = hostPart.substring(0, portIndex);
                                port = hostPart.substring(portIndex + 1);
                              } else {
                                host = hostPart;
                              }
                            }
                            
                            if (host) {
                              console.log("MongoDB URI parsed successfully", { username, host, port, database });
                              
                              setFormData(prev => ({
                                ...prev,
                                host: host,
                                port: port || (srvFormat ? '27017' : prev.port),
                                database: database || 'test',
                                username: username || prev.username,
                                password: password || prev.password
                              }));
                              
                              // Mark fields as touched
                              setTouched(prev => ({
                                ...prev,
                                host: true,
                                port: !!port,
                                database: !!database,
                                username: !!username
                              }));
                            } else {
                              console.log("MongoDB URI parsing failed: could not extract host");
                            }
                          } catch (err) {
                            // Invalid URI format, just continue
                            console.log("Invalid MongoDB URI format", err);
                          }
                        }}
                      />
                      <p className="text-gray-500 text-xs mt-2">
                        Connection URI will be used to auto-fill the fields below. Both standard and Atlas SRV formats supported.
                      </p>
                    </div>
                  )}
                    
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
                      value={formData.password || ''}
                      onChange={handleChange}
                      className="neo-input w-full"
                      placeholder="Enter your database password"
                    />
                    <p className="text-gray-500 text-xs mt-2">Leave blank if the database has no password, but it's recommended to set a password for the database user</p>
                  </div>
                </div>
              </div>
            )}

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

      {/* Select Tables Modal */}
      {showSelectTablesModal && initialData && (
        <SelectTablesModal
          chat={initialData}
          onClose={() => {
            setShowSelectTablesModal(false);
            onClose();
          }}
          onSave={handleUpdateSelectedCollections}
        />
      )}
    </>
  );
}