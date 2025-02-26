import { EventSourcePolyfill } from 'event-source-polyfill';
import {
  ArrowRight,
  Boxes,
  Loader2,
  PanelLeft,
  PanelLeftClose,
  Plus,
  Trash2
} from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { toast } from 'react-hot-toast';
import { useUser } from '../../contexts/UserContext';
import chatService from '../../services/chatService';
import { Chat } from '../../types/chat';
import DatabaseLogo from '../icons/DatabaseLogos';
import ConfirmationModal from '../modals/ConfirmationModal';
import DeleteConnectionModal from '../modals/DeleteConnectionModal';

export interface Connection {
  id: string;
  name: string;
  type: 'postgresql' | 'yugabytedb' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j';
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

const formatDate = (dateString: string) => {
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric'
  });
};

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
  const { user } = useUser();

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
              [...connections].sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()).map((connection) => (
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
                          type={connection.connection.type as 'postgresql' | 'yugabytedb' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j'}
                          size={28}
                          className={`transition-transform ${selectedConnection?.id === connection.id ? 'scale-110' : ''}`}
                        />
                        <div className={`transition-opacity duration-300 ${isExpanded ? 'opacity-100' : 'opacity-0 w-0 overflow-hidden'
                          }`}>
                          <div className="text-left">
                            <h3 className="font-bold text-lg leading-tight">{connection.connection.database.length > 20 ? connection.connection.database.slice(0, 20) + '...' : connection.connection.database}</h3>
                            <p className="text-gray-600 capitalize text-sm">{connection.connection.type === 'postgresql' ? 'PostgreSQL' : connection.connection.type === 'yugabytedb' ? 'YugabyteDB' : connection.connection.type === 'mysql' ? 'MySQL' : connection.connection.type === 'clickhouse' ? 'ClickHouse' : connection.connection.type === 'mongodb' ? 'MongoDB' : connection.connection.type === 'redis' ? 'Redis' : connection.connection.type === 'neo4j' ? 'Neo4j' : 'Unknown'}</p>
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
              isExpanded && (
                <div className="flex flex-col items-center justify-center h-full bg-neo-gray rounded-lg p-4">
                  <p className="text-black font-bold text-lg">No connections found</p>
                  <p className="text-gray-600">Add a connection to get started</p>
                </div>
              )
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

            {isExpanded && (
              <div className="p-3 border-2 border-black rounded-lg bg-white shadow-neo">
                <div className="flex flex-col space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="font-bold text-base">{user?.username}</span>
                    <span className="text-xs text-gray-500 bg-gray-100 px-2 py-1 rounded-full">
                      Joined {formatDate(user?.created_at || '')}
                    </span>
                  </div>
                  <div className="border-t border-gray-200 my-2"></div>
                  <button
                    onClick={handleLogoutClick}
                    className="text-red-500 hover:text-red-600 text-sm font-medium flex items-center gap-2 transition-colors"
                  >
                    <span>Logout</span>
                    <ArrowRight className="w-4 h-4" />
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Mobile view - show only when expanded */}
          <div className={`flex flex-col gap-3 md:hidden ${!isExpanded && 'hidden'}`}>
            <button
              onClick={onAddConnection}
              className="neo-button h-12 flex items-center justify-center"
              title="Add Connection"
            >
              <Plus className="w-5 h-5" />
              <span className="ml-2">Add Connection</span>
            </button>

            <div className="p-3 border-2 border-black rounded-lg bg-white shadow-neo">
              <div className="flex flex-col space-y-2">
                <div className="flex items-center justify-between">
                  <span className="font-bold text-base">{user?.username}</span>
                  <span className="text-xs text-gray-500 bg-gray-100 px-2 py-1 rounded-full">
                    Joined {formatDate(user?.created_at || '')}
                  </span>
                </div>
                <div className="border-t border-gray-200 my-2"></div>
                <button
                  onClick={handleLogoutClick}
                  className="text-red-500 hover:text-red-600 text-sm font-medium flex items-center gap-2 transition-colors"
                >
                  <span>Sign out</span>
                  <ArrowRight className="w-4 h-4" />
                </button>
              </div>
            </div>
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
            onConfirm={async () => {
              await handleLogoutConfirm();
              setShowLogoutConfirm(false);
            }}
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