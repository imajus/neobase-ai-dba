import React, { useState, useEffect } from 'react';
import visualizationService from '../../services/visualizationService';
import { VisualizationData, VisualizationSuggestion } from '../../types/visualization';
import { useStream } from '../../contexts/StreamContext';
import { BarChartComponent, LineChartComponent, PieChartComponent, TableComponent, TimeSeriesChartComponent } from './ChartComponents';
import { BarChart, LineChart, PieChart, Table, Loader2 } from 'lucide-react';

interface VisualizationPanelProps {
  chatId: string;
  tableName?: string; // Optional - if provided, show suggestions for specific table
  isVisible: boolean;
  onClose: () => void;
}

const VisualizationPanel: React.FC<VisualizationPanelProps> = ({ 
  chatId, 
  tableName, 
  isVisible,
  onClose
}) => {
  const [suggestions, setSuggestions] = useState<VisualizationSuggestion[]>([]);
  const [allSuggestions, setAllSuggestions] = useState<Record<string, VisualizationSuggestion[]>>({});
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedTable, setSelectedTable] = useState<string>(tableName || '');
  const [selectedSuggestion, setSelectedSuggestion] = useState<VisualizationSuggestion | null>(null);
  const [visualizationData, setVisualizationData] = useState<VisualizationData | null>(null);
  const [executingVisualization, setExecutingVisualization] = useState<boolean>(false);
  
  const { streamId } = useStream();
  
  // Load initial suggestions
  useEffect(() => {
    if (isVisible && chatId) {
      if (tableName) {
        loadTableSuggestions(tableName);
      } else {
        loadAllSuggestions();
      }
    }
  }, [chatId, tableName, isVisible]);
  
  // Update selected table when tableName prop changes
  useEffect(() => {
    if (tableName) {
      setSelectedTable(tableName);
    }
  }, [tableName]);
  
  // Load suggestions for all tables
  const loadAllSuggestions = async () => {
    setLoading(true);
    setError(null);
    
    try {
      const response = await visualizationService.getAllSuggestions(chatId);
      if (response.success) {
        setAllSuggestions(response.data);
        
        // If we have tables, select one to show suggestions
        const tableNames = Object.keys(response.data);
        if (tableNames.length > 0) {
          // Either use the currently selected table or default to first available table
          const tableToSelect = selectedTable && response.data[selectedTable] 
            ? selectedTable 
            : tableNames[0];
            
          setSelectedTable(tableToSelect);
          setSuggestions(response.data[tableToSelect]);
        }
      } else {
        setError('Failed to load visualization suggestions');
      }
    } catch (err: any) {
      setError(err.message || 'Failed to load visualization suggestions');
    } finally {
      setLoading(false);
    }
  };
  
  // Load suggestions for a specific table
  const loadTableSuggestions = async (tableName: string) => {
    setLoading(true);
    setError(null);
    
    try {
      const response = await visualizationService.getTableSuggestions(chatId, tableName);
      if (response.success) {
        setSuggestions(response.data);
      } else {
        setError('Failed to load visualization suggestions for this table');
      }
    } catch (err: any) {
      setError(err.message || 'Failed to load visualization suggestions');
    } finally {
      setLoading(false);
    }
  };
  
  // Handle table selection change
  const handleTableChange = (tableName: string) => {
    setSelectedTable(tableName);
    setSelectedSuggestion(null);
    setVisualizationData(null);
    
    if (allSuggestions[tableName]) {
      setSuggestions(allSuggestions[tableName]);
    } else {
      loadTableSuggestions(tableName);
    }
  };
  
  // Execute a visualization
  const executeVisualization = async (suggestion: VisualizationSuggestion) => {
    setSelectedSuggestion(suggestion);
    setExecutingVisualization(true);
    setError(null);
    
    try {
      const response = await visualizationService.executeVisualization(
        chatId,
        streamId,
        suggestion
      );
      
      if (response.success) {
        setVisualizationData(response.data);
      } else {
        setError('Failed to execute visualization');
      }
    } catch (err: any) {
      setError(err.message || 'Failed to execute visualization');
    } finally {
      setExecutingVisualization(false);
    }
  };
  
  // Render visualization based on type
  const renderVisualization = () => {
    if (!visualizationData) return null;
    
    switch (visualizationData.visualization_type) {
      case 'bar_chart':
        return <BarChartComponent data={visualizationData} />;
      case 'line_chart':
        return <LineChartComponent data={visualizationData} />;
      case 'pie_chart':
        return <PieChartComponent data={visualizationData} />;
      case 'time_series':
        return <TimeSeriesChartComponent data={visualizationData} />;
      case 'table':
        return <TableComponent data={visualizationData} />;
      default:
        return <div className="p-4 text-center text-gray-500">Unsupported visualization type</div>;
    }
  };
  
  // Get the icon for a visualization type
  const getVisualizationIcon = (type: string) => {
    switch (type) {
      case 'bar_chart':
        return <BarChart className="w-5 h-5" />;
      case 'line_chart':
        return <LineChart className="w-5 h-5" />;
      case 'pie_chart':
        return <PieChart className="w-5 h-5" />;
      case 'time_series':
        return <LineChart className="w-5 h-5" />;
      case 'table':
        return <Table className="w-5 h-5" />;
      default:
        return <BarChart className="w-5 h-5" />;
    }
  };
  
  if (!isVisible) return null;
  
  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl w-11/12 md:w-4/5 lg:w-3/4 xl:w-2/3 h-5/6 overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex justify-between items-center border-b p-4">
          <h2 className="text-xl font-bold">Data Visualizations</h2>
          <button
            onClick={onClose}
            className="text-gray-500 hover:text-gray-700 focus:outline-none"
          >
            <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        
        <div className="flex flex-1 overflow-hidden">
          {/* Sidebar with tables and suggestions */}
          <div className="w-64 border-r flex flex-col bg-gray-50">
            {/* Table Selection */}
            <div className="p-4 border-b">
              <h3 className="font-semibold mb-2">Tables</h3>
              {loading && !Object.keys(allSuggestions).length ? (
                <div className="flex items-center justify-center py-4">
                  <Loader2 className="w-5 h-5 animate-spin mr-2" />
                  <span>Loading tables...</span>
                </div>
              ) : (
                <div className="max-h-40 overflow-y-auto">
                  {Object.keys(allSuggestions).map(tableName => (
                    <button
                      key={tableName}
                      onClick={() => handleTableChange(tableName)}
                      className={`block w-full text-left px-3 py-2 text-sm rounded ${
                        selectedTable === tableName
                          ? 'bg-blue-100 text-blue-800'
                          : 'hover:bg-gray-100'
                      }`}
                    >
                      {tableName}
                    </button>
                  ))}
                </div>
              )}
            </div>
            
            {/* Visualization Suggestions */}
            <div className="flex-1 overflow-y-auto p-4">
              <h3 className="font-semibold mb-2">Suggestions</h3>
              {loading && !suggestions.length ? (
                <div className="flex items-center justify-center py-4">
                  <Loader2 className="w-5 h-5 animate-spin mr-2" />
                  <span>Loading suggestions...</span>
                </div>
              ) : error ? (
                <div className="text-red-500 p-2">{error}</div>
              ) : !suggestions.length ? (
                <div className="text-gray-500 p-2">No visualization suggestions available</div>
              ) : (
                <div className="space-y-2">
                  {suggestions.map((suggestion, index) => (
                    <button
                      key={index}
                      onClick={() => executeVisualization(suggestion)}
                      className={`block w-full text-left p-3 text-sm border rounded hover:bg-gray-50 ${
                        selectedSuggestion === suggestion ? 'border-blue-500 bg-blue-50' : 'border-gray-200'
                      }`}
                    >
                      <div className="flex items-center">
                        {getVisualizationIcon(suggestion.visualization_type)}
                        <span className="ml-2 font-medium">{suggestion.title}</span>
                      </div>
                      <p className="text-xs text-gray-500 mt-1">{suggestion.description}</p>
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
          
          {/* Main Content Area for Visualization */}
          <div className="flex-1 overflow-y-auto p-4">
            {executingVisualization ? (
              <div className="flex flex-col items-center justify-center h-full">
                <Loader2 className="w-10 h-10 animate-spin mb-4" />
                <p>Executing visualization...</p>
              </div>
            ) : error && !visualizationData ? (
              <div className="flex flex-col items-center justify-center h-full">
                <div className="text-red-500 mb-2">Error: {error}</div>
                <button
                  onClick={() => selectedSuggestion && executeVisualization(selectedSuggestion)}
                  className="px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
                >
                  Try Again
                </button>
              </div>
            ) : !visualizationData ? (
              <div className="flex flex-col items-center justify-center h-full text-gray-500">
                <div className="mb-4 text-center">
                  <p className="text-xl mb-2">Select a visualization from the sidebar</p>
                  <p className="text-sm">Choose from the suggested visualizations to analyze your data</p>
                </div>
              </div>
            ) : (
              renderVisualization()
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default VisualizationPanel; 