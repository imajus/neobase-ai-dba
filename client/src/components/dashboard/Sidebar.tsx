import axios from 'axios';
import { EventSourcePolyfill } from 'event-source-polyfill';
import {
  Boxes,
  Loader2,
  LogOut,
  PanelLeft,
  PanelLeftClose,
  Plus,
  Trash2
} from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { toast } from 'react-hot-toast';
import chatService from '../../services/chatService';
import { Chat } from '../../types/chat';
import DatabaseLogo from '../icons/DatabaseLogos';
import ConfirmationModal from '../modals/ConfirmationModal';
import DeleteConnectionModal from '../modals/DeleteConnectionModal';

export interface Connection {
  id: string;
  name: string;
  type: 'postgresql' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j';
}

interface ConnectionStatus {
  isConnected: boolean;
  type: string;
  host: string;
  port: number;
  database: string;
  username: string;
}

interface SidebarProps {
  isExpanded: boolean;
  onToggleExpand: () => void;
  connections: Chat[];
  onSelectConnection: (id: string) => void;
  onAddConnection: () => void;
  onLogout: () => void;
  onDeleteConnection?: (id: string) => void;
  selectedConnection?: Chat;
  isLoadingConnections: boolean;
  onConnectionStatusChange?: (chatId: string, isConnected: boolean, from: string) => void;
  setupSSEConnection: (chatId: string) => Promise<string>;
  eventSource: EventSourcePolyfill | null;
}

interface SSEEvent {
  event: 'db-connected' | 'db-disconnected';
  data: string;
}

export default function Sidebar({
  isExpanded,
  onToggleExpand,
  connections,
  onSelectConnection,
  onAddConnection,
  onLogout,
  onDeleteConnection,
  selectedConnection,
  isLoadingConnections,
  onConnectionStatusChange,
  setupSSEConnection,
}: SidebarProps) {
  const [showLogoutConfirm, setShowLogoutConfirm] = useState(false);
  const [connectionToDelete, setConnectionToDelete] = useState<Chat | null>(null);
  const [currentConnectedChatId, setCurrentConnectedChatId] = useState<string | null>(null);
  const previousConnectionRef = useRef<string | null>(null);

  // Watch for selected connection changes
  useEffect(() => {
    const selectedId = selectedConnection?.id;
    // Only setup new connection if selection changed and is different from current
    if (selectedId && selectedId !== previousConnectionRef.current) {
      handleSelectConnection(selectedId);
      previousConnectionRef.current = selectedId;
    }
  }, [selectedConnection]);

  const handleLogoutClick = () => {
    setShowLogoutConfirm(true);
  };

  const handleLogoutConfirm = () => {
    onLogout();
    setShowLogoutConfirm(false);
  };

  const handleDeleteClick = useCallback((connection: Chat) => {
    setConnectionToDelete(connection);
  }, []);

  const handleDeleteConfirm = async (chatId: string) => {
    try {
      await chatService.deleteChat(chatId);
      if (onDeleteConnection) {
        onDeleteConnection(chatId);
      }
      setConnectionToDelete(null);
    } catch (error: any) {
      toast.error(error.message, {
        style: {
          background: '#ff4444',
          color: '#fff',
          border: '4px solid #cc0000',
          borderRadius: '12px',
          boxShadow: '4px 4px 0px 0px rgba(0,0,0,1)',
        },
      });
    }
  };

  const connectToDatabase = async (chatId: string, streamId: string) => {
    try {
      await axios.post(
        `${import.meta.env.VITE_API_URL}/chats/${chatId}/connect`,
        { stream_id: streamId },
        {
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );
    } catch (error) {
      console.error('Failed to connect to database:', error);
      throw error;
    }
  };

  const checkConnectionStatus = async (chatId: string, streamId: string) => {
    try {
      const response = await axios.get<ConnectionStatus>(
        `${import.meta.env.VITE_API_URL}/chats/${chatId}/connection-status`,
        {
          withCredentials: true,
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`
          }
        }
      );
      return response.data.isConnected;
    } catch (error) {
      console.error('Failed to check connection status:', error);
      return false;
    }
  };

  const handleSelectConnection = useCallback(async (id: string) => {
    try {
      console.log('handleSelectConnection happened', { id, currentConnectedChatId });

      // If same connection is selected, just update UI and return
      if (id === currentConnectedChatId) {
        onSelectConnection(id);
        return;
      }

      setCurrentConnectedChatId(id);
      onSelectConnection(id);
      onConnectionStatusChange?.(id, false, 'sidebar-connecting');

      // Setup new SSE connection
      const newStreamId = await setupSSEConnection(id);

      // Check current connection status before attempting to connect
      const connectionStatus = await checkConnectionStatus(id, newStreamId);
      console.log('Current connection status:', { id, connectionStatus });

      if (!connectionStatus) {
        // Only connect if not already connected
        await connectToDatabase(id, newStreamId);
      } else {
        // If already connected, just update the UI
        onConnectionStatusChange?.(id, true, 'sidebar-existing-connection');
      }

    } catch (error) {
      console.error('Failed to setup connection:', error);
      onConnectionStatusChange?.(id, false, 'sidebar-select-connection');
      toast.error('Failed to connect to database');
    }
  }, [currentConnectedChatId, onSelectConnection, onConnectionStatusChange, setupSSEConnection]);

  return (
    <>
      <div
        className={`${isExpanded ? 'w-80' : 'w-20'
          } fixed left-0 top-0 h-screen bg-white border-r-4 border-black flex flex-col transition-all duration-300 group z-40 transform md:translate-x-0 ${isExpanded ? 'translate-x-0' : '-translate-x-full md:translate-x-0'
          }`}
      >
        <div className="h-16 px-4 border-b-4 border-black shadow-sm flex justify-between items-center relative hidden md:flex">
          <div className="flex items-center gap-3 relative">
            <Boxes className="w-8 h-8" />
            <h1 className={`text-2xl font-bold tracking-tight transition-opacity duration-300 ${isExpanded ? 'opacity-100' : 'opacity-0 w-0'
              }`}>NeoBase</h1>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-4 mt-16 md:mt-0">
          {isLoadingConnections ? (
            <div className="flex items-center justify-center h-full">
              <Loader2 className="w-8 h-8 animate-spin" />
            </div>
          ) : (
            connections.length > 0 ? (
              connections.map((connection) => (
                <div key={connection.id} className="mb-4">
                  <div className={`relative group ${!isExpanded ? 'md:w-12 md:h-12' : ''}`}>
                    <button
                      onClick={() => handleSelectConnection(connection.id)}
                      className={`w-full h-full cursor-pointer ${isExpanded ? 'p-4' : 'p-3'} rounded-lg transition-all ${selectedConnection?.id === connection.id ? 'bg-[#FFDB58]' : 'bg-white hover:bg-gray-50'
                        }`}
                      title={connection.connection.database}
                    >
                      <div className={`flex items-center h-full ${isExpanded ? 'gap-3' : 'justify-center'}`}>
                        <DatabaseLogo
                          type={connection.connection.type as 'postgresql' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j'}
                          size={28}
                          className={`transition-transform ${selectedConnection?.id === connection.id ? 'scale-110' : ''}`}
                        />
                        <div className={`transition-opacity duration-300 ${isExpanded ? 'opacity-100' : 'opacity-0 w-0 overflow-hidden'
                          }`}>
                          <div className="text-left">
                            <h3 className="font-bold text-lg leading-tight">{connection.connection.database}</h3>
                            <p className="text-gray-600 capitalize text-sm">{connection.connection.type === 'postgresql' ? 'PostgreSQL' : connection.connection.type === 'mysql' ? 'MySQL' : connection.connection.type === 'clickhouse' ? 'ClickHouse' : connection.connection.type === 'mongodb' ? 'MongoDB' : connection.connection.type === 'redis' ? 'Redis' : connection.connection.type === 'neo4j' ? 'Neo4j' : 'Unknown'}</p>
                          </div>
                        </div>
                      </div>
                    </button>
                    {isExpanded && onDeleteConnection && (
                      <button
                        onClick={() => handleDeleteClick(connection)}
                        className="absolute right-2 top-1/2 -translate-y-1/2 p-2 opacity-0 group-hover:opacity-100 transition-all neo-border bg-white hover:bg-neo-error hover:text-white"
                        title="Delete connection"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
                  </div>
                </div>
              ))
            ) : (
              // Make good looking empty state
              <div className="flex flex-col items-center justify-center h-full bg-neo-gray rounded-lg p-4">
                <p className="text-black font-bold text-lg">No connections found</p>
                <p className="text-gray-600">Add a connection to get started</p>
              </div>
            )
          )}
        </div>
        <div className="p-4 border-t-4 border-black">
          {/* Desktop view */}
          <div className="hidden md:flex md:flex-col md:space-y-3">
            <button
              onClick={onAddConnection}
              className={`neo-button flex items-center justify-center ${isExpanded ? 'w-full' : 'w-12 h-12 p-3'}`}
              title="Add Connection"
            >
              <Plus className="w-5 h-5" />
              <span className={`transition-opacity duration-300 ${isExpanded ? 'opacity-100 ml-2' : 'opacity-0 w-0 overflow-hidden'}`}>
                Add Connection
              </span>
            </button>
            <button
              onClick={handleLogoutClick}
              className={`neo-button-secondary flex items-center justify-center ${isExpanded ? 'w-full' : 'w-12 h-12 p-3'}`}
              title="Logout"
            >
              <LogOut className="w-5 h-5" />
              <span className={`transition-opacity duration-300 ${isExpanded ? 'opacity-100 ml-2' : 'opacity-0 w-0 overflow-hidden'}`}>
                Logout
              </span>
            </button>
          </div>

          {/* Mobile view - show only when expanded */}
          <div className={`flex gap-3 md:hidden ${!isExpanded && 'hidden'}`}>
            <button
              onClick={onAddConnection}
              className="
                neo-button 
                flex-1
                h-12
                flex 
                items-center 
                justify-center
              "
              title="Add Connection"
            >
              <Plus className="w-5 h-5" />
            </button>
            <button
              onClick={handleLogoutClick}
              className="
                neo-button-secondary 
                flex-1
                h-12
                flex 
                items-center 
                justify-center
              "
              title="Logout"
            >
              <LogOut className="w-5 h-5" />
            </button>
          </div>
        </div>

        <button
          onClick={onToggleExpand}
          className="absolute top-1/2 -translate-y-1/2 -right-4 p-2 bg-white hover:bg-neo-gray rounded-lg transition-colors border-2 border-black"
          title={isExpanded ? "Collapse sidebar" : "Expand sidebar"}
        >
          {isExpanded ? (
            <PanelLeftClose className="w-5 h-5" />
          ) : (
            <PanelLeft className="w-5 h-5" />
          )}
        </button>
      </div>

      {/* Modals rendered outside the sidebar container */}
      {showLogoutConfirm && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50">
          <ConfirmationModal
            title="Confirm Logout"
            message="Are you sure you want to logout? Any unsaved changes will be lost."
            onConfirm={handleLogoutConfirm}
            onCancel={() => setShowLogoutConfirm(false)}
          />
        </div>
      )}

      {connectionToDelete && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50">
          <DeleteConnectionModal
            connectionName={connectionToDelete.connection.database}
            chatId={connectionToDelete.id}
            onConfirm={handleDeleteConfirm}
            onCancel={() => setConnectionToDelete(null)}
          />
        </div>
      )}
    </>
  );
}