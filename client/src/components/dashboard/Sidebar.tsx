import {
  Boxes,
  LogOut,
  PanelLeft,
  PanelLeftClose,
  Plus,
  Trash2,
} from 'lucide-react';
import { useCallback, useState } from 'react';
import DatabaseLogo from '../icons/DatabaseLogos';
import ConfirmationModal from '../modals/ConfirmationModal';
import { ConnectionFormData } from '../modals/ConnectionModal';
import DeleteConnectionModal from '../modals/DeleteConnectionModal';

export interface Connection {
  id: string;
  name: string;
  type: 'postgresql' | 'mysql' | 'clickhouse' | 'mongodb' | 'redis' | 'neo4j';
}

interface SidebarProps {
  isExpanded: boolean;
  onToggleExpand: () => void;
  connections: ConnectionFormData[];
  onSelectConnection: (id: string) => void;
  onAddConnection: () => void;
  onLogout: () => void;
  onDeleteConnection?: (id: string) => void;
  selectedConnection?: ConnectionFormData;
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
}: SidebarProps) {
  const [showLogoutConfirm, setShowLogoutConfirm] = useState(false);
  const [connectionToDelete, setConnectionToDelete] = useState<ConnectionFormData | null>(null);

  const handleLogoutClick = () => {
    setShowLogoutConfirm(true);
  };

  const handleLogoutConfirm = () => {
    onLogout();
    setShowLogoutConfirm(false);
  };

  const handleDeleteClick = useCallback((connection: ConnectionFormData) => {
    setConnectionToDelete(connection);
  }, []);

  const handleDeleteConfirm = useCallback(() => {
    if (connectionToDelete && onDeleteConnection) {
      onDeleteConnection(connectionToDelete.id);
    }
    setConnectionToDelete(null);
  }, [connectionToDelete, onDeleteConnection]);

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
          {connections.map((connection) => (
            <div key={connection.id} className="mb-4">
              <div className={`relative group ${!isExpanded ? 'md:w-12 md:h-12' : ''}`}>
                <button
                  onClick={() => onSelectConnection(connection.id)}
                  className={`w-full h-full cursor-pointer ${isExpanded ? 'p-4' : 'p-3'} rounded-lg transition-all ${selectedConnection?.id === connection.id ? 'bg-[#FFDB58]' : 'bg-white hover:bg-gray-50'
                    }`}
                  title={connection.database}
                >
                  <div className={`flex items-center h-full ${isExpanded ? 'gap-3' : 'justify-center'}`}>
                    <DatabaseLogo
                      type={connection.type}
                      size={28}
                      className={`transition-transform ${selectedConnection?.id === connection.id ? 'scale-110' : ''}`}
                    />
                    <div className={`transition-opacity duration-300 ${isExpanded ? 'opacity-100' : 'opacity-0 w-0 overflow-hidden'
                      }`}>
                      <div className="text-left">
                        <h3 className="font-bold text-lg leading-tight">{connection.database}</h3>
                        <p className="text-gray-600 capitalize text-sm">{connection.type}</p>
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
          ))}
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
            connectionName={connectionToDelete.database}
            onConfirm={handleDeleteConfirm}
            onCancel={() => setConnectionToDelete(null)}
          />
        </div>
      )}
    </>
  );
}