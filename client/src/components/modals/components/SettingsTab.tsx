import React from 'react';

interface SettingsTabProps {
  autoExecuteQuery: boolean;
  shareWithAI: boolean;
  setAutoExecuteQuery: (value: boolean) => void;
  setShareWithAI: (value: boolean) => void;
  onUpdateAutoExecuteQuery?: (chatId: string, autoExecuteQuery: boolean) => Promise<void>;
  initialDataId?: string;
}

const SettingsTab: React.FC<SettingsTabProps> = ({
  autoExecuteQuery,
  shareWithAI,
  setAutoExecuteQuery,
  setShareWithAI,
  onUpdateAutoExecuteQuery,
  initialDataId
}) => {
  return (
    <div className="space-y-6">
      <div className="neo-border p-4 rounded-lg">
        <div className="flex items-center justify-between">
          <div>
            <label className="block font-bold mb-1 text-lg">Auto Fetch Results</label>
            <p className="text-gray-600 text-sm">Automatically fetches results from the database upon the AI response. <br />However, the critical queries still need to be executed manually by the user.</p>
          </div>
          <label className="relative inline-flex items-center cursor-pointer">
            <input 
              type="checkbox" 
              className="sr-only peer" 
              checked={autoExecuteQuery}
              onChange={(e) => {
                const newValue = e.target.checked;
                setAutoExecuteQuery(newValue);
                if (initialDataId && onUpdateAutoExecuteQuery) {
                  onUpdateAutoExecuteQuery(initialDataId, newValue);
                }
              }}
            />
            <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
          </label>
        </div>
      </div>

      <div className="neo-border p-4 rounded-lg">
        <div className="flex items-center justify-between">
          <div>
            <label className="block font-bold mb-1 text-lg">Share Data With AI</label>
            <p className="text-gray-600 text-sm">Allow NeoBase to share your query responses with AI for better responses. <br />NOTE: This will take more tokens that are being sent to the AI.</p>
          </div>
          <label className="relative inline-flex items-center cursor-pointer">
            <input 
              type="checkbox" 
              className="sr-only peer" 
              checked={shareWithAI}
              onChange={(e) => {
                setShareWithAI(e.target.checked);
                // In the future, we would add logic to update this setting on the server
              }}
            />
            <div className="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
          </label>
        </div>
      </div>

      <div className="p-4 bg-emerald-100 border border-emerald-200 rounded-lg">
        <p className="text-sm text-emerald-700 font-medium">
          More settings coming soon! We're constantly working to improve the configuration options for your database connections.
        </p>
      </div>
    </div>
  );
};

export default SettingsTab; 