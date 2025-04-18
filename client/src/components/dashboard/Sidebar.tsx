import { EventSourcePolyfill } from 'event-source-polyfill';
import {
  ArrowRight,
  Boxes,
  HelpCircle,
  Loader2,
  MoreVertical,
  PanelLeft,
  PanelLeftClose,
  Pencil,
  Plus,
  Settings,
  Trash2,
  X
} from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { toast } from 'react-hot-toast';
import { useUser } from '../../contexts/UserContext';
import chatService from '../../services/chatService';
import analyticsService from '../../services/analyticsService';
import { Chat } from '../../types/chat';
import DatabaseLogo from '../icons/DatabaseLogos';
import ConfirmationModal from '../modals/ConfirmationModal';
import DeleteConnectionModal from '../modals/DeleteConnectionModal';
import { DemoModal } from '../modals/DemoModal';

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
  onEditConnection?: () => void;
  selectedConnection?: Chat;
  isLoadingConnections: boolean;
  onConnectionStatusChange?: (chatId: string, isConnected: boolean, from: string) => void;
  eventSource: EventSourcePolyfill | null;
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
  onEditConnection,
  selectedConnection,
  isLoadingConnections,
  onConnectionStatusChange,
}: SidebarProps) {
  const [showLogoutConfirm, setShowLogoutConfirm] = useState(false);
  const [connectionToDelete, setConnectionToDelete] = useState<Chat | null>(null);
  const [currentConnectedChatId, setCurrentConnectedChatId] = useState<string | null>(null);
  const previousConnectionRef = useRef<string | null>(null);
  const { user } = useUser();
  const [openConnectionMenu, setOpenConnectionMenu] = useState<string | null>(null);
  const [menuPosition, setMenuPosition] = useState<{ top: number; left: number } | null>(null);
  const [showDemoModal, setShowDemoModal] = useState(false);
  const [isTutorialClosed, setIsTutorialClosed] = useState(false);
  
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (openConnectionMenu) {
        const target = event.target as HTMLElement;
        if (!target.closest('.connection-menu-container') && !target.closest('.connection-dropdown-menu')) {
          setOpenConnectionMenu(null);
          setMenuPosition(null);
        }
      }
    };
    
    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [openConnectionMenu]);

  useEffect(() => {
    const selectedId = selectedConnection?.id;
    if (selectedId && selectedId !== previousConnectionRef.current) {
      handleSelectConnection(selectedId);
      previousConnectionRef.current = selectedId;
    }
  }, [selectedConnection]);

  useEffect(() => {
    // Check localStorage when component mounts
    const tutorialClosed = localStorage.getItem("isTutorialClosed") === "true";
    setIsTutorialClosed(tutorialClosed);
  }, []);

  const handleToggleExpand = useCallback(() => {
    // Track sidebar toggled event
    analyticsService.trackSidebarToggled(!isExpanded);
    
    onToggleExpand();
  }, [isExpanded, onToggleExpand]);

  const handleLogoutClick = () => {
    setShowLogoutConfirm(true);
  };

  const handleLogoutConfirm = () => {
    // If user exists, track logout event
    if (user) {
      analyticsService.trackLogout(user.id, user.username);
    }
    
    onLogout();
    setShowLogoutConfirm(false);
  };

  const handleEditConnection = (connection: Chat) => {
    setOpenConnectionMenu(null);
    
    // Track connection edit event
    analyticsService.trackConnectionEdited(
      connection.id,
      connection.connection.type,
      connection.connection.database
    );
    
    handleSelectConnection(connection.id);
    
    if (onEditConnection) {
      onEditConnection();
    }
  };

  const handleDeleteClick = useCallback((connection: Chat) => {
    setOpenConnectionMenu(null);
    setConnectionToDelete(connection);
  }, []);

  const handleDeleteConfirm = async (chatId: string) => {
    try {
      const connectionToDelete = connections.find(chat => chat.id === chatId);
      
      if (connectionToDelete) {
        // Track connection deleted event
        analyticsService.trackConnectionDeleted(
          chatId,
          connectionToDelete.connection.type,
          connectionToDelete.connection.database
        );
      }
      
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

      if (id === currentConnectedChatId) {
        onSelectConnection(id);
        return;
      }

      // Track connection selected event
      const connection = connections.find(chat => chat.id === id);
      if (connection) {
        analyticsService.trackConnectionSelected(
          id,
          connection.connection.type,
          connection.connection.database
        );
      }
      
      setCurrentConnectedChatId(id);
      onSelectConnection(id);
      onConnectionStatusChange?.(id, false, 'sidebar-connecting');

    } catch (error) {
      console.error('Failed to setup connection:', error);
      onConnectionStatusChange?.(id, false, 'sidebar-select-connection');
      toast.error('Failed to connect to database');
    }
  }, [currentConnectedChatId, onSelectConnection, onConnectionStatusChange, connections]);

  const handleOpenMenu = (e: React.MouseEvent, connectionId: string) => {
    e.preventDefault();
    e.stopPropagation();
    
    // Get the position of the button
    const button = e.currentTarget;
    const rect = button.getBoundingClientRect();
    
    // Set the position for the dropdown
    setMenuPosition({
      top: rect.top,
      left: rect.right + 10 // Position to the right of the button
    });
    
    // Toggle the menu
    setOpenConnectionMenu(openConnectionMenu === connectionId ? null : connectionId);
  };

  const handleCloseTutorial = (e: React.MouseEvent) => {
    e.stopPropagation(); // Prevent triggering the parent button's onClick
    localStorage.setItem("isTutorialClosed", "true");
    setIsTutorialClosed(true);
  };

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
            <>
              {connections.length > 0 ? (
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
                        <div className="connection-menu-container absolute right-2 top-1/2 -translate-y-1/2">
                          <button
                            onClick={(e) => handleOpenMenu(e, connection.id)}
                            className="p-2 opacity-0 group-hover:opacity-100 transition-all neo-border bg-white hover:bg-gray-100"
                            title="Connection menu"
                          >
                            <MoreVertical className="w-4 h-4" />
                          </button>
                        </div>
                      )}
                    </div>
                  </div>
                ))
              ) : (
                isExpanded && (
                  <div className="flex flex-col items-center justify-center h-full bg-neo-gray rounded-lg p-4">
                    <p className="text-black font-bold text-lg">No connections yet</p>
                    <p className="text-gray-600">Add a connection to get started</p>
                  </div>
                )
              )}
            </>
          )}
        </div>
          {/* How to Use Card - Moved to bottom */}
           {isExpanded && !isTutorialClosed && (
                    <div className="px-4">
            <div className="mb-3 relative">
              <button
                onClick={() => setShowDemoModal(true)}
                className="w-full p-4 rounded-lg bg-purple-100 hover:bg-purple-200 transition-colors border-2 border-purple-300 flex items-center gap-3"
              >
                <div className="bg-purple-500 rounded-full p-2">
                  <HelpCircle className="w-5 h-5 text-white" />
                </div>
                <div className="text-left">
                  <h3 className="font-bold text-lg leading-tight text-base">How to Use NeoBase</h3>
                  <p className="text-gray-600 text-sm">Try this quick tutorial</p>
                </div>
              </button>
              {/* Close button */}
              <button
                onClick={handleCloseTutorial}
                className="absolute top-1.5 right-1.5 p-1 rounded-full hover:bg-purple-200 transition-colors"
                aria-label="Close tutorial"
              >
                <X className="w-4 h-4 text-purple-700" />
              </button>
            </div>
            </div>
          )}
          
        <div className="p-4 border-t-4 border-black">

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
                    <span className="font-bold text-base">{user != null && user?.username!.length > 16 ? user?.username!.slice(0, 16) + '...' : user?.username}</span>
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
          onClick={handleToggleExpand}
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

      {openConnectionMenu && menuPosition && (
        <div 
          className="fixed w-48 bg-white border-4 border-black rounded-lg shadow-[4px_4px_0px_0px_rgba(0,0,0,1)] z-[100] connection-dropdown-menu"
          style={{
            top: `${menuPosition.top}px`,
            left: `${menuPosition.left}px`,
            transform: 'none'
          }}
          onClick={(e) => e.stopPropagation()}
        >
          <div className="py-1">
            <button 
              onClick={() => {
                const connection = connections.find(c => c.id === openConnectionMenu);
                if (connection) {
                  handleEditConnection(connection);
                }
                setOpenConnectionMenu(null);
                setMenuPosition(null);
              }}
              className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-black hover:bg-[#FFDB58] transition-colors"
            >
              <Pencil className="w-4 h-4 mr-2 text-black" />
              Edit Connection
            </button>
            <div className="h-px bg-gray-200 mx-2"></div>
            <button 
              onClick={() => {
                const connection = connections.find(c => c.id === openConnectionMenu);
                if (connection) {
                  handleDeleteClick(connection);
                }
                setOpenConnectionMenu(null);
                setMenuPosition(null);
              }}
              className="flex items-center w-full text-left px-4 py-2 text-sm font-semibold text-red-500 hover:bg-neo-error hover:text-white transition-colors"
            >
              <Trash2 className="w-4 h-4 mr-2 text-red-500" />
              Delete Connection
            </button>
          </div>
        </div>
      )}

      {/* Demo Modal */}
      <DemoModal isOpen={showDemoModal} onClose={() => setShowDemoModal(false)} />
    </>
  );
}