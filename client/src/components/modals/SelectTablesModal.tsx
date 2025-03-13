import { AlertCircle, ChevronDown, ChevronRight, Database, Loader2, Search, X } from 'lucide-react';
import React, { useEffect, useState } from 'react';
import { Chat, TableInfo } from '../../types/chat';
import chatService from '../../services/chatService';

interface SelectTablesModalProps {
  chat: Chat;
  onClose: () => void;
  onSave: (selectedTables: string) => Promise<void>;
}

export default function SelectTablesModal({ chat, onClose, onSave }: SelectTablesModalProps) {
  const [isLoading, setIsLoading] = useState(true);
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [selectedTables, setSelectedTables] = useState<string[]>([]);
  const [expandedTables, setExpandedTables] = useState<Record<string, boolean>>({});
  const [error, setError] = useState<string | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [selectAll, setSelectAll] = useState(true);
  const [isApiCallInProgress, setIsApiCallInProgress] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  // Filter tables based on search query
  const filteredTables = tables?.filter(table => 
    table.name.toLowerCase().includes(searchQuery.toLowerCase())
  );

  // Load tables when the modal opens
  useEffect(() => {
    console.log('SelectTablesModal useEffect triggered with chat.id:', chat.id);
    
    // Skip if API call is already in progress
    if (isApiCallInProgress) {
      console.log('SelectTablesModal: API call already in progress, skipping');
      return;
    }
    
    const loadTables = async () => {
      try {
        setIsApiCallInProgress(true);
        setIsLoading(true);
        setError(null);
        setValidationError(null);
        
        console.log('SelectTablesModal: About to call getTables API for chat.id:', chat.id);
        // Get tables from the API
        const tablesResponse = await chatService.getTables(chat.id);
        console.log('SelectTablesModal: Received tables response:', tablesResponse.tables?.length, 'tables');
        setTables(tablesResponse.tables || []);
        
        // Initialize selected tables based on is_selected field
        const selectedTableNames = tablesResponse.tables?.filter((table: TableInfo) => table.is_selected)
          .map((table: TableInfo) => table.name);
        
        setSelectedTables(selectedTableNames);
        
        // Check if all tables are selected to set selectAll state correctly
        setSelectAll(selectedTableNames?.length === tablesResponse.tables?.length);
      } catch (error: any) {
        console.error('Failed to load tables:', error);
        setError(error.message || 'Failed to load tables');
      } finally {
        setIsLoading(false);
        setIsApiCallInProgress(false);
      }
    };

    loadTables();
  }, [chat.id]); // Only re-run when chat.id changes

  const toggleTable = (tableName: string) => {
    setValidationError(null);
    setSelectedTables(prev => {
      if (prev.includes(tableName)) {
        // If removing a table, also uncheck "Select All"
        setSelectAll(false);
        
        // Prevent removing if it's the last selected table
        if (prev.length === 1) {
          setValidationError("At least one table must be selected");
          return prev;
        }
        
        return prev.filter(name => name !== tableName);
      } else {
        // If all tables are now selected, check "Select All"
        const newSelected = [...prev, tableName];
        if (newSelected.length === tables?.length) {
          setSelectAll(true);
        }
        return newSelected;
      }
    });
  };

  const toggleExpandTable = (tableName: string) => {
    setExpandedTables(prev => ({
      ...prev,
      [tableName]: !prev[tableName]
    }));
  };

  const toggleSelectAll = () => {
    setValidationError(null);
    if (selectAll) {
      // Prevent deselecting all tables
      setValidationError("At least one table must be selected");
      return;
    } else {
      // Select all
      setSelectedTables(tables?.map(table => table.name) || []);
      setSelectAll(true);
    }
  };

  const handleSave = async () => {
    // Validate that at least one table is selected
    if (selectedTables?.length === 0) {
      setValidationError("At least one table must be selected");
      return;
    }
    
    try {
      setIsSaving(true);
      setError(null);
      setValidationError(null);
      
      // Format selected tables as "ALL" or comma-separated list
      const formattedSelection = selectAll ? 'ALL' : selectedTables.join(',');
      
      // Check if the selection has changed
      if (formattedSelection !== chat.selected_collections) {
        // Only save if the selection has changed
        await onSave(formattedSelection);
        
        // If this is a new connection (chat was created within the last minute) and "ALL" is selected, refresh schema
        const chatCreatedAt = new Date(chat.created_at);
        const now = new Date();
        const isNewConnection = (now.getTime() - chatCreatedAt.getTime()) < 180000; // Within last 3 minutes
        if (isNewConnection && formattedSelection === 'ALL') {
          try {
            const abortController = new AbortController();
            await chatService.refreshSchema(chat.id, abortController);
            console.log('Schema refreshed successfully for new connection');
          } catch (error) {
            console.error('Failed to refresh schema:', error);
          }
        }
        
        // Close the modal only on success
        onClose();
      } else {
        console.log('Selection unchanged, skipping save');
        // Close the modal even if no changes were made
        onClose();
      }
    } catch (error: any) {
      console.error('Failed to save selected tables:', error);
      setError(error.message || 'Failed to save selected tables');
      // Don't close the modal on error
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4 z-[200]">
      <div className="bg-white neo-border rounded-lg w-full max-w-2xl max-h-[90vh] overflow-hidden flex flex-col relative z-[201]">
        <div className="flex justify-between items-center p-6 border-b-4 border-black">
          <div className="flex items-center gap-2">
            <Database className="w-6 h-6" />
            <h2 className="text-2xl font-bold">Select Tables/Collections</h2>
          </div>
          <button
            onClick={onClose}
            className="hover:bg-gray-100 rounded-lg p-2 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <div className="p-6 overflow-y-auto flex-grow">
          {error && (
            <div className="p-4 bg-red-50 border-2 border-red-500 rounded-lg mb-4">
              <div className="flex items-center gap-2 text-red-600">
                <AlertCircle className="w-5 h-5" />
                <p className="font-medium">{error}</p>
              </div>
            </div>
          )}
          
          {validationError && (
            <div className="p-4 bg-amber-50 border-2 border-amber-500 rounded-lg mb-4">
              <div className="flex items-center gap-2 text-amber-600">
                <AlertCircle className="w-5 h-5" />
                <p className="font-medium">{validationError}</p>
              </div>
            </div>
          )}

          {isLoading ? (
            <div className="flex flex-col items-center justify-center py-12">
              <Loader2 className="w-10 h-10 animate-spin text-gray-400 mb-4" />
              <span className="text-gray-600 font-medium">Loading tables...</span>
              <p className="text-gray-500 text-sm mt-2 text-center max-w-md">
                This may take a moment depending on the size of your database schema.
                <br />
                Large databases with many tables may take longer to load.
              </p>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="neo-border p-4 rounded-lg">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    className="w-5 h-5 rounded-md border-2 border-black"
                    checked={selectAll}
                    onChange={toggleSelectAll}
                  />
                  <span className="font-bold text-lg">All & Auto Detect</span>
                </label>
                <p className="text-gray-600 text-sm mt-1 ml-7">
                  Select all tables/collections in the database. This option auto detects the added/removed entities in the database.
                </p>
              </div>

              <div className="mt-4">
                <div className="flex justify-between items-center mb-2">
                  <h3 className="font-bold text-lg">Individual Tables/Collections</h3>
                  <div className="flex items-center gap-2">
                    <span className="text-sm text-gray-600">
                      {selectAll ? 'All' : `${selectedTables?.length}/${tables?.length}`} selected
                    </span>
                    <button
                      type="button"
                      onClick={() => {
                        const allExpanded = Object.values(expandedTables).every(v => v);
                        const newExpandedState = !allExpanded;
                        const newExpandedTables = tables?.reduce((acc, table) => {
                          acc[table.name] = newExpandedState;
                          return acc;
                        }, {} as Record<string, boolean>);
                        setExpandedTables(newExpandedTables);
                      }}
                      className="text-xs bg-gray-100 hover:bg-gray-200 text-gray-700 px-2 py-1 rounded transition-colors"
                    >
                      {Object.values(expandedTables).every(v => v) ? 'Collapse All' : 'Expand All'}
                    </button>
                  </div>
                </div>
                <p className="text-gray-600 text-sm mb-4">
                  Or select specific tables to include in your database schema.
                </p>

                {/* Search input */}
                <div className="relative mb-4">
                  <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                    <Search className="h-4 w-4 text-gray-400" />
                  </div>
                  <input
                    type="text"
                    placeholder="Search tables..."
                    className="neo-input pl-10 w-full"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                  />
                  {searchQuery && (
                    <button
                      onClick={() => setSearchQuery('')}
                      className="absolute inset-y-0 right-0 pr-3 flex items-center"
                    >
                      <X className="h-4 w-4 text-gray-400 hover:text-gray-600" />
                    </button>
                  )}
                </div>

                <div className="space-y-2 max-h-[40vh] overflow-y-auto neo-border p-4 rounded-lg">
                  {filteredTables.length === 0 ? (
                    <div className="text-center py-4 text-gray-500">
                      {searchQuery ? 'No tables/collections match your search' : 'No tables/collections found in database'}
                    </div>
                  ) : (
                    filteredTables.map(table => (
                      <div key={table.name} className="border-b border-gray-200 last:border-b-0 pb-2">
                        <div className="flex items-center">
                          <button
                            onClick={() => toggleExpandTable(table.name)}
                            className="mr-2 p-1 hover:bg-gray-100 rounded"
                          >
                            {expandedTables[table.name] ? (
                              <ChevronDown className="w-4 h-4" />
                            ) : (
                              <ChevronRight className="w-4 h-4" />
                            )}
                          </button>
                          <label className="flex items-center gap-2 cursor-pointer flex-grow">
                            <input
                              type="checkbox"
                              className="w-4 h-4 rounded-md border-2 border-black checked:bg-green-500 checked:border-green-500 focus:ring-green-500 text-green-500"
                              checked={selectedTables.includes(table.name)}
                              onChange={() => toggleTable(table.name)}
                            />
                            <span className="font-medium">{table.name}</span>
                          </label>
                        </div>
                        
                        {expandedTables[table.name] && (
                          <div className="ml-8 mt-2 pl-2 border-l-2 border-gray-200">
                            <p className="text-sm font-medium text-gray-700 mb-2">Columns:</p>
                            <div className="space-y-1.5 max-h-[200px] overflow-y-auto pr-2">
                              {table.columns.map(column => (
                                <div key={column.name} className="flex items-start p-1.5 rounded-md hover:bg-gray-50">
                                  <div className="flex-1">
                                    <span className="font-medium text-sm inline-block">{column.name}</span>
                                    <span className="text-xs bg-gray-100 text-gray-700 px-1.5 py-0.5 rounded ml-2">{column.type}</span>
                                    {column.is_nullable && (
                                      <span className="text-xs bg-gray-100 text-gray-500 px-1.5 py-0.5 rounded ml-1">nullable</span>
                                    )}
                                  </div>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    ))
                  )}
                </div>
              </div>
            </div>
          )}
        </div>

        <div className="p-6 border-t-4 border-black">
          <div className="flex gap-4">
            <button
              type="button"
              onClick={handleSave}
              className="neo-button flex-1 relative"
              disabled={isLoading || isSaving}
            >
              {isSaving ? (
                <div className="flex items-center justify-center gap-2">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>Saving...</span>
                </div>
              ) : (
                <span>Save & Apply</span>
              )}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="neo-button-secondary flex-1"
              disabled={isSaving}
            >
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  );
} 